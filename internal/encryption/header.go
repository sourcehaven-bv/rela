package encryption

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// Header is the small metadata block prepended to every sealed
// file's plaintext before it reaches age.Encrypt. On read the block
// is parsed back out; the caller enforces whatever invariants it
// cares about (version monotonicity, path match) and the remaining
// bytes — identical to what the caller wrote — flow on to the
// higher layer.
//
// Two fields carry load-bearing information:
//
//   - Version is the monotonic repo-encryption version; the cryptofs
//     layer uses it to detect rollback of any single sealed file
//     against per-machine last-seen-version state.
//   - Path is the repo-relative path this blob was written to; the
//     cryptofs layer uses it to detect swap/rename attacks (the
//     adversary renames sealed A to B; age is happy, path check
//     isn't).
type Header struct {
	Version int
	Path    string
}

// headerMagic is the first token of every sealed-file header.
// Guards against mis-parsing a file that wasn't written by rela's
// cryptofs layer (e.g. someone sealed a plain age blob by hand).
const headerMagic = "rela"

// headerTerminator is the byte that ends the header line. The rest
// of the plaintext is opaque bytes — entity YAML frontmatter,
// markdown body, binary attachment content — and stays untouched.
const headerTerminator = '\n'

// Encode writes the header as a single ASCII line terminated by \n.
// Format:
//
//	rela v=<version> path=<repo-relative-path>\n
//
// Keeping the format positional/prefixed rather than YAML avoids
// collision with entity files' own YAML frontmatter delimiter (---)
// and halves the per-file overhead vs. a multi-line YAML header.
// Key=value shape (rather than purely positional) tolerates paths
// that might contain unusual characters without re-quoting rules.
func (h *Header) Encode() []byte {
	var buf bytes.Buffer
	buf.WriteString(headerMagic)
	buf.WriteByte(' ')
	buf.WriteString("v=")
	buf.WriteString(strconv.Itoa(h.Version))
	buf.WriteByte(' ')
	buf.WriteString("path=")
	buf.WriteString(h.Path)
	buf.WriteByte(headerTerminator)
	return buf.Bytes()
}

// ErrMalformedHeader is returned when cryptofs tries to parse a
// sealed blob's plaintext but the header line is missing or
// syntactically invalid. This indicates either a corrupted file,
// a blob written by an older rela version (none exist yet — the
// feature is unreleased), or a blob crafted by a party that
// didn't use the rela encryption path.
var ErrMalformedHeader = errors.New("encryption: malformed rela header")

// ParseHeader reads the header line from the start of plaintext and
// returns both the parsed Header and the remaining body bytes (what
// the caller originally wrote before Encode prepended the header).
//
// Parsing is tolerant of field order (v=X path=Y and path=Y v=X are
// both accepted) but strict on the magic token and on field presence.
// Unknown tokens return ErrMalformedHeader — we'd rather refuse a
// file we don't fully understand than silently drop metadata.
func ParseHeader(plaintext []byte) (*Header, []byte, error) {
	idx := bytes.IndexByte(plaintext, headerTerminator)
	if idx < 0 {
		return nil, nil, fmt.Errorf("%w: no terminator", ErrMalformedHeader)
	}
	line := plaintext[:idx]
	body := plaintext[idx+1:]

	tokens := bytes.Split(line, []byte(" "))
	if len(tokens) == 0 || string(tokens[0]) != headerMagic {
		return nil, nil, fmt.Errorf("%w: bad magic", ErrMalformedHeader)
	}

	h := &Header{Version: -1}
	for _, tok := range tokens[1:] {
		eq := bytes.IndexByte(tok, '=')
		if eq < 0 {
			return nil, nil, fmt.Errorf("%w: token %q missing =", ErrMalformedHeader, tok)
		}
		key, val := string(tok[:eq]), string(tok[eq+1:])
		switch key {
		case "v":
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: invalid version %q", ErrMalformedHeader, val)
			}
			h.Version = n
		case "path":
			h.Path = val
		default:
			return nil, nil, fmt.Errorf("%w: unknown token %q", ErrMalformedHeader, key)
		}
	}

	if h.Version < 0 {
		return nil, nil, fmt.Errorf("%w: missing v=", ErrMalformedHeader)
	}
	if h.Path == "" {
		return nil, nil, fmt.Errorf("%w: missing path=", ErrMalformedHeader)
	}
	return h, body, nil
}
