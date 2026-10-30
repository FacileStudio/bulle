package assistant

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	t     *testing.T
	files map[string]string
	fails map[string]*CmdError
	calls []string
}

func newFake(t *testing.T, files map[string]string) *fakeRunner {
	return &fakeRunner{t: t, files: files, fails: map[string]*CmdError{}}
}

func (f *fakeRunner) run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if e, ok := f.fails[key]; ok {
		return nil, e
	}
	val, ok := f.files[key]
	if !ok {
		return nil, fmt.Errorf("unexpected command: %s", key)
	}
	if strings.HasPrefix(val, "[") || strings.HasPrefix(val, "{") {
		return []byte(val), nil
	}
	data, err := os.ReadFile(filepath.Join("testdata", val))
	if err != nil {
		f.t.Fatal(err)
	}
	return data, nil
}
