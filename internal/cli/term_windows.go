//go:build windows

package cli

// tryMuteTerminal is a no-op on Windows. Operators are expected to use
// --api-key for non-interactive logins or pipe the key on stdin; piping is
// always quiet regardless of console echo state.
func tryMuteTerminal() (bool, func()) {
	return false, func() {}
}
