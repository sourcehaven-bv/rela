package channels

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Caps on the files of one message, enforced at the store boundary. The
// service applies the configured, lower limits first; these hold for every
// caller.
const (
	// MaxFilesPerMessage caps the files of one message.
	MaxFilesPerMessage = 4
	// MaxFileBytes caps one file.
	MaxFileBytes = 10 << 20
	// MaxMessageFileBytes caps the files of one message together.
	MaxMessageFileBytes = 16 << 20
	// MaxFileNameBytes caps a file's display name.
	MaxFileNameBytes = 255
	// MaxMIMEBytes caps a file's MIME type.
	MaxMIMEBytes = 100
)

// File is one file a message holds. The store keeps the entries in the
// message and the bytes beside it, keyed by channel, seq and ID. An edit
// keeps the files; a removal erases the bytes and clears the entries with
// the rest of the message.
type File struct {
	// ID is minted by the store; see [ValidFileID].
	ID string `json:"id"`
	// Property is the message's file property that holds the file.
	Property string `json:"property"`
	// Name is the display name. Two files of one property never share it.
	// It names the file for a reader only; storage never uses it.
	Name string `json:"name"`
	// MIME is the sniffed type of the stored bytes.
	MIME string `json:"mime"`
	// Size is the number of stored bytes.
	Size int64 `json:"size"`
}

// NewFile is a file to append with a message. Its bytes are final: the
// caller has checked them against the property's policy, so the store
// only applies its caps and never reads a stream inside its lock or
// transaction.
type NewFile struct {
	Property string
	Name     string
	MIME     string
	Data     []byte
}

// fileIDLen is the length of a minted file id: 26 base32 characters, 130
// bits.
const fileIDLen = 26

var (
	fileIDPattern = regexp.MustCompile(`^[a-z2-7]{26}$`)
	// filePropertyPattern is the grammar of a message property name in
	// realtime.yaml.
	filePropertyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
)

// ValidFileID reports whether id has the shape of a minted file id, so a
// caller can refuse a malformed id before any lookup.
func ValidFileID(id string) bool { return fileIDPattern.MatchString(id) }

const fileIDAlphabet = "abcdefghijklmnopqrstuvwxyz234567"

func mintFileID() (string, error) {
	var buf [fileIDLen]byte
	if _, err := rand.Read(buf[:]); err != nil { // coverage-ignore: crypto/rand does not fail on supported platforms
		return "", fmt.Errorf("channels: mint file id: %w", err)
	}
	for i, b := range buf {
		buf[i] = fileIDAlphabet[b&31]
	}
	return string(buf[:]), nil
}

// PrepareFiles applies the file caps to files and mints an id for each. It
// returns the entries in the order of files, so a backend stores files[i]
// under out[i].ID. Nil files prepare to nil.
func PrepareFiles(files []NewFile) ([]File, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if len(files) > MaxFilesPerMessage {
		return nil, fmt.Errorf("%w: %d files, at most %d", ErrTooLarge, len(files), MaxFilesPerMessage)
	}
	out := make([]File, 0, len(files))
	total := 0
	names := map[[2]string]bool{}
	for _, f := range files {
		if err := checkNewFile(f); err != nil {
			return nil, err
		}
		total += len(f.Data)
		if total > MaxMessageFileBytes {
			return nil, fmt.Errorf("%w: the files are over %d bytes together", ErrTooLarge, MaxMessageFileBytes)
		}
		k := [2]string{f.Property, f.Name}
		if names[k] {
			return nil, fmt.Errorf("%w: two files named %q in property %s", ErrInvalidMessage, f.Name, f.Property)
		}
		names[k] = true
		id, err := mintFileID()
		if err != nil {
			return nil, err
		}
		out = append(out, File{ID: id, Property: f.Property, Name: f.Name, MIME: f.MIME, Size: int64(len(f.Data))})
	}
	return out, nil
}

func checkNewFile(f NewFile) error {
	switch {
	case !filePropertyPattern.MatchString(f.Property):
		return fmt.Errorf("%w: file property %q", ErrInvalidMessage, f.Property)
	case len(f.Data) == 0:
		return fmt.Errorf("%w: file %q is empty", ErrInvalidMessage, f.Name)
	case len(f.Data) > MaxFileBytes:
		return fmt.Errorf("%w: file %q is %d bytes, at most %d", ErrTooLarge, f.Name, len(f.Data), MaxFileBytes)
	case f.MIME == "" || len(f.MIME) > MaxMIMEBytes || strings.ContainsFunc(f.MIME, badNameRune):
		return fmt.Errorf("%w: file %q has a bad MIME type", ErrInvalidMessage, f.Name)
	}
	return checkFileName(f.Name)
}

func checkFileName(name string) error {
	switch {
	case name == "" || len(name) > MaxFileNameBytes:
		return fmt.Errorf("%w: a file name has 1 to %d bytes", ErrInvalidMessage, MaxFileNameBytes)
	case !utf8.ValidString(name):
		return fmt.Errorf("%w: a file name must be valid UTF-8", ErrInvalidMessage)
	case name == "." || name == ".." || strings.ContainsFunc(name, func(r rune) bool {
		return r == '/' || r == '\\' || badNameRune(r)
	}):
		return fmt.Errorf("%w: file name %q", ErrInvalidMessage, name)
	}
	return nil
}

func badNameRune(r rune) bool { return unicode.IsControl(r) || r == utf8.RuneError }

// FileOf returns the entry of the file id in m, or false when m holds no
// such file. A removed message holds none.
func FileOf(m Message, id string) (File, bool) {
	if m.Removed != nil {
		return File{}, false
	}
	for _, f := range m.Files {
		if f.ID == id {
			return f, true
		}
	}
	return File{}, false
}
