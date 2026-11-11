package attachment

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// FilePolicy is the attachment policy of one file slot, resolved against the
// global `attachments:` block: the MIME allowlist, the scan command and the
// transform steps. It names no entity, so a file that is not an entity
// property value, such as a file on a channel message, is checked by the same
// rules as an entity upload. Build one with [PolicyFor].
type FilePolicy struct {
	// Accept is the effective allowlist of sniffed MIME types. Empty means
	// the default-safe preset ([DefaultSafeMIMETypes]).
	Accept []string
	// ScanCmd is the scan command to run, or nil when the slot is not
	// scanned.
	ScanCmd []string
	// Transform is the ordered list of transform steps.
	Transform []metamodel.TransformStep
}

// PolicyFor resolves the file property prop against meta's global
// attachment settings, with the semantics of an entity `file` property:
// `accept` replaces the global allow list, a property `scan_cmd` replaces the
// global one, and `scan: off` opts out of scanning. The deny list of active
// content applies whatever the allow list says.
//
// Nil: meta accepted — no global settings apply.
func PolicyFor(meta *metamodel.Metamodel, prop metamodel.PropertyDef) FilePolicy {
	pol := FilePolicy{Accept: prop.Accept, Transform: prop.Transform}
	if meta == nil {
		if prop.Scan != metamodel.ScanOff {
			pol.ScanCmd = prop.ScanCmd
		}
		return pol
	}
	if len(pol.Accept) == 0 && meta.Attachments != nil {
		pol.Accept = meta.Attachments.Allow
	}
	pol.ScanCmd = metamodel.NewAttachmentPolicy(meta).ScanCommandFor(prop)
	return pol
}

// Checker applies a [FilePolicy] to a file's bytes. It is safe for concurrent
// use and is built once at wiring, around the shared command runner.
type Checker struct {
	runner CommandRunner
}

// NewChecker returns a checker that runs scan and `cmd:` steps through
// runner.
//
// Nil: runner accepted — native MIME checks and `image:` steps still run, and
// a policy that needs a scan or a `cmd:` step refuses every file (fail
// closed), as an entity upload without a runner does.
func NewChecker(runner CommandRunner) *Checker {
	return &Checker{runner: runner}
}

// Checked is the outcome of [Checker.Check]: the bytes to store, the name to
// store them under and their sniffed MIME type.
type Checked struct {
	Data     []byte
	FileName string
	MIME     string
}

// Check runs the MIME allowlist, the scan and the transforms of pol over
// data, in that order. Unlike [PolicyProcessor.Process] it never passes bytes
// through unchecked. When a transform ran, the output is checked again
// against the allowlist and the deny lists under its final name, because a
// `cmd:` step can produce any type. Empty data is refused. A refusal wraps
// [ErrRejected].
func (p *Checker) Check(ctx context.Context, pol FilePolicy, pc ProcessContext, data []byte) (Checked, error) {
	if len(data) == 0 {
		return Checked{}, Rejectedf("file is empty")
	}
	if _, _, err := newMIMEProcessor(pol.Accept).Process(ctx, pc, bytes.NewReader(data)); err != nil {
		return Checked{}, err
	}
	out, info, err := p.applyTransforms(ctx, pc, pol, bytes.NewReader(data), ProcessInfo{})
	if err != nil {
		return Checked{}, err
	}
	final, err := io.ReadAll(out)
	if err != nil {
		return Checked{}, fmt.Errorf("attachment: read processed bytes: %w", err)
	}
	name := pc.FileName
	if info.FileName != "" {
		name = info.FileName
	}
	if len(pol.Transform) > 0 {
		if len(final) == 0 {
			return Checked{}, Rejectedf("the transform produced an empty file")
		}
		after := pc
		after.FileName = name
		if _, _, err := newMIMEProcessor(pol.Accept).Process(ctx, after, bytes.NewReader(final)); err != nil {
			return Checked{}, fmt.Errorf("after transform: %w", err)
		}
	}
	return Checked{Data: final, FileName: name, MIME: SniffMIME(final)}, nil
}

// SniffMIME returns the base MIME type the bytes sniff as, the type every
// allowlist check in this package uses.
func SniffMIME(data []byte) string {
	return baseMIME(http.DetectContentType(data))
}
