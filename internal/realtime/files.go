package realtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Denial types of message files.
const (
	// TypeFileRefused is a 422 for a file the property's policy refuses:
	// a MIME type outside its accept list, a positive or failed scan, or a
	// transform whose output the policy refuses.
	TypeFileRefused = "file_refused"
	// TypeFileNotFound is a 404 for a file the message does not hold, of a
	// removed message, or of a property no longer declared `file`.
	TypeFileNotFound = "file_not_found"
)

// File limits, for callers that read files before they post them.
const (
	// MaxFilesPerMessage is the most files one message holds.
	MaxFilesPerMessage = channels.MaxFilesPerMessage
	// MaxFileBytes is the most bytes of one file.
	MaxFileBytes = realtimelimits.MaxFileBytes
	// MaxMessageFileBytes is the most bytes of a message's files together.
	MaxMessageFileBytes = realtimelimits.MaxMessageFileBytes
)

// ErrFileRefused wraps a [FileChecker] refusal of a file's content. Any
// other checker error is a server error.
var ErrFileRefused = errors.New("realtime: file refused")

// FileChecker checks one posted file against the policy of its property
// and returns what to store. The wiring site supplies it over the
// attachment pipeline (MIME allowlist, scan, transforms), and records a
// refusal in the audit log.
type FileChecker interface {
	CheckFile(ctx context.Context, f FileCheck) (CheckedFile, error)
}

// FileCheck is one file to check.
type FileCheck struct {
	// Channel is the channel the file is posted to, for the audit record.
	Channel channels.Key
	// Property is the message property that holds the file; Policy is its
	// attachment policy (accept, scan, scan_cmd, transform).
	Property string
	Policy   metamodel.PropertyDef
	Name     string
	Data     []byte
}

// CheckedFile is a file after its policy: the bytes to store, and the name
// and sniffed MIME type they have after any transform.
type CheckedFile struct {
	Name string
	MIME string
	Data []byte
}

// FileInput is a file posted with a message.
type FileInput struct {
	// Property is the message type's `file` property that holds it.
	Property string
	// Name is the client's file name; only its display form is kept.
	Name string
	Data []byte
}

// fileCounts returns the number of files per property.
//
// Nil: returned when there are none.
func fileCounts[F any](files []F, prop func(F) string) map[string]int {
	if len(files) == 0 {
		return nil
	}
	out := map[string]int{}
	for _, f := range files {
		out[prop(f)]++
	}
	return out
}

func inputCounts(files []FileInput) map[string]int {
	return fileCounts(files, func(f FileInput) string { return f.Property })
}

func storedCounts(files []channels.File) map[string]int {
	return fileCounts(files, func(f channels.File) string { return f.Property })
}

// checkFileShape checks files against mt before any byte is examined:
// every file names a `file` property of mt, no property holds more than
// its `max`, a required one holds at least one, no file is over its
// property's `max_size`, and together they stay under
// [realtimelimits.MaxMessageFileBytes].
func checkFileShape(mt *realtimecfg.MessageType, files []FileInput) error {
	counts := inputCounts(files)
	for _, name := range sortedKeys(mt.Properties) {
		p := mt.Properties[name]
		if p.Type != realtimecfg.PropertyFile {
			continue
		}
		if p.Required && counts[name] == 0 {
			return invalid("property %s requires a file", name)
		}
		if counts[name] > p.File.MaxFiles {
			return invalid("property %s holds at most %d files", name, p.File.MaxFiles)
		}
	}
	total := 0
	for _, f := range files {
		p, ok := mt.Properties[f.Property]
		if !ok || p.Type != realtimecfg.PropertyFile {
			return invalid("message type %s has no file property %q", mt.Name, f.Property)
		}
		if int64(len(f.Data)) > p.File.MaxSize {
			return deny(http.StatusRequestEntityTooLarge, TypeTooLarge,
				fmt.Sprintf("a file of %s is over %d bytes", f.Property, p.File.MaxSize))
		}
		total += len(f.Data)
		if total > realtimelimits.MaxMessageFileBytes {
			return deny(http.StatusRequestEntityTooLarge, TypeTooLarge,
				fmt.Sprintf("the files of a message are over %d bytes together", realtimelimits.MaxMessageFileBytes))
		}
	}
	return nil
}

// checkFiles runs every file through checker and returns what to
// store. The first refusal stops the post: nothing is stored.
func checkFiles(
	c *call, checker FileChecker, mt *realtimecfg.MessageType, files []FileInput,
) ([]channels.NewFile, error) {
	if len(files) == 0 {
		return nil, nil
	}
	out := make([]channels.NewFile, 0, len(files))
	for _, f := range files {
		p := mt.Properties[f.Property]
		got, err := checker.CheckFile(c.ctx, FileCheck{
			Channel: c.key, Property: f.Property, Policy: p.File.Policy, Name: f.Name, Data: f.Data,
		})
		if errors.Is(err, ErrFileRefused) {
			return nil, deny(http.StatusUnprocessableEntity, TypeFileRefused,
				strings.TrimPrefix(err.Error(), ErrFileRefused.Error()+": "))
		}
		if err != nil {
			return nil, fmt.Errorf("realtime: check file: %w", err)
		}
		if int64(len(got.Data)) > p.File.MaxSize {
			return nil, deny(http.StatusRequestEntityTooLarge, TypeTooLarge,
				fmt.Sprintf("a file of %s is over %d bytes after its transform", f.Property, p.File.MaxSize))
		}
		out = append(out, channels.NewFile{Property: f.Property, Name: got.Name, MIME: got.MIME, Data: got.Data})
	}
	if err := uniqueNames(out); err != nil {
		return nil, err
	}
	return out, nil
}

// uniqueNames suffixes a repeated name within one property, as entity
// uploads do: "a.png", "a-2.png".
func uniqueNames(files []channels.NewFile) error {
	seen := map[[2]string]bool{}
	for i := range files {
		f := &files[i]
		name := f.Name
		for n := 2; seen[[2]string{f.Property, name}]; n++ {
			if n > channels.MaxFilesPerMessage+1 {
				return invalid("property %s holds two files named %q", f.Property, f.Name)
			}
			name = suffixName(f.Name, n)
		}
		f.Name = name
		seen[[2]string{f.Property, name}] = true
	}
	return nil
}

func suffixName(name string, n int) string {
	ext := ""
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		name, ext = name[:i], name[i:]
	}
	return fmt.Sprintf("%s-%d%s", name, n, ext)
}

// fileViews renders the files of m whose property mt still declares
// `file`; a download of any other is refused (see [FileOpener.OpenFile]).
//
// Nil: returned when there are none.
func fileViews(mt *realtimecfg.MessageType, files []channels.File) []channels.File {
	var out []channels.File
	for _, f := range files {
		if declaresFile(mt, f.Property) {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Property < out[j].Property })
	return out
}

func declaresFile(mt *realtimecfg.MessageType, prop string) bool {
	if mt == nil {
		return false
	}
	p, ok := mt.Properties[prop]
	return ok && p.Type == realtimecfg.PropertyFile
}

// FileDownload is a file to serve. The caller closes Body.
type FileDownload struct {
	File channels.File
	Body io.ReadCloser
}

// FileOpener serves the files of channel messages: their download, and
// the check before an upload is read. It is apart from [Service] so the
// service's API does not grow with every file operation.
type FileOpener struct {
	s *Service
}

// NewFileOpener returns the file reader over s.
//
// Nil: s rejected.
func NewFileOpener(s *Service) (*FileOpener, error) {
	if s == nil {
		return nil, errNil("Service")
	}
	return &FileOpener{s: s}, nil
}

// OpenFile opens file id of message seq for a reader of the channel. It
// answers the channel's read check first, as every read does, then a
// uniform 404 file_not_found for a file the message does not hold, a
// removed message, or a property the message type no longer declares
// `file`.
func (o *FileOpener) OpenFile(ctx context.Context, addr RawAddress, seq int64, id string) (FileDownload, error) {
	s := o.s
	c, err := s.open(ctx, addr)
	if err != nil {
		return FileDownload{}, err
	}
	if c.head == nil || !channels.ValidFileID(id) {
		return FileDownload{}, fileNotFound()
	}
	err = s.local.readLimiter.take(c.now, bucketIdentity(c.p), c.key, opFileRead, realtimelimits.FileReadRate)
	if err != nil {
		return FileDownload{}, err
	}
	//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
	m, err := s.store.Get(c.ctx, c.key, seq)
	if errors.Is(err, channels.ErrNotFound) {
		return FileDownload{}, fileNotFound()
	}
	if err != nil {
		return FileDownload{}, storeError(err)
	}
	f, ok := channels.FileOf(m, id)
	if !ok || !declaresFile(c.ct.MessageTypes[m.Type], f.Property) {
		return FileDownload{}, fileNotFound()
	}
	//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
	f, body, err := s.store.OpenFile(c.ctx, c.key, seq, id)
	if errors.Is(err, channels.ErrNotFound) {
		return FileDownload{}, fileNotFound()
	}
	if err != nil {
		return FileDownload{}, storeError(err)
	}
	return FileDownload{File: f, Body: body}, nil
}

// CheckUpload answers whether the caller may post a message of msgType
// with files to addr, before the files are read: the channel's read
// check, the type's post roles, an open channel, and a type with a `file`
// property. [Service.Post] checks all of it again, and the rest.
func (o *FileOpener) CheckUpload(ctx context.Context, addr RawAddress, msgType string) error {
	c, err := o.s.open(ctx, addr)
	if err != nil {
		return err
	}
	if err := c.requireCaller(); err != nil {
		return err
	}
	mt := c.ct.MessageTypes[msgType]
	if mt == nil {
		return invalid("channel type %s has no message type %q", c.ct.Name, msgType)
	}
	if !c.mayPost(mt) {
		return postDenied(c.ct, mt)
	}
	if err := c.requireOpen(); err != nil {
		return err
	}
	if len(fileProperties(mt)) == 0 {
		return invalid("message type %s has no file property", mt.Name)
	}
	return nil
}

func fileNotFound() *DenialError {
	return deny(http.StatusNotFound, TypeFileNotFound, "file not found")
}
