package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"
)

// revealTimeout bounds the call to open(1), which returns as soon as Finder
// has the request.
const revealTimeout = 10 * time.Second

// revealArgs is the open(1) command line that selects path in a Finder
// window. "--" keeps a path that starts with "-" from being read as a flag.
func revealArgs(path string) []string {
	return []string{"-R", "--", path}
}

// revealInFinder selects path in a Finder window, as File > Show in Finder
// does in every Mac document app.
// coverage-ignore-func: starts Finder
func revealInFinder(path string) error {
	if path == "" {
		return errors.New("no project is open")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), revealTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/open", revealArgs(path)...).CombinedOutput()
	if err != nil && len(out) > 0 {
		return errors.New(string(out))
	}
	return err
}
