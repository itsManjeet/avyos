package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"avyos.dev/pkg/fs"
)

type archConfig struct {
	releaseArchs []string
	qemuBin      string
	qemuExtra    []string
}

var (
	flagArch    string
	flagBranch  string
	flagCPU     string
	flagMemory  string
	flagVNC     string
	flagAccel   string
	flagDBGPort int
)

func init() {
	flag.StringVar(&flagArch, "arch", runtime.GOARCH, "Target arch")
	flag.StringVar(&flagBranch, "branch", "main", "Release branch")
	flag.StringVar(&flagCPU, "cpu", "2", "CPU to allocate for emulator")
	flag.StringVar(&flagMemory, "memory", "2G", "Memory to allocate for emulator")
	flag.StringVar(&flagVNC, "vnc", "", "Start VNC server")
	flag.StringVar(&flagAccel, "accel", "", "Hardware Acceleration")
	flag.IntVar(&flagDBGPort, "dbg-port", 5037, "Forward host TCP port to guest dbgd port 5037 (0 disables)")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "runimage - Download and run avyos image in QEMU")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  runimage [--arch ARCH] [--branch BRANCH] [--cpu N] [--memory SIZE] [--vnc DISPLAY] [--dbg-port PORT] [qemu args...]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  (none)")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/tool error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

func main() {
	flag.Parse()
	if err := run(flag.Args()); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	branch := flagBranch
	arch := flagArch
	url := fmt.Sprintf("https://github.com/itsmanjeet/avyos/releases/download/%s/avyos-%s-%s.zip", branch, branch, arch)
	archive := filepath.Base(url)
	name := strings.TrimSuffix(archive, ".zip")

	fmt.Printf("[*] Downloading %s\n", url)
	if err := download(
		url,
		archive); err != nil {
		return err
	}

	fmt.Println("[*] Extracting disk image...")
	if err := extract(archive, name, []string{"disk.img", "firmware", "variables"}); err != nil {
		return err
	}

	var bin string
	qemuArgs := []string{
		"-smp", flagCPU,
		"-m", flagMemory,
		"-serial", "mon:stdio",
		"-vga", "none",
		"-device", "virtio-gpu-pci",
		"-device", "virtio-keyboard-pci",
		"-drive", fmt.Sprintf("if=pflash,file=%s/firmware,readonly=on,format=raw", name),
		"-drive", fmt.Sprintf("if=pflash,file=%s/variables,format=raw", name),
		"-drive", fmt.Sprintf("file=%s/disk.img,format=raw", name),
	}
	pointerDevice := "virtio-mouse-pci"
	if flagVNC != "" {
		pointerDevice = "virtio-tablet-pci"
	}
	qemuArgs = append(qemuArgs, "-device", pointerDevice)
	if flagDBGPort < 0 || flagDBGPort > 65535 {
		return fmt.Errorf("invalid dbg-port: %d", flagDBGPort)
	}
	nicArg := "user,model=virtio-net-pci"
	if flagDBGPort > 0 {
		nicArg += fmt.Sprintf(",hostfwd=tcp:127.0.0.1:%d-:5037", flagDBGPort)
	}
	qemuArgs = append(qemuArgs, "-nic", nicArg)

	if flagAccel != "none" {
		if flagAccel == "" {
			switch runtime.GOOS {
			case "linux":
				if fs.Exists("/dev/kvm") {
					fmt.Println("[*] Using kvm hardware acceleration")
					flagAccel = "kvm"
				}
			case "windows":
				fmt.Println("[*] Using whpx hardware acceleration")
				flagAccel = "whpx"
			case "darwin":
				fmt.Println("[*] Using hcf hardware acceleration")
				flagAccel = "hvf"
			}
		}
		qemuArgs = append(qemuArgs, "-accel", flagAccel)
	}

	switch arch {
	case "amd64":
		bin = "qemu-system-x86_64"
	case "arm64":
		bin = "qemu-system-aarch64"
		qemuArgs = append(qemuArgs, "-M", "virt")
	}

	if vnc := flagVNC; vnc != "" {
		qemuArgs = append(qemuArgs, "-vnc", vnc)
	}

	qemuArgs = append(qemuArgs, args...)

	fmt.Println("[*] Starting qemu emulator...")
	cmd := exec.Command(bin, qemuArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func extract(zipPath, dest string, allowed []string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	allowedSet := make(map[string]struct{})
	for _, a := range allowed {
		allowedSet[a] = struct{}{}
	}

	for _, f := range r.File {
		name := filepath.Clean(f.Name)

		if len(allowedSet) > 0 {
			top := strings.Split(name, string(os.PathSeparator))[0]
			if _, ok := allowedSet[top]; !ok {
				continue
			}
		}

		targetPath := filepath.Join(dest, name)

		if !strings.HasPrefix(
			filepath.Clean(targetPath),
			filepath.Clean(dest)+string(os.PathSeparator),
		) {
			return fmt.Errorf("illegal path: %s", name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.OpenFile(
			targetPath,
			os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
			f.Mode(),
		)
		if err != nil {
			rc.Close()
			return err
		}

		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}

		out.Close()
		rc.Close()
	}

	return nil
}

func download(url, outputPath string) error {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	var existingSize int64

	if fs.Exists(outputPath) {
		return nil
	}

	// Check if file already exists (resume support)
	if fi, err := os.Stat(outputPath + ".part"); err == nil {
		existingSize = fi.Size()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// Add Range header if resuming
	if existingSize > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(existingSize, 10)+"-")
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle server response
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("url(%s) bad status: %s", url, resp.Status)
	}

	var file *os.File

	// If server ignored range → restart
	if resp.StatusCode == http.StatusOK && existingSize > 0 {
		file, err = os.Create(outputPath + ".part")
		existingSize = 0
	} else {
		file, err = os.OpenFile(outputPath+".part", os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			_, err = file.Seek(existingSize, io.SeekStart)
		}
	}
	if err != nil {
		return err
	}
	defer file.Close()

	totalSize := resp.ContentLength
	if resp.StatusCode == http.StatusPartialContent {
		totalSize += existingSize
	}

	var downloaded int64 = existingSize
	start := time.Now()

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				d := atomic.LoadInt64(&downloaded)
				elapsed := time.Since(start).Seconds()
				speed := float64(d-existingSize) / 1024 / elapsed

				if totalSize > 0 {
					percent := float64(d) * 100 / float64(totalSize)
					fmt.Printf("\r%.2f%% | %d/%d bytes | %.2f KB/s",
						percent, d, totalSize, speed)
				} else {
					fmt.Printf("\r%d bytes | %.2f KB/s",
						d, speed)
				}
			case <-done:
				return
			}
		}
	}()

	counter := &writeCounter{total: &downloaded}
	_, err = io.Copy(file, io.TeeReader(resp.Body, counter))
	close(done)
	fmt.Println()
	if err != nil {
		return err
	}

	return fs.Move(outputPath+".part", outputPath)
}

type writeCounter struct {
	total *int64
}

func (wc *writeCounter) Write(p []byte) (int, error) {
	n := len(p)
	atomic.AddInt64(wc.total, int64(n))
	return n, nil
}
