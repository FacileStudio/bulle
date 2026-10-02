package ide

import (
	"os"
	"strings"
	"sync/atomic"
)

// EnvVar turns the IDE surface on for every session in this environment,
// without a flag on each one.
const EnvVar = "BULLE_IDE"

// switchOn is the switch the command line parsed --ide into.
var switchOn atomic.Pointer[bool]

// BindFlag records the switch the command line parsed --ide into. The command
// package owns flag parsing and this one only needs the answer, so the flag
// stays declared in exactly one place.
func BindFlag(on *bool) {
	switchOn.Store(on)
}

// Enabled reports whether this process publishes to an editor: --ide was
// passed, or $BULLE_IDE holds a true word. A session with the surface off
// creates nothing at all — no socket, no file, no goroutine, no hook.
func Enabled() bool {
	if on := switchOn.Load(); on != nil && *on {
		return true
	}
	return truthy(os.Getenv(EnvVar))
}

// truthy reads the words that mean yes out of a switch's value. Empty, "0",
// "false", "no" and "off" leave the surface off, so a stray BULLE_IDE=0 in a
// shell profile does not open a socket somebody has to wonder about.
func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
