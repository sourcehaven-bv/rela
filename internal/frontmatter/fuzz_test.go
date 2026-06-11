package frontmatter

import "testing"

// FuzzSplit asserts Split never panics and never invents content: the
// combined frontmatter + body length stays within a small constant of
// the input (accounting for join overhead).
func FuzzSplit(f *testing.F) {
	seeds := []string{
		"---\nkey: value\n---\nbody",
		"",
		"no frontmatter",
		"---\n---\n",
		"---\nunclosed",
		"body\n---\nkey: value\n---\nmore body",
		"---\r\nkey: value\r\n---\r\nbody",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		fm, body := Split(input)
		if len(fm)+len(body) > len(input)+100 {
			t.Errorf("output larger than input: fm=%d, body=%d, input=%d",
				len(fm), len(body), len(input))
		}
	})
}
