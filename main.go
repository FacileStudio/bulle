package main

import (
	"errors"
	"os"
	"strings"

	"github.com/FacileStudio/bulle/cmd"
	"github.com/FacileStudio/bulle/internal/agent"
)

var version = "v0.82.0"

func main() {
	if err := cmd.Execute(version); err != nil {
		if _, used := errors.AsType[*agent.UsageError](err); used {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func unprefixed(err error) string {
	return strings.TrimPrefix(err.Error(), "bulle: ")
}
