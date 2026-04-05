package lua

import (
	"strings"
	"testing"
)

func FuzzStripShebang(f *testing.F) {
	f.Add("#!/usr/bin/env rela script\nprint('hello')")
	f.Add("")
	f.Add("#!")
	f.Add("#!/bin/sh\r\n")
	f.Add("print('hello')")
	f.Add("#not a shebang\ncode")
	f.Add("\xEF\xBB\xBF#!/usr/bin/env rela\ncode")

	f.Fuzz(func(t *testing.T, code string) {
		result := stripShebang(code)

		cleaned := strings.TrimPrefix(code, "\xEF\xBB\xBF")
		if !strings.HasPrefix(cleaned, "#!") {
			if result != cleaned {
				t.Errorf("non-shebang code was modified: input=%q result=%q", code, result)
			}
			return
		}

		// Code has shebang — result must not start with #!
		if strings.HasPrefix(result, "#!") {
			t.Errorf("shebang not stripped: input=%q result=%q", code, result)
		}

		// If input had a newline after shebang, line count must be preserved
		if idx := strings.Index(cleaned, "\n"); idx != -1 {
			inputLines := strings.Count(cleaned, "\n")
			resultLines := strings.Count(result, "\n")
			if resultLines != inputLines {
				t.Errorf("line count changed: input=%d result=%d", inputLines, resultLines)
			}
		}
	})
}
