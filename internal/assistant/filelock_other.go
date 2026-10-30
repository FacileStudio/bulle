//go:build !(linux || darwin || freebsd || netbsd || openbsd)

package assistant

func lockFile(string, bool, bool) (func(), error) {
	return func() {}, nil
}
