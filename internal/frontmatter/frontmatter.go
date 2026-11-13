// Package frontmatter splits a YAML-frontmatter markdown document into
// its raw frontmatter block and body. It is a dependency-free leaf: it
// returns only strings — no YAML parsing, no domain types — so both
// internal/markdown (the general parser) and internal/store/fsstore
// (the storage backend, which deliberately does not import
// internal/markdown) can share one implementation without coupling.
//
// rela's on-disk format is a YAML block fenced by "---" delimiters
// followed by a markdown body:
//
//	---
//	id: REQ-001
//	type: requirement
//	---
//	# Body
//
// Callers own yaml.Unmarshal of the returned frontmatter block and any
// higher-level concerns (git-conflict-marker detection, type mapping).
package frontmatter

import "strings"

// Delimiter is the fence that opens and closes a frontmatter block.
const Delimiter = "---"

// Split separates the YAML frontmatter block from the markdown body.
//
// The first run of a delimiter-only line opens the block and the next
// closes it; everything before the opening delimiter and after the
// closing one is body. A document with no opening delimiter is all
// body (frontmatter == "").
//
// Lines are split on "\n" — NOT via bufio.Scanner, which caps a single
// line at bufio.MaxScanTokenSize (64 KB) and errors past it. A markdown
// file with one long line (a base64 data: URI, a minified blob, a
// pasted log) must round-trip: writable AND readable. Splitting on "\n"
// has no per-line limit. A trailing "\r" on any line is stripped (CRLF
// tolerance), matching the scanner's previous behavior; a trailing
// newline does not yield a final empty body line.
func Split(content string) (frontmatter, body string) {
	var bodyLines, fmLines []string
	inFrontmatter := false
	frontmatterEnded := false

	for _, line := range splitLines(content) {
		if !inFrontmatter && !frontmatterEnded && strings.TrimSpace(line) == Delimiter {
			inFrontmatter = true
			continue
		}
		if inFrontmatter && isClosingDelimiter(line) {
			inFrontmatter = false
			frontmatterEnded = true
			continue
		}
		if inFrontmatter {
			fmLines = append(fmLines, line)
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	frontmatter = strings.Join(fmLines, "\n")
	if len(fmLines) > 0 {
		// Every frontmatter line was terminated in the file, including the
		// last one before the closing delimiter. Without restoring that
		// terminator a clip block scalar ("|") as the final property reads
		// back one newline short: "0\n" came back as "0", silently and only
		// for the last key (BUG-NWQA0E).
		frontmatter += "\n"
	}
	body = strings.TrimPrefix(strings.Join(bodyLines, "\n"), "\n")
	return frontmatter, body
}

// isClosingDelimiter reports whether a line inside the frontmatter block
// closes it. The delimiter must start at column 0; trailing whitespace is
// tolerated. An indented "---" is YAML content: yaml.v3 writes a block
// scalar whose text has a "---" line (a value "a\n---\nb", or a key "---\n"
// emitted as "? |") with that line indented. Trimming the indent closed the
// block there, so the property was truncated or renamed and the rest of the
// YAML leaked into the body (BUG-EXFOEI3).
func isClosingDelimiter(line string) bool {
	return strings.TrimRight(line, " \t") == Delimiter
}

// splitLines splits content into lines with the same observable
// semantics bufio.Scanner had — trailing "\r" stripped per line, no
// final empty line from a trailing "\n" — but with no per-line size cap.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	content = strings.TrimSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}
