package fsimport

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// relaClass is what the import does with one entry of the source's .rela/.
type relaClass int

const (
	relaUnknown relaClass = iota
	relaConfig            // copied as a file (copyRelaConfig)
	relaState             // copied into the target's state KV
	relaSkip              // not copied, for the reason given
)

// relaState entries are runtime state that the file tier keeps in .rela/ and
// the database tiers keep in their state KV, under the same key. A trailing
// slash names a prefix: every file below it is a key.
var relaStateKeys = []string{
	"caldav/aliases.json",
	"user-defaults.yaml",
	"palette.yaml",
	"theme/",
	"next-action-state.json",
	"scheduler-run-state.json",
	"migration/",
}

// relaSkipped entries are caches, locks and state with no meaning in the
// target, with the reason the report gives.
var relaSkipped = map[string]string{
	"comments/":               "comments are copied into the database",
	"documents/":              "rendered-document cache; rebuilt on demand",
	"search/":                 "search index; rebuilt on first open",
	"fsstore-index.json":      "filesystem index cache",
	"pending-deletes.json":    "undoable deletes are not imported",
	"migration.lock":          "lock file",
	"scheduler-run-children/": "progress of scheduler runs in flight; not resumed",
}

// classifyRela returns what the import does with the .rela-relative path p,
// a file. reason is set for relaSkip.
func classifyRela(p string) (class relaClass, reason string) {
	first, _, _ := strings.Cut(p, "/")
	if _, ok := relaConfigFiles[p]; ok || first == relaAuditDir {
		return relaConfig, ""
	}
	for _, k := range relaStateKeys {
		if p == k || (strings.HasSuffix(k, "/") && strings.HasPrefix(p, k)) {
			return relaState, ""
		}
	}
	for k, why := range relaSkipped {
		if p == k || (strings.HasSuffix(k, "/") && strings.HasPrefix(p, k)) {
			return relaSkip, why
		}
	}
	return relaUnknown, ""
}

// copyState copies the runtime state keys from the source's .rela/ into the
// target's state KV, and lists every other .rela entry the import does not
// handle elsewhere.
func (c *copier) copyState(ctx context.Context) error {
	root, err := os.OpenRoot(c.src.root)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if _, err := root.Stat(".rela"); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	listed := map[string]bool{} // skipped folders, reported once
	return fs.WalkDir(root.FS(), ".rela", func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		p := strings.TrimPrefix(full, ".rela/")
		class, reason := classifyRela(p)
		switch class {
		case relaConfig:
			return nil
		case relaSkip:
			c.skipRela(full, p, reason, listed)
			return nil
		case relaUnknown:
			c.rep.skip(full, "not a file rela keeps in .rela/; not copied")
			return nil
		case relaState:
			// Copied below.
		}
		if !d.Type().IsRegular() {
			c.rep.skip(full, "not a regular file")
			return nil
		}
		c.copyStateKey(ctx, full, p)
		return nil
	})
}

// skipRela lists one skipped .rela file. A skipped folder is one line, not
// one per file; comment threads get their own per-thread lines from the
// comment copy.
func (c *copier) skipRela(full, p, reason string, listed map[string]bool) {
	if strings.HasPrefix(p, "comments/") {
		if !strings.HasSuffix(p, ".yaml") {
			c.rep.skip(full, "not a comment thread file")
		}
		return
	}
	if dir, _, nested := strings.Cut(p, "/"); nested {
		if !listed[dir] {
			listed[dir] = true
			c.rep.skip(".rela/"+dir+"/", "%s", reason)
		}
		return
	}
	c.rep.skip(full, "%s", reason)
}

// copyStateKey copies one state file into the target's state.KV.
func (c *copier) copyStateKey(ctx context.Context, full, key string) {
	data, err := c.src.state.Get(ctx, key)
	if err != nil {
		c.rep.fail("read state %s: %v", full, err)
		return
	}
	if key == schedulerStateKey {
		if data, err = filterSchedulerState(data); err != nil {
			c.rep.fail("read state %s: %v", full, err)
			return
		}
	}
	if err := c.dst.State.Put(ctx, key, data); err != nil {
		c.rep.fail("write state %s: %v", full, err)
		return
	}
	c.rep.StateKeys++
}

// schedulerStateKey is the scheduler's run-state document
// (schedulerstate/kvstate.StateKey).
const schedulerStateKey = "scheduler-run-state.json"

// sourceStateKeys lists the .rela-relative paths copyState copies.
func sourceStateKeys(root string) ([]string, error) {
	var keys []string
	dir := filepath.Join(root, ".rela")
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	err := filepath.WalkDir(dir, func(full string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(dir, full)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if class, _ := classifyRela(rel); class == relaState {
			keys = append(keys, rel)
		}
		return nil
	})
	return keys, err
}

// filterSchedulerState keeps each task's schedule position (last run,
// failures, next retry) and drops the record of runs. A run in flight in the
// source cannot be resumed in the target, and its subject keys are not
// copied; carrying the run over would leave it waiting forever.
func filterSchedulerState(data []byte) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	delete(doc, "runs")
	return json.Marshal(doc)
}
