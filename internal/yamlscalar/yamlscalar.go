// Package yamlscalar holds checks on single YAML values that several
// config loaders share.
//
// It is a leaf: it imports nothing from rela.
package yamlscalar

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// PlainYAML11Bool reports whether n is a scalar that YAML 1.1 reads as a
// boolean and YAML 1.2 reads as text: not single- or double-quoted, not
// tagged `!!str`, and, trimmed of whitespace, one of yes, no, on, off, y
// or n in any case.
//
// A loader refuses such a value where it takes a boolean or a text, so
// `confirm: yes` does not silently become the text "yes". The trim also
// catches a block scalar (`|`), whose value ends in a newline. An
// explicit `!!str` tag states that the value is text, as quotes do.
//
// Nil: accepted. A nil node is no value, so it reports false.
func PlainYAML11Bool(n *yaml.Node) bool {
	if n == nil || n.Kind != yaml.ScalarNode || n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
		return false
	}
	if n.Style&yaml.TaggedStyle != 0 && n.ShortTag() == "!!str" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(n.Value)) {
	case "yes", "no", "y", "n", "on", "off":
		return true
	}
	return false
}
