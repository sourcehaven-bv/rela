package predicate

import "strings"

// utf8BOM is the three-byte UTF-8 byte-order mark. Some editors and
// upstream tools stamp this at the start of a file; a verbatim
// pass-through would surface as a confusing lexer error.
const utf8BOM = "\xef\xbb\xbf"

// preprocess strips a leading UTF-8 BOM and rejects sources whose first
// significant token is the Lua statement keyword `return` — that case
// would otherwise interact with the synthetic "return " prefix the
// parser front-end prepends, yielding a confusing parse error.
//
// Returns the cleaned source (BOM-stripped) and either nil or a
// *CompileError describing why the source is unacceptable.
func preprocess(src string) (string, error) {
	cleaned := strings.TrimPrefix(src, utf8BOM)
	if startsWithReturnKeyword(cleaned) {
		return "", &CompileError{
			Reason: "source must be an expression, not a statement (leading 'return' rejected)",
		}
	}
	return cleaned, nil
}

// startsWithReturnKeyword reports whether the first significant token
// of s — skipping whitespace and Lua comments — is the keyword `return`.
//
// "Significant" excludes: whitespace runs (\t \n \r space), single-line
// comments starting with --, and long-bracket block comments starting
// with --[=*[ ... ]=*] for any nesting level (matched by counting `=`
// characters; RR-3XZY).
func startsWithReturnKeyword(s string) bool {
	i := 0
	for i < len(s) {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			i++
			continue
		case '-':
			if i+1 < len(s) && s[i+1] == '-' {
				i += 2
				// long-bracket comment --[=*[ ... ]=*] for any level.
				// Count the `=` chars between the two `[`s and match
				// the same number in the closing `]=*]`.
				if level, opened := matchLongBracketOpen(s, i); opened {
					i += 2 + level // skip '[' + '='*level + '['
					if end := findLongBracketClose(s, i, level); end >= 0 {
						i = end + 2 + level // skip ']' + '='*level + ']'
						continue
					}
					// Unterminated long comment — let the parser report.
					return false
				}
				// short comment to end of line
				if nl := strings.IndexByte(s[i:], '\n'); nl >= 0 {
					i += nl + 1
					continue
				}
				return false
			}
			return false
		default:
			// First non-whitespace, non-comment byte. Check whether the
			// next token is the bare word `return`. Lua identifiers are
			// [A-Za-z_][A-Za-z0-9_]*; the keyword must be followed by a
			// non-ident byte or end-of-source.
			const kw = "return"
			if !strings.HasPrefix(s[i:], kw) {
				return false
			}
			end := i + len(kw)
			if end == len(s) {
				return true
			}
			c := s[end]
			if isIdentTail(c) {
				return false
			}
			return true
		}
	}
	return false
}

func isIdentTail(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// matchLongBracketOpen tests whether s[i:] starts with a Lua long
// bracket opening — `[`, zero or more `=`, then `[`. Returns the
// number of `=` chars (the "level") and whether a match was found.
// Level 0 is the conventional `[[ ... ]]`; higher levels embed inside
// each other.
func matchLongBracketOpen(s string, i int) (level int, ok bool) {
	if i >= len(s) || s[i] != '[' {
		return 0, false
	}
	j := i + 1
	for j < len(s) && s[j] == '=' {
		j++
	}
	if j >= len(s) || s[j] != '[' {
		return 0, false
	}
	return j - (i + 1), true
}

// findLongBracketClose searches s for the closing long bracket
// matching the given level, starting at i. Returns the index of the
// leading `]` of the close marker, or -1 if not found.
func findLongBracketClose(s string, i, level int) int {
	needle := "]" + strings.Repeat("=", level) + "]"
	idx := strings.Index(s[i:], needle)
	if idx < 0 {
		return -1
	}
	return i + idx
}
