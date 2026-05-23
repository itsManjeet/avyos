/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"avyos.dev/api/distro"
	"avyos.dev/lib/ini"
	"avyos.dev/lib/net"
)

const defaultShell = "/bin/sh"

var (
	linuxBase = "/linux"
)

const defaultURL = "https://dl-cdn.alpinelinux.org/alpine/v3.23/releases/<arch>/alpine-minirootfs-3.23.3-<arch>.tar.gz"

func init() {
	if os.Geteuid() != 0 {
		linuxBase = filepath.Join(os.Getenv("HOME"), "Linux")
	}
}

func resolveURL() string {
	u := defaultURL
	u = strings.ReplaceAll(u, "<goarch>", runtime.GOARCH)
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	default:
		arch = runtime.GOARCH
	}
	u = strings.ReplaceAll(u, "<arch>", arch)
	return u
}

func distroStatus() (bool, string, int) {
	binPath := filepath.Join(linuxBase, "bin")
	if _, err := os.Stat(binPath); err != nil {
		return false, linuxBase, 0
	}
	return true, linuxBase, getDirSize(linuxBase)
}

func installDistro(customURL string) error {
	customURL = strings.TrimSpace(customURL)

	binPath := filepath.Join(linuxBase, "bin")
	if _, err := os.Stat(binPath); err == nil {
		return nil // already installed
	}

	u := customURL
	if u == "" {
		u = resolveURL()
	}

	if err := os.MkdirAll(linuxBase, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := downloadAndExtract(u, linuxBase); err != nil {
		_ = os.RemoveAll(linuxBase)
		return fmt.Errorf("failed to download: %w", err)
	}
	if err := validateExtractedRootfs(linuxBase); err != nil {
		// _ = os.RemoveAll(linuxBase)
		// return fmt.Errorf("invalid rootfs archive: %w", err)
	}

	for _, dir := range []string{"proc", "sys", "dev", "tmp", "root"} {
		_ = os.MkdirAll(filepath.Join(linuxBase, dir), 0755)
	}

	return nil
}

func runContainer(req distro.RunRequest, uid uint32) (distro.RunResult, error) {
	if _, err := os.Stat(filepath.Join(linuxBase, "bin")); os.IsNotExist(err) {
		return distro.RunResult{}, fmt.Errorf("distro not installed (use 'distro install' first)")
	}

	workdir := strings.TrimSpace(req.Workdir)
	if workdir == "" {
		workdir = "/"
	}

	waylandBridge, err := newWaylandBridge(uid)
	if err != nil {
		return distro.RunResult{}, fmt.Errorf("setup wayland bridge: %w", err)
	}
	defer waylandBridge.Close()

	command := distro.DecodeCommand(req.Command)
	return execContainer(linuxBase, command, workdir, req.Bind, req.Env, req.Input, waylandBridge.Env(), waylandBridge.RuntimeHost())
}

func uninstallDistro() error {
	if _, err := os.Stat(linuxBase); os.IsNotExist(err) {
		return fmt.Errorf("distro not installed")
	}

	if err := os.RemoveAll(linuxBase); err != nil {
		return fmt.Errorf("failed to remove: %w", err)
	}
	return nil
}

func runInit() error {
	rootfs := os.Getenv("DISTRO_ROOTFS")
	command := distro.DecodeCommand(os.Getenv("DISTRO_COMMAND"))
	workdir := os.Getenv("DISTRO_WORKDIR")
	bind := os.Getenv("DISTRO_BIND")
	waylandRuntimeHost := os.Getenv(distroWaylandRuntimeHostEnv)

	if rootfs == "" || len(command) == 0 {
		return fmt.Errorf("invalid distro configuration")
	}
	if workdir == "" {
		workdir = "/"
	}

	if err := setupContainerFS(rootfs, bind, waylandRuntimeHost); err != nil {
		return fmt.Errorf("failed to setup filesystem: %w", err)
	}

	if err := pivotRoot(rootfs); err != nil {
		return fmt.Errorf("failed to pivot root: %w", err)
	}

	if err := os.Chdir(workdir); err != nil {
		_ = os.Chdir("/")
	}

	path, err := exec.LookPath(command[0])
	if err != nil {
		path = command[0]
	}

	return syscall.Exec(path, command, os.Environ())
}

func execContainer(rootfs string, command []string, workdir, bind, envVar string, input []byte, extraEnv []string, waylandRuntimeHost string) (distro.RunResult, error) {
	exePath, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return distro.RunResult{}, fmt.Errorf("resolve executable path: %w", err)
	}

	cmd := exec.Command(exePath, "init")
	cmd.Stdin = bytes.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	env := os.Environ()
	env = setEnv(env, "PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	env = setEnv(env, "HOME", "/root")
	env = setEnv(env, "USER", "root")
	env = setEnv(env, "LOGNAME", "root")
	env = setEnv(env, "TERM", "xterm-256color")
	env = setEnv(env, "LANG", "C.UTF-8")
	env = setEnv(env, "DISTRO_ROOTFS", rootfs)
	env = setEnv(env, "DISTRO_COMMAND", distro.EncodeCommand(command))
	env = setEnv(env, "DISTRO_WORKDIR", workdir)
	env = setEnv(env, "DISTRO_BIND", bind)
	env = setEnvMany(env, waylandBaseEnv())
	if strings.TrimSpace(waylandRuntimeHost) != "" {
		env = setEnv(env, distroWaylandRuntimeHostEnv, waylandRuntimeHost)
	}
	env = setEnvMany(env, extraEnv)
	if strings.TrimSpace(envVar) != "" {
		env = setEnvKV(env, envVar)
	}
	cmd.Env = env

	attr := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}
	if os.Geteuid() != 0 {
		attr.Cloneflags |= syscall.CLONE_NEWUSER
		attr.UidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Geteuid(), Size: 1},
		}
		attr.GidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getegid(), Size: 1},
		}
		attr.GidMappingsEnableSetgroups = false
	}
	cmd.SysProcAttr = attr

	result := distro.RunResult{}
	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = int32(exitErr.ExitCode())
		} else {
			return distro.RunResult{}, err
		}
	}

	if result.ExitCode == 0 {
		result.ExitCode = int32(cmd.ProcessState.ExitCode())
	}
	result.Stdout = append(result.Stdout[:0], stdout.Bytes()...)
	result.Stderr = append(result.Stderr[:0], stderr.Bytes()...)
	return result, nil
}

func setupContainerFS(rootfs, bind, waylandRuntimeHost string) error {
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("private mount propagation: %w", err)
	}
	if err := mountWithIgnoreBusy(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mount rootfs: %w", err)
	}

	if err := mountProcFS(rootfs); err != nil {
		return err
	}
	if err := mountSysFS(rootfs); err != nil {
		serviceLog.Debug("container sysfs mount skipped: %v", err)
	}
	if err := mountTmpFS(rootfs); err != nil {
		return err
	}
	if err := mountRunFS(rootfs); err != nil {
		return err
	}
	if err := mountDevFS(rootfs); err != nil {
		return err
	}
	if err := applyBindMounts(rootfs, bind); err != nil {
		return err
	}
	applyDefaultGUIBindMounts(rootfs)
	if err := setupWaylandRuntimeMount(rootfs, waylandRuntimeHost); err != nil {
		return err
	}

	generateResolvConf(rootfs)
	return nil
}

type bindMountSpec struct {
	source string
	target string
}

func mountProcFS(rootfs string) error {
	procPath := filepath.Join(rootfs, "proc")
	if err := os.MkdirAll(procPath, 0555); err != nil {
		return fmt.Errorf("create /proc: %w", err)
	}

	procSource := "/proc"
	if _, err := os.Stat(procSource); err == nil {
		if err := mountBind(procSource, procPath); err != nil {
			return fmt.Errorf("bind %s to /proc: %w", procSource, err)
		}
		return nil
	}

	if err := mountWithIgnoreBusy("proc", procPath, "proc", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("mount /proc fallback: %w", err)
	}
	return nil
}

func mountSysFS(rootfs string) error {
	sysPath := filepath.Join(rootfs, "sys")
	if err := os.MkdirAll(sysPath, 0555); err != nil {
		return fmt.Errorf("create /sys: %w", err)
	}
	if err := mountWithIgnoreBusy("sysfs", sysPath, "sysfs", syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("mount /sys: %w", err)
	}
	return nil
}

func mountTmpFS(rootfs string) error {
	tmpPath := filepath.Join(rootfs, "tmp")
	if err := os.MkdirAll(tmpPath, 1777); err != nil {
		return fmt.Errorf("create /tmp: %w", err)
	}
	if err := mountWithIgnoreBusy("tmpfs", tmpPath, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=1777"); err != nil {
		return fmt.Errorf("mount /tmp: %w", err)
	}
	return nil
}

func mountRunFS(rootfs string) error {
	runPath := filepath.Join(rootfs, "run")
	if err := os.MkdirAll(runPath, 0755); err != nil {
		return fmt.Errorf("create /run: %w", err)
	}
	if err := mountWithIgnoreBusy("tmpfs", runPath, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=755"); err != nil {
		return fmt.Errorf("mount /run: %w", err)
	}
	return nil
}

func mountDevFS(rootfs string) error {
	devPath := filepath.Join(rootfs, "dev")
	if err := os.MkdirAll(devPath, 0755); err != nil {
		return fmt.Errorf("create /dev: %w", err)
	}
	if err := mountWithIgnoreBusy("tmpfs", devPath, "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755"); err != nil {
		return fmt.Errorf("mount /dev: %w", err)
	}

	for _, dev := range []string{"null", "zero", "random", "urandom", "tty"} {
		src := filepath.Join("/dev", dev)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := bindMountIntoRootfs(rootfs, src, filepath.Join("/dev", dev)); err != nil {
			return fmt.Errorf("bind /dev/%s: %w", dev, err)
		}
	}

	ptsPath, err := containerPath(rootfs, "/dev/pts")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ptsPath, 0755); err != nil {
		return fmt.Errorf("create /dev/pts: %w", err)
	}
	if err := mountWithIgnoreBusy("devpts", ptsPath, "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=620"); err != nil {
		return fmt.Errorf("mount /dev/pts: %w", err)
	}

	shmPath, err := containerPath(rootfs, "/dev/shm")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(shmPath, 1777); err != nil {
		return fmt.Errorf("create /dev/shm: %w", err)
	}
	if err := mountWithIgnoreBusy("tmpfs", shmPath, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=1777"); err != nil {
		return fmt.Errorf("mount /dev/shm: %w", err)
	}

	ptmxPath := filepath.Join(devPath, "ptmx")
	_ = os.Remove(ptmxPath)
	if err := os.Symlink("pts/ptmx", ptmxPath); err != nil && !os.IsExist(err) {
		return fmt.Errorf("link /dev/ptmx: %w", err)
	}

	if err := symlinkIfMissing("/proc/self/fd", filepath.Join(devPath, "fd")); err != nil {
		return fmt.Errorf("link /dev/fd: %w", err)
	}
	if err := symlinkIfMissing("/proc/self/fd/0", filepath.Join(devPath, "stdin")); err != nil {
		return fmt.Errorf("link /dev/stdin: %w", err)
	}
	if err := symlinkIfMissing("/proc/self/fd/1", filepath.Join(devPath, "stdout")); err != nil {
		return fmt.Errorf("link /dev/stdout: %w", err)
	}
	if err := symlinkIfMissing("/proc/self/fd/2", filepath.Join(devPath, "stderr")); err != nil {
		return fmt.Errorf("link /dev/stderr: %w", err)
	}

	return nil
}

func applyBindMounts(rootfs, bind string) error {
	specs, err := parseBindMounts(bind)
	if err != nil {
		return err
	}
	for _, spec := range specs {
		if err := bindMountIntoRootfs(rootfs, spec.source, spec.target); err != nil {
			return fmt.Errorf("bind mount %s:%s: %w", spec.source, spec.target, err)
		}
	}
	return nil
}

func applyDefaultGUIBindMounts(rootfs string) {
	specs := make([]bindMountSpec, 0, 16)
	seen := make(map[string]struct{})

	addDeviceBind := func(rel string) {
		rel = strings.TrimPrefix(filepath.Clean("/"+strings.TrimSpace(rel)), "/")
		if rel == "" || rel == "." {
			return
		}

		src := filepath.Join("/dev", rel)
		if _, err := os.Stat(src); err != nil {
			return
		}

		key := src + "->" + rel
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		specs = append(specs, bindMountSpec{
			source: src,
			target: filepath.Join("/dev", rel),
		})
	}

	for _, rel := range []string{"dri", "snd", "fb0", "fb"} {
		addDeviceBind(rel)
	}

	for _, pattern := range []string{"dri/*", "card*", "renderD*"} {
		matches, err := filepath.Glob(filepath.Join("/dev", pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			rel, err := filepath.Rel("/dev", match)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			addDeviceBind(rel)
		}
	}

	for _, spec := range specs {
		if err := bindMountIntoRootfs(rootfs, spec.source, spec.target); err != nil {
			serviceLog.Debug("optional gui bind %s:%s skipped: %v", spec.source, spec.target, err)
		}
	}
}

func parseBindMounts(bind string) ([]bindMountSpec, error) {
	bind = strings.TrimSpace(bind)
	if bind == "" {
		return nil, nil
	}

	entries := strings.Split(bind, ",")
	specs := make([]bindMountSpec, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid bind %q (expected host:container)", entry)
		}

		source := strings.TrimSpace(parts[0])
		target := strings.TrimSpace(parts[1])
		if source == "" || target == "" {
			return nil, fmt.Errorf("invalid bind %q (expected host:container)", entry)
		}
		specs = append(specs, bindMountSpec{
			source: source,
			target: target,
		})
	}
	return specs, nil
}

func setupWaylandRuntimeMount(rootfs, runtimeHost string) error {
	runtimePath, err := containerPath(rootfs, distroWaylandRuntime)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(runtimePath, 0777); err != nil {
		return fmt.Errorf("create %s: %w", distroWaylandRuntime, err)
	}
	_ = os.Chmod(runtimePath, 0777)

	runtimeHost = strings.TrimSpace(runtimeHost)
	if runtimeHost == "" {
		return fmt.Errorf("missing wayland runtime host path")
	}
	if _, err := os.Stat(runtimeHost); err != nil {
		return fmt.Errorf("wayland runtime host path %q: %w", runtimeHost, err)
	}
	if err := mountBind(runtimeHost, runtimePath); err != nil {
		return fmt.Errorf("bind mount wayland runtime: %w", err)
	}
	_ = os.Chmod(runtimePath, 0777)

	socketPath := filepath.Join(runtimePath, distroWaylandDisplay)
	info, err := os.Lstat(socketPath)
	if err != nil {
		return fmt.Errorf("wayland socket %q missing after mount: %w", socketPath, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("wayland socket %q is not a socket", socketPath)
	}
	return nil
}

func bindMountIntoRootfs(rootfs, source, target string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return fmt.Errorf("empty source path")
	}

	targetPath, err := containerPath(rootfs, target)
	if err != nil {
		return err
	}
	return mountBind(source, targetPath)
}

func mountBind(source, target string) error {
	if err := ensureMountTarget(source, target); err != nil {
		return err
	}

	srcInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat source %q: %w", source, err)
	}

	flags := uintptr(syscall.MS_BIND)
	if srcInfo.IsDir() {
		flags |= syscall.MS_REC
	}

	if err := mountWithIgnoreBusy(source, target, "", flags, ""); err != nil {
		return fmt.Errorf("bind %q -> %q: %w", source, target, err)
	}
	return nil
}

func ensureMountTarget(source, target string) error {
	srcInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat source %q: %w", source, err)
	}

	targetInfo, err := os.Stat(target)
	if err == nil {
		if srcInfo.IsDir() && !targetInfo.IsDir() {
			return fmt.Errorf("target %q exists and is not a directory", target)
		}
		if !srcInfo.IsDir() && targetInfo.IsDir() {
			return fmt.Errorf("target %q exists and is a directory", target)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("stat target %q: %w", target, err)
	}

	if srcInfo.IsDir() {
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("create target directory %q: %w", target, err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("create target parent %q: %w", filepath.Dir(target), err)
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("create target file %q: %w", target, err)
	}
	return f.Close()
}

func containerPath(rootfs, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("empty container path")
	}
	clean := filepath.Clean("/" + strings.TrimPrefix(target, "/"))
	return filepath.Join(rootfs, strings.TrimPrefix(clean, "/")), nil
}

func mountWithIgnoreBusy(source, target, fstype string, flags uintptr, data string) error {
	err := syscall.Mount(source, target, fstype, flags, data)
	if err == nil || errors.Is(err, syscall.EBUSY) {
		return nil
	}
	return err
}

func symlinkIfMissing(target, linkName string) error {
	if _, err := os.Lstat(linkName); err == nil {
		return nil
	}
	return os.Symlink(target, linkName)
}

func pivotRoot(rootfs string) error {
	oldRoot := filepath.Join(rootfs, ".old_root")
	if err := os.MkdirAll(oldRoot, 0700); err != nil {
		return err
	}

	if err := syscall.PivotRoot(rootfs, oldRoot); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return err
	}

	oldRootNew := "/.old_root"
	if err := syscall.Unmount(oldRootNew, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root: %w", err)
	}

	_ = os.RemoveAll(oldRootNew)
	return nil
}

func downloadAndExtract(u, targetDir string) error {
	client := net.NewClient()

	resp, err := client.Get(u)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http error: %s", resp.Status)
	}

	lowerURL := strings.ToLower(strings.TrimSpace(u))
	archivePath := lowerURL
	if parsed, err := url.Parse(lowerURL); err == nil && parsed.Path != "" {
		archivePath = strings.ToLower(parsed.Path)
	}
	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"), strings.HasSuffix(archivePath, ".tgz"):
		return extractTarGz(resp.Body, targetDir)
	// case strings.HasSuffix(archivePath, ".tar.xz"), strings.HasSuffix(archivePath, ".txz"):
	// 	return extractTarXz(resp.Body, targetDir)
	case strings.HasSuffix(archivePath, ".tar"):
		return extractTar(resp.Body, targetDir)
	}

	binPath := filepath.Join(targetDir, "bin")
	_ = os.MkdirAll(binPath, 0755)

	parts := strings.Split(u, "/")
	filename := parts[len(parts)-1]
	if filename == "" {
		filename = "binary"
	}

	outFile, err := os.Create(filepath.Join(binPath, filename))
	if err != nil {
		return err
	}
	defer outFile.Close()
	_ = outFile.Chmod(0755)
	_, err = io.Copy(outFile, resp.Body)
	return err
}

func extractTarGz(r io.Reader, targetDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	return extractTar(gzr, targetDir)
}

// func extractTarXz(r io.Reader, targetDir string) error {
// 	xzr, err := xz.NewReader(r)
// 	if err != nil {
// 		return fmt.Errorf("create xz reader: %w", err)
// 	}

// 	return extractTar(xzr, targetDir)
// }

func extractTar(r io.Reader, targetDir string) error {
	base := filepath.Clean(targetDir)
	prefix := base + string(os.PathSeparator)
	tr := tar.NewReader(r)
	entries := 0

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(targetDir, header.Name)
		cleanTarget := filepath.Clean(target)
		if cleanTarget != base && !strings.HasPrefix(cleanTarget, prefix) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTarget, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create directory %q: %w", cleanTarget, err)
			}
			entries++
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
				return fmt.Errorf("create parent directory for %q: %w", cleanTarget, err)
			}
			f, err := os.Create(cleanTarget)
			if err != nil {
				return fmt.Errorf("create file %q: %w", cleanTarget, err)
			}
			n, copyErr := io.Copy(f, tr)
			closeErr := f.Close()
			if copyErr != nil {
				return fmt.Errorf("write file %q: %w", cleanTarget, copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close file %q: %w", cleanTarget, closeErr)
			}
			if n != header.Size {
				return fmt.Errorf("incomplete file %q: wrote %d of %d bytes", cleanTarget, n, header.Size)
			}
			if err := os.Chmod(cleanTarget, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("set file mode on %q: %w", cleanTarget, err)
			}
			entries++
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
				return fmt.Errorf("create parent directory for symlink %q: %w", cleanTarget, err)
			}
			if err := os.Remove(cleanTarget); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove existing path %q: %w", cleanTarget, err)
			}
			if err := os.Symlink(header.Linkname, cleanTarget); err != nil {
				return fmt.Errorf("create symlink %q -> %q: %w", cleanTarget, header.Linkname, err)
			}
			entries++
		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
				return fmt.Errorf("create parent directory for hardlink %q: %w", cleanTarget, err)
			}
			linkTarget := filepath.Clean(filepath.Join(targetDir, header.Linkname))
			if linkTarget != base && !strings.HasPrefix(linkTarget, prefix) {
				return fmt.Errorf("invalid hardlink target %q", header.Linkname)
			}
			if err := os.Remove(cleanTarget); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove existing path %q: %w", cleanTarget, err)
			}
			if err := os.Link(linkTarget, cleanTarget); err != nil {
				return fmt.Errorf("create hardlink %q -> %q: %w", cleanTarget, linkTarget, err)
			}
			entries++
		}
	}

	if entries == 0 {
		return fmt.Errorf("archive contains no extractable entries")
	}

	return nil
}

func validateExtractedRootfs(rootfs string) error {
	if hasRootfsShell(rootfs) {
		return nil
	}
	if err := flattenSingleTopLevelRootfs(rootfs); err != nil {
		return err
	}
	if hasRootfsShell(rootfs) {
		return nil
	}

	artifacts := findDiskImageArtifacts(rootfs)
	if len(artifacts) > 0 {
		return fmt.Errorf(
			"archive contains VM disk image(s): %s; expected a rootfs tar archive with /bin/sh",
			strings.Join(artifacts, ", "),
		)
	}

	return fmt.Errorf("archive does not contain a Linux rootfs (missing /bin/sh)")
}

func hasRootfsShell(rootfs string) bool {
	candidates := []string{
		filepath.Join(rootfs, "bin", "sh"),
		filepath.Join(rootfs, "usr", "bin", "sh"),
		filepath.Join(rootfs, "bin", "bash"),
		filepath.Join(rootfs, "usr", "bin", "bash"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func flattenSingleTopLevelRootfs(rootfs string) error {
	entries, err := os.ReadDir(rootfs)
	if err != nil {
		return fmt.Errorf("read extracted rootfs directory: %w", err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}

	top := filepath.Join(rootfs, entries[0].Name())
	if !hasRootfsShell(top) {
		return nil
	}

	children, err := os.ReadDir(top)
	if err != nil {
		return fmt.Errorf("read nested rootfs directory %q: %w", top, err)
	}
	for _, child := range children {
		src := filepath.Join(top, child.Name())
		dst := filepath.Join(rootfs, child.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("flatten rootfs from %q to %q: %w", src, dst, err)
		}
	}
	if err := os.Remove(top); err != nil {
		return fmt.Errorf("remove nested rootfs directory %q: %w", top, err)
	}

	return nil
}

func findDiskImageArtifacts(rootfs string) []string {
	entries, err := os.ReadDir(rootfs)
	if err != nil {
		return nil
	}

	artifacts := make([]string, 0, 4)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".raw") ||
			strings.HasSuffix(name, ".qcow2") ||
			strings.HasSuffix(name, ".img") ||
			strings.HasSuffix(name, ".vmdk") {
			artifacts = append(artifacts, entry.Name())
		}
	}
	return artifacts
}

func getDirSize(path string) int {
	total := 0
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += int(info.Size())
		}
		return nil
	})
	return total
}

// generateResolvConf writes /etc/resolv.conf inside the container rootfs
// using DNS servers from /etc/net.conf.
func generateResolvConf(rootfs string) {
	netConf := "/etc/net.conf"
	cfg, err := ini.ParseFile(netConf)
	if err != nil {
		return
	}

	servers, ok := cfg.Get("dns", "servers")
	if !ok || strings.TrimSpace(servers) == "" {
		return
	}

	etcDir := filepath.Join(rootfs, "etc")
	_ = os.MkdirAll(etcDir, 0755)

	var buf strings.Builder
	buf.WriteString("# Generated by avyos distro service\n")
	for s := range strings.SplitSeq(servers, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			buf.WriteString("nameserver ")
			buf.WriteString(s)
			buf.WriteByte('\n')
		}
	}

	_ = os.WriteFile(filepath.Join(etcDir, "resolv.conf"), []byte(buf.String()), 0644)
}
