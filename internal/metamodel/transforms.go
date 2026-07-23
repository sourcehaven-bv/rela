package metamodel

import (
	"errors"
	"fmt"
	"mime"
	"sort"
	"strings"
)

// transformFormatMarkdown is the only supported transform input format in v1.
// Kept as a local constant (rather than importing internal/transform) so the
// dependency stays one-way: transform imports metamodel, not the reverse.
const transformFormatMarkdown = "markdown"

// validateTransforms checks the top-level `transforms:` registry. Every entry
// must have a non-empty command, an input `from` of "markdown", and a
// well-formed `produces` media type (RR-VYTL35: the value is echoed into the
// export response Content-Type, so a malformed or CRLF-bearing value would be a
// header-injection vector). A bad entry fails the whole metamodel load so a
// broken registry can never half-work at request time.
func validateTransforms(m *Metamodel) []string {
	var errs []string

	// Deterministic order so error output is stable.
	names := make([]string, 0, len(m.Transforms))
	for name := range m.Transforms {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		def := m.Transforms[name]
		if strings.TrimSpace(name) == "" {
			errs = append(errs, "transform has an empty name")
			continue
		}

		from := def.From
		if from == "" {
			from = transformFormatMarkdown // default; markdown is the only value today
		}
		if from != transformFormatMarkdown {
			errs = append(errs, fmt.Sprintf(
				"transform %q: unsupported from: %q (only %q is supported)", name, from, transformFormatMarkdown))
		}

		if len(def.Command) == 0 {
			errs = append(errs, fmt.Sprintf("transform %q: command is required", name))
		} else if strings.TrimSpace(def.Command[0]) == "" {
			errs = append(errs, fmt.Sprintf("transform %q: command[0] (the binary) is empty", name))
		}

		if def.Produces == "" {
			errs = append(errs, fmt.Sprintf("transform %q: produces (content-type) is required", name))
		} else if err := validateMediaType(def.Produces); err != nil {
			errs = append(errs, fmt.Sprintf("transform %q: invalid produces %q: %v", name, def.Produces, err))
		}
	}

	return errs
}

// validateMediaType rejects malformed media types and any control character
// (notably CR/LF, which would break out of the Content-Type header).
func validateMediaType(v string) error {
	if strings.ContainsAny(v, "\r\n") {
		return errors.New("contains a newline")
	}
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return errors.New("contains a control character")
		}
	}
	if _, _, err := mime.ParseMediaType(v); err != nil {
		return err
	}
	return nil
}
