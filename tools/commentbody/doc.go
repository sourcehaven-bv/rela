// Command commentbody fails when Go code carries comments that a better name
// or a smaller function would make unnecessary (TKT-GL18KOA).
//
// It reports two things in non-test, non-generated files:
//
//   - a comment inside a function body, closures included;
//   - a function doc comment longer than docCap prose lines.
//
// Directives (//go:, //nolint:, //plimsoll: and the like) never count, and
// neither does a comment that starts "why:" and gives a reason, which is the
// escape hatch for the rare fact no name can carry.
//
// Existing findings are held in baseline.json as a per-file count that may
// only shrink: a higher count fails, and so does a lower one until the
// baseline is regenerated with -update, so progress cannot be undone later.
package main
