package attachment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/imgproc"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// CommandRunner drives an external command (scan or transform) over an
// attachment's bytes. It is a consumer-side seam: the data-entry/CLI wiring
// supplies a concrete runner (the cmd: harness); when nil the [PolicyProcessor]
// performs native MIME validation and any native (image:) transforms only —
// external (cmd:) scan/transform steps are then unavailable and rejected.
//
// Implementations MUST invoke commands safely: array args (never a shell
// string), templated {in}/{out} paths owned by the runner, a timeout, and an
// output-size cap. See the attachment-security guide.
type CommandRunner interface {
	// Scan runs the field's virus-scan command over data. A nil error means
	// clean; an error wrapping [ErrRejected] means a positive/blocked result or
	// (fail-closed) that the scan could not run for a `required` field.
	Scan(ctx context.Context, cmd []string, data []byte) error

	// Transform runs a transform command and returns the rewritten bytes and an
	// optional new file name.
	Transform(ctx context.Context, cmd []string, in ProcessContext, data []byte) ([]byte, string, error)
}

// applyTransforms runs the scan (when policy requires it) then the configured
// transform steps for one property, in declared order. A step is EITHER an
// external command (cmd:, runner-gated, sandboxed) OR a native in-process image
// op (image:, no runner, no sandbox). Native steps run even when the runner is
// nil; a scan or a cmd: step with a nil runner is a configuration error and
// fails closed.
func (p *PolicyProcessor) applyTransforms(
	ctx context.Context, pc ProcessContext, prop metamodel.PropertyDef, r io.Reader, info ProcessInfo,
) (io.Reader, ProcessInfo, error) {
	// Fast path: nothing to scan and nothing to transform.
	scanCmd := metamodel.NewAttachmentPolicy(p.meta).ScanCommandFor(prop)
	if len(scanCmd) == 0 && len(prop.Transform) == 0 {
		return r, info, nil
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, ProcessInfo{}, err
	}

	// Scan first — a reject here stops the write before any transform cost.
	// Scanning is always fail-closed and requires the runner.
	if len(scanCmd) > 0 {
		if p.runner == nil {
			return nil, ProcessInfo{}, Rejectedf("scan is configured but no command runner is available")
		}
		if serr := p.runner.Scan(ctx, scanCmd, data); serr != nil {
			return nil, ProcessInfo{}, serr
		}
	}

	// Transforms in declared order; each rewrites the bytes.
	for i, step := range prop.Transform {
		switch step.Kind() {
		case "image":
			data, info, err = p.applyImageStep(ctx, pc, step.Image, data, info)
		case "cmd":
			data, info, err = p.applyCmdStep(ctx, pc, step.Cmd, data, info)
		default:
			// Loader validation rejects malformed steps, so this is defensive.
			err = fmt.Errorf("attachment: transform step %d is malformed", i)
		}
		if err != nil {
			return nil, ProcessInfo{}, err
		}
	}

	return readerFor(data), info, nil
}

// applyImageStep runs a native in-process image transform: decode, orient, and
// re-encode. It needs no runner and no sandbox. On success it updates the file
// name's extension to match the re-encoded format so the stored bytes and the
// download Content-Type stay consistent (RR-7R3EYR).
func (p *PolicyProcessor) applyImageStep(
	ctx context.Context, pc ProcessContext, step *metamodel.ImageStep, data []byte, info ProcessInfo,
) ([]byte, ProcessInfo, error) {
	out, iinfo, err := imgproc.Normalize(ctx, bytes.NewReader(data), imageConfig(step))
	if err != nil {
		return nil, ProcessInfo{}, mapImageError(err)
	}
	// Base the new name on the last name set by an earlier step, else the
	// resolved upload name, then swap the extension to match the re-encoded
	// format so stored bytes and download Content-Type stay consistent.
	info.FileName = swapExt(currentName(info, pc.FileName), iinfo.Format.Ext())
	return out, info, nil
}

// applyCmdStep runs one external (cmd:) transform step via the runner.
func (p *PolicyProcessor) applyCmdStep(
	ctx context.Context, pc ProcessContext, cmd []string, data []byte, info ProcessInfo,
) ([]byte, ProcessInfo, error) {
	if p.runner == nil {
		return nil, ProcessInfo{}, Rejectedf("a command transform is configured but no command runner is available")
	}
	out, newName, err := p.runner.Transform(ctx, cmd, pc, data)
	if err != nil {
		return nil, ProcessInfo{}, err
	}
	if newName != "" {
		info.FileName = newName
	}
	return out, info, nil
}

// imageConfig builds an imgproc.Config from an ImageStep, applying defaults.
func imageConfig(step *metamodel.ImageStep) imgproc.Config {
	out := imgproc.FormatJPEG
	if step.ImageReencode() == "png" {
		out = imgproc.FormatPNG
	}
	cfg := imgproc.DefaultConfig(out)
	if step.Quality > 0 {
		cfg.JPEGQuality = step.Quality
	}
	return cfg
}

// mapImageError translates an imgproc error into a user-facing rejection (4xx)
// where appropriate; an unexpected error (e.g. an encode failure) is passed
// through as an internal fault (5xx).
func mapImageError(err error) error {
	switch {
	case errors.Is(err, imgproc.ErrTooLarge):
		return Rejectedf("image is too large to process")
	case errors.Is(err, imgproc.ErrUnsupported):
		return Rejectedf("image format is not supported for native processing")
	case errors.Is(err, imgproc.ErrAnimated):
		return Rejectedf("animated images are not supported by native image processing; use a cmd: transform")
	case errors.Is(err, imgproc.ErrTimeout):
		return Rejectedf("image processing timed out")
	default:
		return fmt.Errorf("attachment: image transform: %w", err)
	}
}

// currentName returns the file name a transform should base its output name on:
// the last name set by an earlier step (info.FileName), else fallback.
func currentName(info ProcessInfo, fallback string) string {
	if info.FileName != "" {
		return info.FileName
	}
	return fallback
}

// swapExt replaces name's extension with newExt (no dot). When name has no
// usable base it returns "image."+newExt so the stored file always has the
// correct, content-matching extension.
func swapExt(name, newExt string) string {
	if newExt == "" {
		return name
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "" {
		base = "image"
	}
	return base + "." + newExt
}
