package core

import "syscall"

type FdComm struct {
	Fd int
}

func (fd FdComm) Write(b []byte) (int, error) {
	return syscall.Write(fd.Fd, b)
}

func (fd FdComm) Read(b []byte) (int, error) {
	return syscall.Read(fd.Fd, b)
}