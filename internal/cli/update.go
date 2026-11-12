package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"omasword/internal/update"
)

// Exit codes for `omasword update`. 0, 1, and 3 mean the same as for
// passages and searches.
const (
	ExitUpdateAvailable = 4
	ExitUpdateFailed    = 5
)

// Seams for tests: the updater and the path of the binary to replace.
var (
	newUpdater = update.New
	executable = update.Executable
)

func runUpdate(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("omasword update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	check := fs.Bool("check", false, "")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: omasword update [--check]")
		return ExitUsage
	}
	u := newUpdater()
	ctx := context.Background()
	rel, err := u.Latest(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return updateExit(err)
	}
	newer, err := update.Newer(rel.Tag, u.Current)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUpdateFailed
	}
	if !newer {
		fmt.Fprintf(stdout, "omasword %s is up to date.\n", u.Current)
		return ExitOK
	}
	if *check {
		fmt.Fprintf(stdout, "omasword %s is available (you have %s). Run omasword update to install it.\n", rel.Tag, u.Current)
		return ExitUpdateAvailable
	}
	exe, err := executable()
	if err != nil {
		fmt.Fprintf(stderr, "%v: %v\n", update.ErrNotWritable, err)
		return ExitUpdateFailed
	}
	if err := u.Install(ctx, rel, exe); err != nil {
		fmt.Fprintln(stderr, err)
		return updateExit(err)
	}
	fmt.Fprintf(stdout, "Updated omasword %s to %s (%s).\n", u.Current, rel.Tag, exe)
	return ExitOK
}

func updateExit(err error) int {
	if errors.Is(err, update.ErrFetch) {
		return ExitFetch
	}
	return ExitUpdateFailed
}
