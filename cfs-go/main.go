package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// mimic this interface:
// docker run image <cmd> <params>
// go run main.go

func main() {
	switch os.Args[1] {
	case "run":
		run()
	case "child":
		child()
	default:
		panic("bad command")
	}
}

func run() {
	fmt.Printf("Running [parent]: %v as %d\n", os.Args[2:], os.Getpid())

	// call itself
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// create namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// CLONE_NEWNS was the first namespace invented - is actually for mounts
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,

		// do not share namespace with the host
		// proc mount from container will not be shared on the host
		Unshareflags: syscall.CLONE_NEWNS,
	}

	err := cmd.Run()
	if err != nil {
		fmt.Printf("%v", err)
	}
}
func child() {
	fmt.Printf("Running [child]: %v as %d\n", os.Args[2:], os.Getpid())

	cg()

	// should already be in new namespace
	syscall.Sethostname([]byte("container"))
	syscall.Chroot("/container/ubuntu-fs")
	syscall.Chdir("/")
	syscall.Mount("proc", "proc", "proc", 0, "")

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run()

	syscall.Unmount("/proc", 0)
}

func cg() {
	cgroups := "/sys/fs/cgroup/"
	pids := filepath.Join(cgroups, "pids")
	err := os.Mkdir(filepath.Join(pids, "cam"), 0755)
	if err != nil && os.IsExist(err) {
		panic(err)
	}

	// make a limit of 20 processes
	must(ioutil.WriteFile(filepath.Join(pids, "cam/pids.max"), []byte("20"), 0700))

	// Removes the new cgroup in place after the container exists
	must(ioutil.WriteFile(filepath.Join(pids, "cam/notify_on_release"), []byte("1"), 0700))
	must(ioutil.WriteFile(filepath.Join(pids, "cam/cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
