package settings

import (
	"fmt"
	"os"
	"path/filepath"
)

func mergeInto(from, to string) {
	entries, err := os.ReadDir(from)
	if err != nil {
		return
	}
	for _, e := range entries {
		renameIfMissing(filepath.Join(from, e.Name()), filepath.Join(to, e.Name()))
	}
	removeIfEmpty(from)
}

func removeIfEmpty(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}
	if err := os.Remove(dir); err != nil {
		fmt.Fprintf(os.Stderr, "bulle: removing %s: %v\n", dir, err)
	}
}

func dropRuntimeLeftovers(legacy string) {
	removeIfEmpty(filepath.Join(legacy, "ide"))
	sock := filepath.Join(legacy, "chat.sock")
	if info, err := os.Lstat(sock); err == nil && info.Mode()&os.ModeSocket != 0 {
		if err := os.Remove(sock); err != nil {
			fmt.Fprintf(os.Stderr, "bulle: removing %s: %v\n", sock, err)
		}
	}
}

func renameIfMissing(from, to string) {
	if missing(from) || !missing(to) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "bulle: creating %s: %v\n", filepath.Dir(to), err)
		return
	}
	if err := move(from, to); err != nil {
		fmt.Fprintf(os.Stderr, "bulle: moving %s to %s: %v\n", from, to, err)
	}
}

func move(from, to string) error {
	info, err := os.Lstat(from)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return os.Rename(from, to)
	}
	target, err := os.Readlink(from)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(from), target)
	}
	if err := os.Symlink(target, to); err != nil {
		return err
	}
	return os.Remove(from)
}

func missing(path string) bool {
	_, err := os.Lstat(path)
	return os.IsNotExist(err)
}
