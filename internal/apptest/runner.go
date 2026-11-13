// Package apptest runs `rela test`: the operator's `*.test.lua` files, each
// case against a fresh in-memory world built from the project's config.
//
// A case runs on its own [appbuild.NewInMemory] world: a memstore searched by
// LinearSearch, every write in memory, config read from disk. The world is
// built when the case first writes or reads, after the setup calls
// (t.fixtures, t.principal) have said what it holds. Nothing under the
// project root is written, and the live store, its locks and its database
// are never opened.
//
// A test file is Lua in the script sandbox (no io, os, debug or load*). It
// gets the test module with `local t = require("rela.test")` and declares
// cases with `t.case(name, fn)`. The module is documented on [fileRun.module].
//
// Generated values are reproducible: each case draws from a seed derived from
// the run seed, the file and the case name, so a rerun with the same seed
// gives every case the same values and starting clock, and adding a case does
// not change another case's values. Entity ids and randomness inside
// automations do not come from that seed and are not reproducible.
package apptest

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/rootfs"
)

// TestsDir is the directory under the project root that holds test files.
const TestsDir = "tests"

// FileSuffix ends the name of every test file found in a directory.
const FileSuffix = ".test.lua"

// DefaultCaseTimeout bounds one case when [Options.CaseTimeout] is zero.
const DefaultCaseTimeout = time.Minute

// ErrNoTests is returned when no test file was found.
var ErrNoTests = errors.New("no tests found")

// Options configures [Run].
type Options struct {
	// Root is the project root. Required.
	Root string
	// Paths are test files or directories, absolute or relative to Root,
	// that must lie inside Root. Empty means the tests/ directory.
	Paths []string
	// Run, when set, is a regular expression; only cases whose name it
	// matches run.
	Run string
	// Seed is the run seed. When SeedSet is false a random seed is chosen
	// and reported.
	Seed    uint64
	SeedSet bool
	// CaseTimeout bounds one case. Zero means [DefaultCaseTimeout].
	CaseTimeout time.Duration
}

// MissingConfigError says that config the test world reads from disk is
// not there. A project that keeps its config only in rela.db cannot be
// tested yet.
type MissingConfigError struct {
	Files []string
}

func (e *MissingConfigError) Error() string {
	return fmt.Sprintf("rela test reads the project's config from disk, and %s is missing. "+
		"A project that keeps its config only in rela.db cannot be tested yet; "+
		"export it with `rela db dump` first. Testing against the database is planned in TKT-DAUS290",
		strings.Join(e.Files, ", "))
}

// requiredConfig are the config files a test world cannot be built without.
var requiredConfig = []string{"schema.yaml"}

// CheckConfig returns a [*MissingConfigError] when a required config file
// is not on disk under root.
func CheckConfig(root string) error {
	var missing []string
	for _, name := range requiredConfig {
		if _, err := os.Stat(filepath.Join(root, name)); errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return &MissingConfigError{Files: missing}
	}
	return nil
}

// Run runs the test files opts names and returns the report. The error is
// for a run that could not start: missing config, a bad path or pattern,
// or [ErrNoTests]. A failing case is a result in the report, not an error.
func Run(ctx context.Context, opts Options) (*Report, error) {
	if opts.Root == "" {
		return nil, errors.New("apptest: project root is required")
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	if cerr := CheckConfig(root); cerr != nil {
		return nil, cerr
	}
	var filter *regexp.Regexp
	if opts.Run != "" {
		if filter, err = regexp.Compile(opts.Run); err != nil {
			return nil, fmt.Errorf("--run: %w", err)
		}
	}
	files, err := discover(root, opts.Paths)
	if err != nil {
		return nil, err
	}
	seed := opts.Seed
	if !opts.SeedSet {
		seed = randomSeed()
	}
	rep := &Report{Seed: seed, Cases: []CaseResult{}}
	if len(files) == 0 {
		return rep, ErrNoTests
	}
	policy, err := loadPolicy(ctx, root)
	if err != nil {
		return nil, err
	}
	timeout := opts.CaseTimeout
	if timeout <= 0 {
		timeout = DefaultCaseTimeout
	}
	r := &runner{root: root, files: rootfs.New(root), seed: seed, filter: filter, policy: policy, timeout: timeout}
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		r.runFile(ctx, f, rep)
	}
	return rep, nil
}

// runner holds what every file and case of one run shares.
type runner struct {
	root    string
	files   *rootfs.Dir
	seed    uint64
	filter  *regexp.Regexp
	timeout time.Duration
	// policy is acl.yaml as parsed once for the run, used to check the
	// roles a case names. Each world parses its own copy. Nil: accepted —
	// the project has no acl.yaml.
	policy *acl.Policy
}

// loadPolicy parses root/acl.yaml, or returns nil when there is none.
func loadPolicy(ctx context.Context, root string) (*acl.Policy, error) {
	data, err := rootfs.New(root).Load(ctx, "acl.yaml")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil //nolint:nilnil // no acl.yaml: every principal may do everything
	}
	if err != nil {
		return nil, fmt.Errorf("read acl.yaml: %w", err)
	}
	return acl.ParsePolicy(data, "acl.yaml")
}

// caseSeed derives the seed of one case, or of a file's top level when name
// is empty, from the run seed.
func (r *runner) caseSeed(file, name string) uint64 {
	h := fnv.New64a()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], r.seed)
	_, _ = h.Write(b[:])
	_, _ = h.Write([]byte(file))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(name))
	return h.Sum64()
}

// Seed-derived clock: a case without t.clock starts at an instant within
// clockSpan after clockEpoch, chosen by its seed.
var clockEpoch = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

const clockSpan = 10 * 365 * 24 * time.Hour

// startClock is the instant a case's clock starts at.
func startClock(seed uint64) time.Time {
	return clockEpoch.Add(time.Duration(seed%uint64(clockSpan/time.Second)) * time.Second)
}

func randomSeed() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return uint64(time.Now().UnixNano())
	}
	// Keep it below 2^53 so it survives a JSON reader that parses numbers
	// as float64.
	return binary.LittleEndian.Uint64(b[:]) & (1<<53 - 1)
}

// discover returns the test files paths name, relative to root with forward
// slashes, sorted. A directory contributes the files below it that end in
// [FileSuffix]; a file is taken as named. Symbolic links are not followed.
func discover(root string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		dir := filepath.Join(root, TestsDir)
		if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		paths = []string{dir}
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, p)
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil || !filepath.IsLocal(rel) {
			return nil, fmt.Errorf("%s is outside the project %s", p, root)
		}
		info, err := os.Lstat(abs)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("%s is not a regular file", p)
			}
			out = appendNew(out, seen, filepath.ToSlash(rel))
			continue
		}
		err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), FileSuffix) {
				return nil
			}
			r, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			out = appendNew(out, seen, filepath.ToSlash(r))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	slices.Sort(out)
	return out, nil
}

func appendNew(out []string, seen map[string]bool, s string) []string {
	if seen[s] {
		return out
	}
	seen[s] = true
	return append(out, s)
}
