//go:build !windows

package cli

import (
	"os"
	"os/exec"
)

// tryMuteTerminal disables stdin echo on POSIX terminals via `stty -echo` and
// returns a restore function. It returns false when /dev/tty is not present
// or stty is unavailable so callers can fall through to visible-echo input.
func tryMuteTerminal() (bool, func()) {
	if _, err := os.Stat("/dev/tty"); err != nil {
		return false, func() {}
	}
	stty, err := exec.LookPath("stty")
	if err != nil {
		return false, func() {}
	}

	off := exec.Command(stty, "-echo")
	off.Stdin = os.Stdin
	if err := off.Run(); err != nil {
		return false, func() {}
	}
	return true, func() {
		on := exec.Command(stty, "echo")
		on.Stdin = os.Stdin
		_ = on.Run()
	}
}
