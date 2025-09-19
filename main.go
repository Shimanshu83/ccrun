package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {

	// read os args and run things based on os args | in my case I have to read this os args
	// os.Args | executable-name | then other arguments
	parseCommandArgs(os.Args)
}

func parseCommandArgs(args []string) {

	fmt.Println("Container started <->")
	fmt.Println("args", os.Args)

	if len(args) < 2 {
		fmt.Println("no commands passed")
		os.Exit(1)
	}

	if args[1] != "run" {
		fmt.Println("command not supported")
		os.Exit(1)
	}

	if os.Getenv("IS_CHILD") == "1" {
		child()
		return
	}

	run()
}

func run() {

	cmd := exec.Command("/proc/self/exe", os.Args[1:]...)
	cmd.Env = append(os.Environ(), "IS_CHILD=1")

	if err := makeDir(ROOTFS_DIR); err != nil {
		panic(err)
	}

	if err := setupContainerNetworkingFiles(CONTAINER_DIR); err != nil {
		panic(err)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWCGROUP,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	if err := syscall.Mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
		log.Fatal("failed to make / rslave:", err)
	}

	if err := syscall.Mount(ROOTFS_DIR, ROOTFS_DIR, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		log.Fatal("failed to rbind rootfs:", err)
	}

	if err := syscall.Mount("", ROOTFS_DIR, "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		log.Fatal("failed to make rootfs private:", err)
	}

	procPath := ROOTFS_DIR + "/proc"
	if err := os.MkdirAll(procPath, 0755); err != nil {
		log.Fatal("failed to create proc dir:", err)
	}

	if err := syscall.Mount("proc", procPath, "proc", 0, ""); err != nil {
		log.Fatal("failed to mount proc:", err)
	}

	// if err := setupSysAndCgroup(ROOTFS_DIR); err != nil {
	// log.Fatal("error while mounting sys and cgrougp %w", err)
	// }
	if err := bindEtcFiles(ROOTFS_DIR, CONTAINER_DIR); err != nil {

		log.Fatal("failed to mount etc files %w", err)
	}

	if err := setupSysAndCgroup(ROOTFS_DIR); err != nil {
		log.Fatal("failed to mount root dir %w", err)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic(err)
	}

}

func child() {

	if err := syscall.Sethostname([]byte("container")); err != nil {
		panic(err)
	}
	if err := switchRoot(ROOTFS_DIR); err != nil {
		log.Fatal("error while pivoting a root %w", err)
	}

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run()
}

func setupSysAndCgroup(rootfs string) error {
	// Create $ROOTFS_DIR/sys
	sysDir := rootfs + "/sys"
	if err := os.MkdirAll(sysDir, 0755); err != nil {
		return fmt.Errorf("failed to create sys dir: %w", err)
	}

	// Mount sysfs -> $ROOTFS_DIR/sys
	if err := exec.Command("mount", "-t", "sysfs", "-o", "ro,nosuid,nodev,noexec", "sysfs", sysDir).Run(); err != nil {
		return fmt.Errorf("failed to mount sysfs: %w", err)
	}

	// Create $ROOTFS_DIR/sys/fs/cgroup
	cgroupDir := sysDir + "/fs/cgroup"
	if err := os.MkdirAll(cgroupDir, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup dir: %w", err)
	}

	// Mount cgroup2 -> $ROOTFS_DIR/sys/fs/cgroup
	if err := exec.Command("mount", "-t", "cgroup2", "-o", "ro,nosuid,nodev,noexec", "cgroup2", cgroupDir).Run(); err != nil {
		return fmt.Errorf("failed to mount cgroup2: %w", err)
	}

	return nil
}

func setupContainerNetworkingFiles(containerDir string) error {

	hostsContent := `127.0.0.1       localhost container-2
					::1             localhost ip6-localhost ip6-loopback `

	if err := os.WriteFile(containerDir+"/hosts", []byte(hostsContent), 0644); err != nil {
		return fmt.Errorf("failed to write hosts: %w", err)
	}

	// 2. Write /etc/hostname equivalent
	hostnameContent := CONTAINER_NAME

	if err := os.WriteFile(containerDir+"/hostname", []byte(hostnameContent), 0644); err != nil {
		return fmt.Errorf("failed to write hostname: %w", err)
	}

	// copying a resolve.conf file from host to docker container
	src, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return fmt.Errorf("failed to open resolv.conf: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(containerDir + "/resolv.conf")
	if err != nil {
		return fmt.Errorf("failed to create container resolv.conf: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy resolv.conf: %w", err)
	}

	return nil
}

func bindEtcFiles(rootfsDir, containerDir string) error {
	files := []string{"hostname", "hosts", "resolv.conf"}

	for _, p := range files {
		target := rootfsDir + "/etc/" + p
		source := containerDir + "/" + p

		// Ensure the target file exists (like `touch`)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			if f, err := os.Create(target); err != nil {
				return fmt.Errorf("failed to create %s: %w", target, err)
			} else {
				f.Close()
			}
		}

		// Bind mount source -> target
		cmd := exec.Command("mount", "--bind", source, target)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to bind %s to %s: %w", source, target, err)
		}
	}

	return nil
}

func switchRoot(newRoot string) error {
	// Step 1: make rootfs rslave so mounts don’t propagate back to the host
	if err := syscall.Mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
		return err
	}

	// Step 2: create /.oldroot inside newRoot
	oldRoot := newRoot + "/.oldroot"
	if err := os.MkdirAll(oldRoot, 0700); err != nil {
		return err
	}

	// Step 3: pivot_root(newRoot, oldRoot)
	if err := syscall.PivotRoot(newRoot, oldRoot); err != nil {
		return err
	}

	// Step 4: change directory to new root "/"
	if err := os.Chdir("/"); err != nil {
		return err
	}

	// Step 5: unmount old root
	if err := syscall.Unmount("/.oldroot", syscall.MNT_DETACH); err != nil {
		return err
	}

	// Step 6: remove /.oldroot directory
	if err := os.RemoveAll("/.oldroot"); err != nil {
		return err
	}

	return nil
}
