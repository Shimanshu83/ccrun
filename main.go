package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func main() {

	// read os args and run things based on os args | in my case I have to read this os args
	// os.Args | executable-name | then other arguments
	parseCommandArgs(os.Args)
}

func parseCommandArgs(args []string) {

	fmt.Println("total number of time this started")
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

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWCGROUP,
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
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	time.Sleep(time.Minute * 45)

	cmd.Run()
}
