package tokenstore

import "fmt"

// Name is a validated connection name. Construct one with [ParseName]; the
// zero value is not a valid name.
//
// The grammar is principal.ValidIntegrationName's, so `integration:<name>`
// names the identity a connector runs as. It is duplicated rather than
// imported because this package may depend only on state; a test asserts the
// two agree.
type Name string

// maxNameLen bounds a connection name.
const maxNameLen = 63

// ParseName validates s as a connection name: 1 to 63 characters of
// lowercase letters, digits, '_' and '-', starting with a letter or digit,
// and not a Windows reserved device name.
func ParseName(s string) (Name, error) {
	if !validName(s) {
		return "", fmt.Errorf("invalid connection name %q: use 1-63 lowercase letters, digits, "+
			"'_' or '-', starting with a letter or digit", s)
	}
	return Name(s), nil
}

func validName(s string) bool {
	if s == "" || len(s) > maxNameLen {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case (c == '_' || c == '-') && i > 0:
		default:
			return false
		}
	}
	return !windowsReserved[s]
}

// windowsReserved lists device names the filesystem state backend cannot
// store as a key segment.
var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// stateKey is where [Sealed] keeps the token for n.
func (n Name) stateKey() string { return "tokens/" + string(n) }
