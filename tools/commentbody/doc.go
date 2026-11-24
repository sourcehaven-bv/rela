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
// only shrink: a higher count fails. A lower count only prints a
// note, so a PR that removes comments cannot break an unrelated PR; run
// -update to lock the gain in.
package main
