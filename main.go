package main

import (
	"fmt"
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

	/* spawning a new child process here  */
	// cmd := exec.Command(args[2], args[3:]...)
	cmd := exec.Command("/proc/self/exe", os.Args[1:]...)
	cmd.Env = append(os.Environ(), "IS_CHILD=1")

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS,
	}

	/* attaching parent process i/o to child process i/o */
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
