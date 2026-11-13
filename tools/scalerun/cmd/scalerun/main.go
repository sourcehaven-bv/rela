// Command scalerun seeds the perf project into PostgreSQL at one or more
// scales, measures rela-server's key endpoints against each, and checks the
// numbers against tools/scalerun/scalerun.yaml. See the "PostgreSQL Scale Check"
// guide (docs/postgres-scale-run.md) and `just scale-run`.
//
//	scalerun run   -rela BIN -server BIN [-config FILE] [-project DIR] [-scales 1,5] [-out DIR] [-port N] [-keep]
//	scalerun check -results FILE [-config FILE] [-report FILE] [-bench-old FILE -bench-new FILE] [-bench-note TEXT]
//
// run reads the DSN from RELA_DATABASE_URL, like rela itself: never from a
// flag, so it stays out of ps and shell history. Each scale gets its own
// schema, scalerun_s<scale>, which run drops and recreates; point it at a
// scratch database. check exits 1 when a limit is exceeded.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sourcehaven-BV/rela/tools/scalerun"
)

const (
	defaultConfig = "tools/scalerun/scalerun.yaml"
	// defaultPort is unlikely to clash with a dev server (rela-server's
	// default is 8080).
	defaultPort = 18790
)

func main() {
	os.Exit(realMain())
}

// realMain returns the exit code, so main's deferred calls run before the
// process exits.
func realMain() int {
	if len(os.Args) < 2 {
		usage()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	switch os.Args[1] {
	case "run":
		err = runCmd(ctx, os.Args[2:])
	case "check":
		err = checkCmd(os.Args[2:])
	default:
		usage()
	}
	var failed failedError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &failed):
		fmt.Fprintln(os.Stderr, "scalerun:", err)
		return 1
	default:
		fmt.Fprintln(os.Stderr, "scalerun:", err)
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: scalerun run|check [flags]; see the package doc")
	os.Exit(2)
}

// failedError is a run whose measurements broke a limit, as opposed to a
// run that could not measure. The first exits 1, the second 2.
type failedError struct{ n int }

func (e failedError) Error() string { return fmt.Sprintf("%d finding(s); see the report", e.n) }

func runCmd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	o := runOptions{}
	fs.StringVar(&o.configPath, "config", defaultConfig, "scalerun config")
	fs.StringVar(&o.relaBin, "rela", "", "rela CLI built with -tags postgres")
	fs.StringVar(&o.serverBin, "server", "", "rela-server built with -tags postgres")
	fs.StringVar(&o.project, "project", "prototypes/perf/project", "perf project to copy and seed")
	scales := fs.String("scales", "1,5", "comma-separated seed scales; the config's base scale must be one")
	fs.StringVar(&o.out, "out", ".ignored/scalerun/out", "directory for results.json, logs and project copies")
	fs.IntVar(&o.port, "port", defaultPort, "HTTP port for rela-server")
	fs.BoolVar(&o.keep, "keep", false, "keep each scale's schema instead of dropping it")
	fs.StringVar(&o.commit, "commit", "", "commit id to record in the results")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if o.relaBin == "" || o.serverBin == "" {
		return errors.New("-rela and -server are required")
	}
	o.dsn = os.Getenv("RELA_DATABASE_URL")
	if o.dsn == "" {
		return errors.New("RELA_DATABASE_URL is not set; point it at a scratch database")
	}
	cfg, err := scalerun.LoadConfig(o.configPath)
	if err != nil {
		return err
	}
	if o.scales, err = parseScales(*scales, cfg.Ratio.BaseScale); err != nil {
		return err
	}
	return run(ctx, cfg, o)
}

func checkCmd(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	configPath := fs.String("config", defaultConfig, "scalerun config")
	resultsPath := fs.String("results", ".ignored/scalerun/out/results.json", "results.json written by run")
	reportPath := fs.String("report", "", "write the markdown report here (default: next to the results)")
	benchOld := fs.String("bench-old", "", "previous run's benchmark output (go test -bench); may be missing")
	benchNew := fs.String("bench-new", "", "this run's benchmark output")
	benchNote := fs.String("bench-note", "", "report note when -bench-new is missing, e.g. that the benchmarks failed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := scalerun.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	n, err := check(cfg, *resultsPath, *reportPath, benchInput{old: *benchOld, new: *benchNew, note: *benchNote})
	if err != nil {
		return err
	}
	if n > 0 {
		return failedError{n}
	}
	return nil
}
