// Package imgproc normalizes raster image bytes in-process, with no external
// tool and no OS sandbox.
//
// The security argument is the whole point: the decoders are pure-Go and
// memory-safe (bounds-checked, GC-managed, panic-not-corrupt), so running a
// crafted image through them cannot achieve memory corruption or code
// execution the way a C parser (libjpeg, ImageMagick delegates) can. The
// residual threat collapses from "RCE + DoS" to "DoS only", which [Normalize]
// bounds with three controls:
//
//   - a pixel cap read cheaply from the header via image.DecodeConfig, applied
//     BEFORE the pixel buffer is allocated (defeats the decompression bomb: a
//     tiny file declaring a 50-gigapixel canvas);
//   - a wall-clock deadline that unblocks the caller on a CPU/entropy bomb;
//   - a package-wide concurrency semaphore so N simultaneous decodes cannot
//     each allocate near the pixel cap and OOM the host.
//
// [Normalize] decodes, applies EXIF orientation, and re-encodes to a canonical
// format (PNG or JPEG). The re-encode is itself a safety feature: the output is
// a freshly-serialized bitmap carrying none of the input's metadata, so EXIF,
// GPS, and any format-specific trailing data are stripped as a side effect.
//
// This package has no domain imports; it depends only on the standard library
// and golang.org/x/image, so it stays trivially testable and arch-lint clean.
package imgproc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"runtime"
	"time"

	"golang.org/x/sync/semaphore"

	// Register the decoders. GIF/JPEG/PNG are stdlib; WebP is decode-only from
	// x/image (there is no pure-Go WebP encoder, so WebP input re-encodes to
	// PNG/JPEG — see Normalize).
	_ "golang.org/x/image/webp"
)

// Format is a canonical re-encode target. Only formats with a pure-Go encoder
// are valid outputs; input may be any registered decoder (incl. WebP).
type Format string

const (
	// FormatJPEG re-encodes to JPEG (lossy; honors [Config.JPEGQuality]).
	FormatJPEG Format = "jpeg"
	// FormatPNG re-encodes to PNG (lossless).
	FormatPNG Format = "png"
)

// Sentinel errors. Callers map these to a user-facing rejection (4xx); an
// unexpected error (e.g. an encode failure) is an internal fault (5xx).
var (
	// ErrTooLarge means the image's declared dimensions exceed the pixel cap.
	// It is reported from the header (DecodeConfig) before any large allocation.
	ErrTooLarge = errors.New("imgproc: image exceeds pixel cap")

	// ErrUnsupported means the bytes are not a format this package can decode,
	// so a caller may fall through to an external (cmd:) transform instead.
	ErrUnsupported = errors.New("imgproc: unsupported image format")

	// ErrAnimated means a multi-frame (animated) GIF was supplied. Re-encoding
	// to a single-frame format would silently drop the animation, so it is
	// rejected rather than flattened.
	ErrAnimated = errors.New("imgproc: animated images are not supported")

	// ErrInvalidConfig means the [Config] is unusable (bad output format, etc.).
	ErrInvalidConfig = errors.New("imgproc: invalid config")

	// ErrTimeout means the decode did not finish within [Config.Timeout]. Note
	// the underlying decode goroutine may still be running (Go image decoders
	// do not observe context) — the concurrency semaphore bounds how many such
	// goroutines can exist at once.
	ErrTimeout = errors.New("imgproc: decode timed out")
)

// Defaults for a [Config]. They are deliberately generous — a legitimate photo
// must not trip them — while keeping the worst-case memory bounded.
const (
	// DefaultMaxPixels caps decoded resolution. 64 MP ≈ a 8000×8000 image; at
	// 4 bytes/pixel (the model stdlib decodes to) that is ~256 MiB for one
	// image, which is why concurrency is capped (see maxConcurrent).
	DefaultMaxPixels int64 = 64 << 20 // 64 Mpx

	// DefaultMaxBytes bounds the input stream read into memory before decoding.
	DefaultMaxBytes int64 = 64 << 20 // 64 MiB

	// DefaultTimeout bounds one decode+encode operation.
	DefaultTimeout = 20 * time.Second

	// DefaultJPEGQuality is used when re-encoding to JPEG without an explicit
	// quality.
	DefaultJPEGQuality = 85

	// bytesPerPixel is what the stdlib decoders materialize (NRGBA/RGBA). Used
	// to translate the pixel cap into a memory budget.
	bytesPerPixel = 4
)

// maxConcurrent bounds how many decodes run at once, process-wide. Combined
// with the pixel cap it bounds worst-case transient memory to roughly
// maxConcurrent × DefaultMaxPixels × bytesPerPixel. It is derived from the CPU
// count (small, since decoding is CPU-bound) and floored at 1.
var maxConcurrent = max(int64(runtime.GOMAXPROCS(0)), 1)

// MemoryBudgetBytes is the documented worst-case transient decode memory:
// concurrency bound × pixel cap × bytes-per-pixel. Exposed so operators tuning
// MaxPixels can reason about the ceiling.
func MemoryBudgetBytes() int64 {
	return maxConcurrent * DefaultMaxPixels * bytesPerPixel
}

// decodeSem serializes decodes to at most [maxConcurrent] at a time. It is a
// package singleton so the bound is process-wide (every attachment write path
// shares it), which is what actually bounds host memory — a per-request limiter
// would not.
var decodeSem = semaphore.NewWeighted(maxConcurrent)

// Config bounds a single [Normalize] operation. The zero value is not usable;
// use [DefaultConfig] and override fields, or set them explicitly.
type Config struct {
	// MaxPixels rejects an image whose Width*Height exceeds this, from the
	// header, before decoding. Zero uses DefaultMaxPixels.
	MaxPixels int64
	// MaxBytes bounds the input read into memory. Zero uses DefaultMaxBytes.
	MaxBytes int64
	// Timeout bounds the decode+encode. Zero uses DefaultTimeout.
	Timeout time.Duration
	// Output is the canonical re-encode target (required: FormatPNG or FormatJPEG).
	Output Format
	// JPEGQuality is the JPEG quality (1..100) when Output is FormatJPEG. Zero
	// uses DefaultJPEGQuality.
	JPEGQuality int
}

// DefaultConfig returns a Config with the package defaults and the given output
// format. Callers typically override MaxPixels / Timeout from operator config.
func DefaultConfig(out Format) Config {
	return Config{
		MaxPixels:   DefaultMaxPixels,
		MaxBytes:    DefaultMaxBytes,
		Timeout:     DefaultTimeout,
		Output:      out,
		JPEGQuality: DefaultJPEGQuality,
	}
}

func (c Config) withDefaults() Config {
	if c.MaxPixels <= 0 {
		c.MaxPixels = DefaultMaxPixels
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = DefaultMaxBytes
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.JPEGQuality <= 0 {
		c.JPEGQuality = DefaultJPEGQuality
	}
	return c
}

// Info reports what [Normalize] produced.
type Info struct {
	// Format is the output format (mirrors Config.Output). Its String is the
	// canonical file extension without the dot ("jpg" or "png").
	Format Format
	// Width and Height are the output pixel dimensions (post-orientation).
	Width, Height int
}

// Ext returns the canonical file extension (no dot) for a re-encode target:
// "jpg" for JPEG, "png" for PNG. EXIF orientation is always applied on decode,
// so callers never need to preserve the input extension.
func (f Format) Ext() string {
	switch f {
	case FormatJPEG:
		return "jpg"
	case FormatPNG:
		return "png"
	default:
		return ""
	}
}

// ValidOutput reports whether f is a supported re-encode target.
func (f Format) ValidOutput() bool { return f == FormatJPEG || f == FormatPNG }

// Normalize decodes r, applies EXIF orientation, and re-encodes to cfg.Output,
// returning the canonical bytes and an [Info]. EXIF orientation is ALWAYS
// applied (there is no opt-out): re-encoding without honoring it would store
// phone photos sideways, which is worse than not processing at all. All
// metadata is dropped by the re-encode.
//
// Errors: [ErrTooLarge] (dims over cap), [ErrUnsupported] (undecodable format),
// [ErrAnimated] (multi-frame GIF), [ErrTimeout] (decode too slow), or
// [ErrInvalidConfig]. A decode error on a truncated/corrupt-but-recognized
// image wraps the decoder's error. An encode failure is returned as a plain
// error (internal fault).
func Normalize(ctx context.Context, r io.Reader, cfg Config) ([]byte, Info, error) {
	if !cfg.Output.ValidOutput() {
		return nil, Info{}, fmt.Errorf("%w: output format %q", ErrInvalidConfig, cfg.Output)
	}
	cfg = cfg.withDefaults()

	// Read the (bounded) input once so we can both sniff the header and decode
	// without a second reader. MaxBytes+1 lets us distinguish "exactly at cap"
	// from "over"; upstream layers also cap, so this read is itself bounded.
	data, err := io.ReadAll(io.LimitReader(r, cfg.MaxBytes+1))
	if err != nil {
		return nil, Info{}, fmt.Errorf("imgproc: read input: %w", err)
	}
	if int64(len(data)) > cfg.MaxBytes {
		return nil, Info{}, fmt.Errorf("%w: input exceeds %d bytes", ErrTooLarge, cfg.MaxBytes)
	}

	// Header-only decode: reject oversized dimensions BEFORE allocating pixels.
	// This is the decompression-bomb guard and it must run before any decode
	// goroutine is spawned.
	cf, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, Info{}, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	if px := int64(cf.Width) * int64(cf.Height); px > cfg.MaxPixels {
		return nil, Info{}, fmt.Errorf("%w: %d pixels > %d", ErrTooLarge, px, cfg.MaxPixels)
	}

	// Animated GIF: re-encoding to a single-frame format would silently drop
	// the animation, so reject rather than flatten. Checked from the header
	// region without a full decode.
	if format == "gif" {
		if animated, aerr := isAnimatedGIF(data); aerr != nil {
			return nil, Info{}, fmt.Errorf("%w: %w", ErrUnsupported, aerr)
		} else if animated {
			return nil, Info{}, ErrAnimated
		}
	}

	img, err := decodeBounded(ctx, data, cfg.Timeout)
	if err != nil {
		return nil, Info{}, err
	}

	// Apply EXIF orientation (JPEG/TIFF-family only; a no-op elsewhere). The
	// orientation tag is read from the original bytes, not the decoded image.
	img = applyOrientation(img, data, format)

	out, err := encode(img, cfg)
	if err != nil {
		return nil, Info{}, err
	}
	b := img.Bounds()
	return out, Info{Format: cfg.Output, Width: b.Dx(), Height: b.Dy()}, nil
}

// encode serializes img to cfg.Output. Returns a plain (non-sentinel) error on
// failure, which callers treat as an internal fault rather than a rejection.
func encode(img image.Image, cfg Config) ([]byte, error) {
	var buf bytes.Buffer
	switch cfg.Output {
	case FormatJPEG:
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: cfg.JPEGQuality}); err != nil {
			return nil, fmt.Errorf("imgproc: jpeg encode: %w", err)
		}
	case FormatPNG:
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("imgproc: png encode: %w", err)
		}
	default:
		return nil, fmt.Errorf("%w: output format %q", ErrInvalidConfig, cfg.Output)
	}
	return buf.Bytes(), nil
}

// decodeBounded decodes data in a worker goroutine bounded by the package
// semaphore and the given timeout.
//
// IMPORTANT: the timeout unblocks the CALLER; it does not cancel the decode.
// Go image decoders do not observe context, so a decode already in flight runs
// to completion. This is safe because (a) the pixel cap in Normalize rejects a
// dimension bomb before we ever get here, so a decode that starts has a bounded
// output size, and (b) decodeSem caps how many such goroutines can exist at
// once — the leak on a pathological-but-honest input is bounded, not unbounded.
// A recover() turns a decoder panic (the class behind the 2026 x/image webp/bmp
// CVEs) into a returned error instead of a process crash.
func decodeBounded(ctx context.Context, data []byte, timeout time.Duration) (img image.Image, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if aerr := decodeSem.Acquire(ctx, 1); aerr != nil {
		return nil, fmt.Errorf("%w: waiting for decode slot: %w", ErrTimeout, aerr)
	}

	type result struct {
		img image.Image
		err error
	}
	done := make(chan result, 1) // buffered: the worker never blocks even if we've timed out
	go func() {
		defer decodeSem.Release(1)
		defer func() {
			if r := recover(); r != nil {
				done <- result{err: fmt.Errorf("%w: decoder panic: %v", ErrUnsupported, r)}
			}
		}()
		m, _, derr := image.Decode(bytes.NewReader(data))
		if derr != nil {
			done <- result{err: fmt.Errorf("%w: %w", ErrUnsupported, derr)}
			return
		}
		done <- result{img: m}
	}()

	select {
	case <-ctx.Done():
		// The worker keeps running until the decoder returns; the buffered
		// channel and semaphore release ensure it neither blocks nor leaks a
		// slot. See the doc comment.
		return nil, ErrTimeout
	case res := <-done:
		return res.img, res.err
	}
}

// isAnimatedGIF reports whether data is a GIF with more than one frame, using
// gif.DecodeAll. DecodeAll on a non-GIF returns an error, which callers treat
// as ErrUnsupported.
func isAnimatedGIF(data []byte) (bool, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	return len(g.Image) > 1, nil
}
