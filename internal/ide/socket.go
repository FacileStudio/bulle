package ide

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// SocketPath is where this process listens: $XDG_RUNTIME_DIR/bulle/<pid>.sock,
// or $TMPDIR/bulle-<uid>/<pid>.sock when the runtime directory is unset. Both
// are per-user directories, which matters because what arrives on the socket
// can answer an approval.
func SocketPath(pid int) (string, error) {
	name := fmt.Sprintf("%d.sock", pid)
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "bulle", name), nil
	}
	uid := os.Getuid()
	if uid < 0 {
		return "", errors.New("no user id to build the IDE socket path from")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("bulle-%d", uid), name), nil
}

// listen opens the socket, creating its directory 0700 so only its owner can
// reach the socket inside it. A file already at the path is a socket left by
// a process whose pid has come round again; it is cleared rather than treated
// as another listener, since the name is this process's own.
func listen(path string) (*net.UnixListener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := clearSocket(path); err != nil {
		return nil, err
	}
	return net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
}

// clearSocket removes a socket or discovery file left at the given path.
func clearSocket(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
