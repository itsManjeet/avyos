package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	_ "embed"
)

const (
	mib              = int64(1024 * 1024)
	lbaSize          = int64(512)
	efiSizeMiB       = int64(200)
	biosSizeMiB      = int64(1)
	minSysMiB        = int64(256)
	minDataMiB       = int64(512)
	gptTailMiB       = int64(4)
	btrfsTemplateMiB = int64(511)
)

const (
	linuxFSPartType = "0fc63daf-8483-4772-8e79-3d69d8477de4"
	efiPartType     = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
	biosPartType    = "21686148-6449-6e6f-744e-656564454649"
)

//go:embed assets/btrfs-512m.img.gz
var btrfsTemplateGzip []byte

type config struct {
	out        string
	target     string
	kargs      string
	diskSize   string
	kernel     string
	initrd     string
	rootfs     string
	liminePath string
}

type partitionPlan struct {
	biosStartMiB int64
	biosEndMiB   int64
	efiStartMiB  int64
	efiEndMiB    int64
	sysStartMiB  int64
	sysEndMiB    int64
	dataStartMiB int64
	dataEndMiB   int64
	diskBytes    int64
	diskMiB      int64
	sysSizeMiB   int64
}

type gptPartition struct {
	TypeGUID   [16]byte
	GUID       [16]byte
	FirstLBA   uint64
	LastLBA    uint64
	Attributes uint64
	Name       string
}

type fatFile struct {
	name    string
	short83 [11]byte
	attr    byte
	cluster uint32
	size    uint32
	data    []byte
}

func main() {
	var cfg config
	flag.StringVar(&cfg.out, "out", "avyos.img", "Output disk image path")
	flag.StringVar(&cfg.target, "target", "", "Target arch: amd64 or arm64")
	flag.StringVar(&cfg.kargs, "kargs", "", "Additional kernel args")
	flag.StringVar(&cfg.diskSize, "disk-size", "", "Disk size (for example 1G, 973M)")
	flag.StringVar(&cfg.kernel, "kernel", "", "Kernel image path")
	flag.StringVar(&cfg.initrd, "initrd", "", "Initrd image path")
	flag.StringVar(&cfg.rootfs, "rootfs", "", "SquashFS root image path")
	flag.StringVar(&cfg.liminePath, "limine-path", "/usr/share/limine", "Path containing Limine EFI binary")
	flag.Parse()

	if cfg.target == "" {
		cfg.target = os.Getenv("GOARCH")
		if cfg.target == "" {
			cfg.target = "amd64"
		}
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "genimage:", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	if cfg.kernel == "" {
		return errors.New("missing -kernel")
	}
	if cfg.initrd == "" {
		return errors.New("missing -initrd")
	}
	if cfg.rootfs == "" {
		return errors.New("missing -rootfs")
	}

	for _, path := range []string{cfg.kernel, cfg.initrd, cfg.rootfs} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}

	plan, err := computePlan(cfg)
	if err != nil {
		return err
	}

	fmt.Printf("[*] Creating disk: %s (%d MiB)\n", cfg.out, plan.diskMiB)
	disk, err := os.OpenFile(cfg.out, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer disk.Close()
	if err := disk.Truncate(plan.diskBytes); err != nil {
		return err
	}

	if err := writeGPT(disk, plan); err != nil {
		return err
	}

	efiImg, err := buildEFIImage(cfg)
	if err != nil {
		return err
	}
	if int64(len(efiImg)) != efiSizeMiB*mib {
		return fmt.Errorf("efi image size mismatch: got %d", len(efiImg))
	}
	if err := writeAtMiB(disk, plan.efiStartMiB, bytes.NewReader(efiImg)); err != nil {
		return err
	}

	if cfg.rootfs != "" {
		if err := copyFileAtMiB(disk, cfg.rootfs, plan.sysStartMiB, plan.sysSizeMiB*mib); err != nil {
			return err
		}
	}

	dataBytes := (plan.dataEndMiB - plan.dataStartMiB) * mib
	if dataBytes < btrfsTemplateMiB*mib {
		return fmt.Errorf("data partition too small: %d MiB", dataBytes/mib)
	}
	if err := writeBtrfsTemplateAtMiB(disk, plan.dataStartMiB); err != nil {
		return err
	}

	fmt.Printf("[✓] Image ready: %s\n", cfg.out)
	return nil
}

func computePlan(cfg config) (partitionPlan, error) {
	var plan partitionPlan
	rootfsMiB := int64(0)
	if cfg.rootfs != "" {
		st, err := os.Stat(cfg.rootfs)
		if err != nil {
			return plan, err
		}
		rootfsMiB = ceilDiv(st.Size(), mib)
	}

	plan.sysSizeMiB = max((rootfsMiB*110)/100, minSysMiB)

	if cfg.diskSize == "" {
		totalMiB := biosSizeMiB + efiSizeMiB + plan.sysSizeMiB + minDataMiB + gptTailMiB
		cfg.diskSize = strconv.FormatInt(totalMiB, 10) + "M"
	}

	diskBytes, err := parseSize(cfg.diskSize)
	if err != nil {
		return plan, err
	}
	if diskBytes%mib != 0 {
		return plan, fmt.Errorf("disk size must be MiB-aligned: %s", cfg.diskSize)
	}

	plan.diskBytes = diskBytes
	plan.diskMiB = diskBytes / mib

	plan.biosStartMiB = 1
	plan.biosEndMiB = plan.biosStartMiB + biosSizeMiB
	plan.efiStartMiB = plan.biosEndMiB
	plan.efiEndMiB = plan.efiStartMiB + efiSizeMiB
	plan.sysStartMiB = plan.efiEndMiB
	plan.sysEndMiB = plan.sysStartMiB + plan.sysSizeMiB
	plan.dataStartMiB = plan.sysEndMiB
	plan.dataEndMiB = plan.diskMiB - gptTailMiB

	if plan.dataEndMiB <= plan.dataStartMiB {
		return plan, errors.New("disk too small for data partition")
	}
	if plan.dataEndMiB-plan.dataStartMiB < btrfsTemplateMiB {
		return plan, fmt.Errorf("data partition too small: need at least %d MiB", btrfsTemplateMiB)
	}
	return plan, nil
}

func writeGPT(disk *os.File, plan partitionPlan) error {
	totalSectors := uint64(plan.diskBytes / lbaSize)
	if totalSectors < 2048 {
		return errors.New("disk too small for GPT")
	}

	entries := make([]byte, 128*128)
	parts := []gptPartition{
		mustPart(biosPartType, plan.biosStartMiB, plan.biosEndMiB, "bios"),
		mustPart(efiPartType, plan.efiStartMiB, plan.efiEndMiB, "esp"),
		mustPart(linuxFSPartType, plan.sysStartMiB, plan.sysEndMiB, "system"),
		mustPart(linuxFSPartType, plan.dataStartMiB, plan.dataEndMiB, "data"),
	}
	for i, part := range parts {
		encodeGPTEntry(entries[i*128:(i+1)*128], part)
	}
	entriesCRC := crc32.ChecksumIEEE(entries)

	firstUsable := uint64(34)
	lastUsable := totalSectors - 34
	primaryEntriesLBA := uint64(2)
	backupEntriesLBA := totalSectors - 33

	diskGUID, err := randomGUID()
	if err != nil {
		return err
	}
	primaryHeader := makeGPTHeader(diskGUID, 1, totalSectors-1, firstUsable, lastUsable, primaryEntriesLBA, entriesCRC)
	backupHeader := makeGPTHeader(diskGUID, totalSectors-1, 1, firstUsable, lastUsable, backupEntriesLBA, entriesCRC)

	if err := writeProtectiveMBR(disk, totalSectors); err != nil {
		return err
	}
	if err := writeAtLBA(disk, primaryEntriesLBA, entries); err != nil {
		return err
	}
	if err := writeAtLBA(disk, 1, primaryHeader); err != nil {
		return err
	}
	if err := writeAtLBA(disk, backupEntriesLBA, entries); err != nil {
		return err
	}
	if err := writeAtLBA(disk, totalSectors-1, backupHeader); err != nil {
		return err
	}
	return nil
}

func mustPart(typeGUID string, startMiB, endMiB int64, name string) gptPartition {
	guid, err := randomGUID()
	if err != nil {
		panic(err)
	}
	typeID := mustParseGUID(typeGUID)
	return gptPartition{
		TypeGUID: typeID,
		GUID:     guid,
		FirstLBA: uint64((startMiB * mib) / lbaSize),
		LastLBA:  uint64((endMiB*mib)/lbaSize - 1),
		Name:     name,
	}
}

func makeGPTHeader(diskGUID [16]byte, current, backup, firstUsable, lastUsable, entriesLBA uint64, entriesCRC uint32) []byte {
	header := make([]byte, 512)
	copy(header[0:8], []byte("EFI PART"))
	binary.LittleEndian.PutUint32(header[8:12], 0x00010000)
	binary.LittleEndian.PutUint32(header[12:16], 92)
	binary.LittleEndian.PutUint64(header[24:32], current)
	binary.LittleEndian.PutUint64(header[32:40], backup)
	binary.LittleEndian.PutUint64(header[40:48], firstUsable)
	binary.LittleEndian.PutUint64(header[48:56], lastUsable)
	copy(header[56:72], diskGUID[:])
	binary.LittleEndian.PutUint64(header[72:80], entriesLBA)
	binary.LittleEndian.PutUint32(header[80:84], 128)
	binary.LittleEndian.PutUint32(header[84:88], 128)
	binary.LittleEndian.PutUint32(header[88:92], entriesCRC)
	binary.LittleEndian.PutUint32(header[16:20], crc32.ChecksumIEEE(header[:92]))
	return header
}

func encodeGPTEntry(buf []byte, p gptPartition) {
	copy(buf[0:16], p.TypeGUID[:])
	copy(buf[16:32], p.GUID[:])
	binary.LittleEndian.PutUint64(buf[32:40], p.FirstLBA)
	binary.LittleEndian.PutUint64(buf[40:48], p.LastLBA)
	binary.LittleEndian.PutUint64(buf[48:56], p.Attributes)
	nameUTF16 := utf16.Encode([]rune(p.Name))
	for i := 0; i < len(nameUTF16) && i < 36; i++ {
		binary.LittleEndian.PutUint16(buf[56+i*2:58+i*2], nameUTF16[i])
	}
}

func writeProtectiveMBR(disk *os.File, totalSectors uint64) error {
	mbr := make([]byte, 512)
	part := mbr[446:462]
	part[0] = 0x00
	part[1] = 0x00
	part[2] = 0x02
	part[3] = 0x00
	part[4] = 0xEE
	part[5] = 0xFF
	part[6] = 0xFF
	part[7] = 0xFF
	binary.LittleEndian.PutUint32(part[8:12], 1)
	count := uint32(math.MaxUint32)
	if totalSectors > 1 && totalSectors-1 < uint64(math.MaxUint32) {
		count = uint32(totalSectors - 1)
	}
	binary.LittleEndian.PutUint32(part[12:16], count)
	mbr[510] = 0x55
	mbr[511] = 0xAA
	_, err := disk.WriteAt(mbr, 0)
	return err
}

func buildEFIImage(cfg config) ([]byte, error) {
	kernelData, err := os.ReadFile(cfg.kernel)
	if err != nil {
		return nil, err
	}
	var initrdData []byte
	if cfg.initrd != "" {
		initrdData, err = os.ReadFile(cfg.initrd)
		if err != nil {
			return nil, err
		}
	}

	efiBinName := "BOOTX64.EFI"
	avDevice := "device:sda3"
	rootDevice := "device:sda4"
	switch cfg.target {
	case "amd64":
		efiBinName = "BOOTX64.EFI"
		avDevice = "device:sda3"
		rootDevice = "device:sda4"
	case "arm64":
		efiBinName = "BOOTAA64.EFI"
		avDevice = "device:vda3"
		rootDevice = "device:vda4"
	default:
		return nil, fmt.Errorf("invalid target %s", cfg.target)
	}
	efiBinData, err := os.ReadFile(filepath.Join(cfg.liminePath, efiBinName))
	if err != nil {
		return nil, err
	}

	limineConf := buildLimineConfig(cfg.kargs, avDevice, rootDevice)

	files := []fatFile{
		{name: "kernel", short83: shortNoExt("KERNEL"), attr: 0x20, data: kernelData},
		{name: "EFI", short83: shortNoExt("EFI"), attr: 0x10},
		{name: "limine", short83: shortNoExt("LIMINE"), attr: 0x10},
		{name: "BOOT", short83: shortNoExt("BOOT"), attr: 0x10},
		{name: efiBinName, short83: shortWithExt(strings.TrimSuffix(efiBinName, ".EFI"), "EFI"), attr: 0x20, data: efiBinData},
		{name: "limine.conf", short83: shortWithExt("LIMINE~1", "CON"), attr: 0x20, data: limineConf},
	}
	if len(initrdData) > 0 {
		files = append(files, fatFile{name: "initrd", short83: shortNoExt("INITRD"), attr: 0x20, data: initrdData})
	}

	return buildFAT32(efiSizeMiB*mib, files)
}

func buildLimineConfig(kargs, avDevice, rootDevice string) []byte {
	cmdline := []string{
		"root=" + rootDevice,
		"rootfstype=btrfs",
		"avyos=" + avDevice,
		"avyosfstype=squashfs",
	}
	if trimmed := strings.TrimSpace(kargs); trimmed != "" {
		cmdline = append(cmdline, strings.Fields(trimmed)...)
	}
	cmdline = append(cmdline, "fbcon=font:TER10x18", "quiet")
	return fmt.Appendf(nil, "timeout: 0\n\n/avyos\n    protocol: linux\n    path: boot():/kernel\n    cmdline: %s\n    module_path: boot():/initrd\n", strings.Join(cmdline, " "))
}

func buildFAT32(size int64, files []fatFile) ([]byte, error) {
	if size%lbaSize != 0 {
		return nil, errors.New("fat image size must be sector aligned")
	}
	img := make([]byte, size)
	totalSectors := uint32(size / lbaSize)
	reserved := uint32(32)
	numFATs := uint32(2)
	sectorsPerCluster := uint32(1)

	fatSectors := uint32(1)
	for {
		dataSectors := totalSectors - reserved - numFATs*fatSectors
		clusters := dataSectors / sectorsPerCluster
		need := uint32(ceilDiv(int64((clusters+2)*4), lbaSize))
		if need == fatSectors {
			break
		}
		fatSectors = need
	}
	dataStartSector := reserved + numFATs*fatSectors

	putBootSector(img[0:512], totalSectors, sectorsPerCluster, reserved, numFATs, fatSectors)
	putFSInfo(img[512:1024])
	copy(img[6*512:7*512], img[0:512])
	copy(img[7*512:8*512], img[512:1024])

	fat := make([]uint32, (fatSectors*512)/4)
	fat[0] = 0x0FFFFFF8
	fat[1] = 0xFFFFFFFF
	fat[2] = 0x0FFFFFFF

	nextCluster := uint32(3)
	alloc := func(n uint32) uint32 {
		start := nextCluster
		for i := range n {
			cur := start + i
			nxt := cur + 1
			if i == n-1 {
				nxt = 0x0FFFFFFF
			}
			fat[cur] = nxt
		}
		nextCluster += n
		return start
	}

	cEFI := alloc(1)
	cBoot := alloc(1)
	cLimine := alloc(1)

	var fKernel, fInitrd, fBootEFI, fLimineConf *fatFile
	for i := range files {
		f := &files[i]
		switch f.name {
		case "kernel":
			fKernel = f
		case "initrd":
			fInitrd = f
		case "BOOTX64.EFI", "BOOTAA64.EFI":
			fBootEFI = f
		case "limine.conf":
			fLimineConf = f
		}
	}
	if fKernel == nil || fBootEFI == nil || fLimineConf == nil {
		return nil, errors.New("missing required EFI files")
	}

	allocFile := func(f *fatFile) {
		n := uint32(ceilDiv(int64(len(f.data)), lbaSize*int64(sectorsPerCluster)))
		if n == 0 {
			n = 1
		}
		f.cluster = alloc(n)
		f.size = uint32(len(f.data))
	}
	allocFile(fKernel)
	if fInitrd != nil {
		allocFile(fInitrd)
	}
	allocFile(fBootEFI)
	allocFile(fLimineConf)

	for i, v := range fat {
		off := int64(reserved)*lbaSize + int64(i*4)
		if off+4 > int64(reserved+fatSectors)*lbaSize {
			break
		}
		binary.LittleEndian.PutUint32(img[off:off+4], v)
		off2 := int64(reserved+fatSectors)*lbaSize + int64(i*4)
		if off2+4 <= int64(reserved+fatSectors*2)*lbaSize {
			binary.LittleEndian.PutUint32(img[off2:off2+4], v)
		}
	}

	clusterOffset := func(c uint32) int64 {
		return int64(dataStartSector)*lbaSize + int64(c-2)*int64(sectorsPerCluster)*lbaSize
	}
	writeCluster := func(c uint32, data []byte) {
		off := clusterOffset(c)
		copy(img[off:off+int64(len(data))], data)
	}

	rootEntries := make([]byte, 0, 512)
	rootEntries = append(rootEntries, dirEntry(shortNoExt("EFI"), 0x10, cEFI, 0)...) // EFI dir
	rootEntries = append(rootEntries, dirEntry(shortNoExt("LIMINE"), 0x10, cLimine, 0)...)
	rootEntries = append(rootEntries, dirEntry(fKernel.short83, 0x20, fKernel.cluster, fKernel.size)...)
	if fInitrd != nil {
		rootEntries = append(rootEntries, dirEntry(fInitrd.short83, 0x20, fInitrd.cluster, fInitrd.size)...)
	}
	writeCluster(2, padTo(rootEntries, 512))

	efiEntries := make([]byte, 0, 512)
	efiEntries = append(efiEntries, dirEntry(shortNoExt("."), 0x10, cEFI, 0)...)
	efiEntries = append(efiEntries, dirEntry(shortNoExt(".."), 0x10, 2, 0)...)
	efiEntries = append(efiEntries, dirEntry(shortNoExt("BOOT"), 0x10, cBoot, 0)...)
	writeCluster(cEFI, padTo(efiEntries, 512))

	bootEntries := make([]byte, 0, 512)
	bootEntries = append(bootEntries, dirEntry(shortNoExt("."), 0x10, cBoot, 0)...)
	bootEntries = append(bootEntries, dirEntry(shortNoExt(".."), 0x10, cEFI, 0)...)
	bootEntries = append(bootEntries, dirEntry(fBootEFI.short83, 0x20, fBootEFI.cluster, fBootEFI.size)...)
	writeCluster(cBoot, padTo(bootEntries, 512))

	limineEntries := make([]byte, 0, 512)
	limineEntries = append(limineEntries, dirEntry(shortNoExt("."), 0x10, cLimine, 0)...)
	limineEntries = append(limineEntries, dirEntry(shortNoExt(".."), 0x10, 2, 0)...)
	limineEntries = append(limineEntries, lfnEntries(fLimineConf.name, fLimineConf.short83)...)
	limineEntries = append(limineEntries, dirEntry(fLimineConf.short83, 0x20, fLimineConf.cluster, fLimineConf.size)...)
	writeCluster(cLimine, padTo(limineEntries, 512))

	writeFile := func(f *fatFile) {
		if f == nil {
			return
		}
		cluster := f.cluster
		rem := f.data
		for {
			chunk := rem
			if len(chunk) > int(lbaSize*int64(sectorsPerCluster)) {
				chunk = chunk[:lbaSize*int64(sectorsPerCluster)]
			}
			writeCluster(cluster, chunk)
			if len(rem) <= len(chunk) {
				break
			}
			rem = rem[len(chunk):]
			cluster = fat[cluster]
			if cluster >= 0x0FFFFFF8 {
				break
			}
		}
	}
	writeFile(fKernel)
	writeFile(fInitrd)
	writeFile(fBootEFI)
	writeFile(fLimineConf)

	return img, nil
}

func putBootSector(bs []byte, totalSectors, sectorsPerCluster, reserved, numFATs, fatSectors uint32) {
	for i := range bs {
		bs[i] = 0
	}
	bs[0] = 0xEB
	bs[1] = 0x58
	bs[2] = 0x90
	copy(bs[3:11], []byte("MSWIN4.1"))
	binary.LittleEndian.PutUint16(bs[11:13], 512)
	bs[13] = byte(sectorsPerCluster)
	binary.LittleEndian.PutUint16(bs[14:16], uint16(reserved))
	bs[16] = byte(numFATs)
	binary.LittleEndian.PutUint16(bs[17:19], 0)
	binary.LittleEndian.PutUint16(bs[19:21], 0)
	bs[21] = 0xF8
	binary.LittleEndian.PutUint16(bs[22:24], 0)
	binary.LittleEndian.PutUint16(bs[24:26], 63)
	binary.LittleEndian.PutUint16(bs[26:28], 255)
	binary.LittleEndian.PutUint32(bs[28:32], 0)
	binary.LittleEndian.PutUint32(bs[32:36], totalSectors)
	binary.LittleEndian.PutUint32(bs[36:40], fatSectors)
	binary.LittleEndian.PutUint16(bs[40:42], 0)
	binary.LittleEndian.PutUint16(bs[42:44], 0)
	binary.LittleEndian.PutUint32(bs[44:48], 2)
	binary.LittleEndian.PutUint16(bs[48:50], 1)
	binary.LittleEndian.PutUint16(bs[50:52], 6)
	bs[64] = 0x80
	bs[66] = 0x29
	binary.LittleEndian.PutUint32(bs[67:71], 0x12345678)
	copy(bs[71:82], []byte("avyos EFI  "))
	copy(bs[82:90], []byte("FAT32   "))
	bs[510] = 0x55
	bs[511] = 0xAA
}

func putFSInfo(bs []byte) {
	for i := range bs {
		bs[i] = 0
	}
	binary.LittleEndian.PutUint32(bs[0:4], 0x41615252)
	binary.LittleEndian.PutUint32(bs[484:488], 0x61417272)
	binary.LittleEndian.PutUint32(bs[488:492], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(bs[492:496], 0xFFFFFFFF)
	binary.LittleEndian.PutUint16(bs[510:512], 0xAA55)
}

func dirEntry(short [11]byte, attr byte, cluster uint32, size uint32) []byte {
	e := make([]byte, 32)
	copy(e[0:11], short[:])
	e[11] = attr
	binary.LittleEndian.PutUint16(e[20:22], uint16(cluster>>16))
	binary.LittleEndian.PutUint16(e[26:28], uint16(cluster&0xFFFF))
	binary.LittleEndian.PutUint32(e[28:32], size)
	return e
}

func lfnEntries(name string, short [11]byte) []byte {
	utf := utf16.Encode([]rune(name))
	utf = append(utf, 0x0000)
	for len(utf)%13 != 0 {
		utf = append(utf, 0xFFFF)
	}
	chunks := len(utf) / 13
	checksum := lfnChecksum(short)
	out := make([]byte, 0, chunks*32)
	for i := chunks - 1; i >= 0; i-- {
		entry := make([]byte, 32)
		ord := byte(i + 1)
		if i == chunks-1 {
			ord |= 0x40
		}
		entry[0] = ord
		entry[11] = 0x0F
		entry[13] = checksum
		entry[26] = 0
		entry[27] = 0
		chars := utf[i*13 : (i+1)*13]
		putLFNChars(entry, chars)
		out = append(out, entry...)
	}
	return out
}

func putLFNChars(entry []byte, chars []uint16) {
	idx := []int{1, 3, 5, 7, 9, 14, 16, 18, 20, 22, 24, 28, 30}
	for i, c := range chars {
		binary.LittleEndian.PutUint16(entry[idx[i]:idx[i]+2], c)
	}
}

func lfnChecksum(short [11]byte) byte {
	var sum byte
	for i := range 11 {
		sum = ((sum & 1) << 7) + (sum >> 1) + short[i]
	}
	return sum
}

func shortNoExt(name string) [11]byte {
	var s [11]byte
	for i := range s {
		s[i] = ' '
	}
	name = strings.ToUpper(name)
	if name == "." {
		s[0] = '.'
		return s
	}
	if name == ".." {
		s[0] = '.'
		s[1] = '.'
		return s
	}
	for i, r := range name {
		if i >= 8 {
			break
		}
		s[i] = byte(r)
	}
	return s
}

func shortWithExt(base, ext string) [11]byte {
	s := shortNoExt(base)
	ext = strings.ToUpper(ext)
	for i, r := range ext {
		if i >= 3 {
			break
		}
		s[8+i] = byte(r)
	}
	return s
}

func writeBtrfsTemplateAtMiB(disk *os.File, startMiB int64) error {
	gz, err := gzip.NewReader(bytes.NewReader(btrfsTemplateGzip))
	if err != nil {
		return err
	}
	defer gz.Close()

	off := startMiB * mib
	if _, err := disk.Seek(off, io.SeekStart); err != nil {
		return err
	}
	written, err := io.CopyN(disk, gz, btrfsTemplateMiB*mib)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if written != btrfsTemplateMiB*mib {
		return fmt.Errorf("btrfs template size mismatch: wrote %d", written)
	}
	return nil
}

func copyFileAtMiB(disk *os.File, src string, startMiB int64, maxBytes int64) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if st.Size() > maxBytes {
		return fmt.Errorf("%s exceeds partition size", src)
	}
	if _, err := disk.Seek(startMiB*mib, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(disk, in)
	return err
}

func writeAtMiB(disk *os.File, startMiB int64, r io.Reader) error {
	if _, err := disk.Seek(startMiB*mib, io.SeekStart); err != nil {
		return err
	}
	_, err := io.Copy(disk, r)
	return err
}

func writeAtLBA(disk *os.File, lba uint64, data []byte) error {
	_, err := disk.WriteAt(data, int64(lba)*lbaSize)
	return err
}

func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty size")
	}
	mult := int64(1)
	last := s[len(s)-1]
	switch last {
	case 'K', 'k':
		mult = 1024
		s = s[:len(s)-1]
	case 'M', 'm':
		mult = mib
		s = s[:len(s)-1]
	case 'G', 'g':
		mult = mib * 1024
		s = s[:len(s)-1]
	case 'T', 't':
		mult = mib * 1024 * 1024
		s = s[:len(s)-1]
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if v <= 0 {
		return 0, errors.New("size must be positive")
	}
	return v * mult, nil
}

func ceilDiv(v, d int64) int64 {
	if v == 0 {
		return 0
	}
	return (v + d - 1) / d
}

func mustParseGUID(s string) [16]byte {
	g, err := parseGUID(s)
	if err != nil {
		panic(err)
	}
	return g
}

func parseGUID(s string) ([16]byte, error) {
	var out [16]byte
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "-")
	if len(parts) != 5 {
		return out, fmt.Errorf("invalid guid %q", s)
	}
	buf := parts[0] + parts[1] + parts[2] + parts[3] + parts[4]
	if len(buf) != 32 {
		return out, fmt.Errorf("invalid guid %q", s)
	}
	raw := make([]byte, 16)
	for i := range 16 {
		b, err := strconv.ParseUint(buf[i*2:i*2+2], 16, 8)
		if err != nil {
			return out, err
		}
		raw[i] = byte(b)
	}
	binary.LittleEndian.PutUint32(out[0:4], binary.BigEndian.Uint32(raw[0:4]))
	binary.LittleEndian.PutUint16(out[4:6], binary.BigEndian.Uint16(raw[4:6]))
	binary.LittleEndian.PutUint16(out[6:8], binary.BigEndian.Uint16(raw[6:8]))
	copy(out[8:16], raw[8:16])
	return out, nil
}

func randomGUID() ([16]byte, error) {
	var g [16]byte
	if _, err := io.ReadFull(rand.Reader, g[:]); err != nil {
		return g, err
	}
	g[6] = (g[6] & 0x0F) | 0x40
	g[8] = (g[8] & 0x3F) | 0x80
	return g, nil
}

func padTo(data []byte, n int) []byte {
	if len(data) >= n {
		return data[:n]
	}
	out := make([]byte, n)
	copy(out, data)
	return out
}
