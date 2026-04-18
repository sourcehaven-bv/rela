// Package natsort provides natural sorting for strings containing numbers.
// It splits strings into text and numeric chunks and compares them so that
// embedded numbers are compared numerically rather than lexicographically.
// For example, "REQ-2" sorts before "REQ-10".
package natsort

import "sort"

// Compare returns -1, 0, or +1 comparing a and b using natural ordering.
// Text is compared case-insensitively with case used only as a final tiebreaker.
func Compare(a, b string) int {
	ia, ib := 0, 0
	la, lb := len(a), len(b)
	caseTiebreak := 0 // first case difference found (used only if otherwise equal)

	for ia < la && ib < lb {
		ca, cb := a[ia], b[ib]
		isDigitA := isDigit(ca)
		isDigitB := isDigit(cb)

		switch {
		case isDigitA && isDigitB:
			if c := compareNumChunks(a, b, &ia, &ib, la, lb); c != 0 {
				return c
			}
		case !isDigitA && !isDigitB:
			if c := compareTextChar(ca, cb); c != 0 {
				return c
			}
			if caseTiebreak == 0 {
				caseTiebreak = caseDiff(ca, cb)
			}
			ia++
			ib++
		default:
			return digitVsText(isDigitA)
		}
	}

	return compareRemaining(la-ia, lb-ib, caseTiebreak)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// compareTextChar compares two non-digit characters case-insensitively.
// Returns -1, 0, or 1.
func compareTextChar(ca, cb byte) int {
	lowA := toLower(ca)
	lowB := toLower(cb)
	if lowA < lowB {
		return -1
	}
	if lowA > lowB {
		return 1
	}
	return 0
}

// caseDiff returns the case tiebreak for two characters that are equal
// case-insensitively. Returns -1, 0, or 1.
func caseDiff(ca, cb byte) int {
	if ca < cb {
		return -1
	}
	if ca > cb {
		return 1
	}
	return 0
}

// digitVsText returns -1 if a digit should sort before text, or 1 otherwise.
func digitVsText(aIsDigit bool) int {
	if aIsDigit {
		return -1
	}
	return 1
}

// compareRemaining handles the tail comparison when one or both strings are exhausted.
func compareRemaining(remA, remB, caseTiebreak int) int {
	if remA < remB {
		return -1
	}
	if remA > remB {
		return 1
	}
	return caseTiebreak
}

// compareNumChunks compares two numeric chunks starting at positions ia and ib.
// It advances both pointers past the numeric chunks.
func compareNumChunks(a, b string, ia, ib *int, la, lb int) int {
	leadingZerosA := skipZeros(a, ia, la)
	leadingZerosB := skipZeros(b, ib, lb)

	// Find end of numeric part (significant digits)
	sigStartA, sigLenA := scanDigits(a, *ia, la)
	*ia = sigStartA + sigLenA
	sigStartB, sigLenB := scanDigits(b, *ib, lb)
	*ib = sigStartB + sigLenB

	// Different number of significant digits means different magnitude
	if sigLenA != sigLenB {
		if sigLenA < sigLenB {
			return -1
		}
		return 1
	}

	// Same length — compare digit by digit
	for k := range sigLenA {
		if a[sigStartA+k] != b[sigStartB+k] {
			if a[sigStartA+k] < b[sigStartB+k] {
				return -1
			}
			return 1
		}
	}

	// Numerically equal — fewer leading zeros first
	if leadingZerosA != leadingZerosB {
		if leadingZerosA < leadingZerosB {
			return -1
		}
		return 1
	}

	return 0
}

// skipZeros advances *pos past leading '0' digits and returns how many were skipped.
func skipZeros(s string, pos *int, length int) int {
	start := *pos
	for *pos < length && s[*pos] == '0' {
		*pos++
	}
	return *pos - start
}

// scanDigits returns the start position and count of consecutive digit characters.
func scanDigits(s string, start, length int) (pos, count int) {
	i := start
	for i < length && isDigit(s[i]) {
		i++
	}
	return start, i - start
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// Less reports whether a should sort before b using natural ordering.
func Less(a, b string) bool {
	return Compare(a, b) < 0
}

// Strings sorts a slice of strings in natural order.
func Strings(s []string) {
	sort.Slice(s, func(i, j int) bool {
		return Compare(s[i], s[j]) < 0
	})
}
