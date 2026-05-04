//go:build !windows

package driver

import "syscall"

func signalStop(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

func processAlive(pid int) (bool, error) {
	if err := syscall.Kill(pid, 0); err != nil {
		return false, nil
	}
	return true, nil
}
