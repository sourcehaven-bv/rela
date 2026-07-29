package imgproc

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// solidImage returns a w×h image filled with c.
func solidImage(w, h int, c color.Color) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.Set(x, y, c)
		}
	}
	return m
}

func pngBytes(t *testing.T, m image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, m))
	return b.Bytes()
}

func jpegBytes(t *testing.T, m image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, m, &jpeg.Options{Quality: 90}))
	return b.Bytes()
}

func TestNormalize_ReencodesCommonFormats(t *testing.T) {
	src := solidImage(20, 10, color.RGBA{R: 200, G: 100, B: 50, A: 255})

	tests := []struct {
		name  string
		input []byte
		out   Format
		wantW int
		wantH int
	}{
		{"png->jpeg", pngBytes(t, src), FormatJPEG, 20, 10},
		{"png->png", pngBytes(t, src), FormatPNG, 20, 10},
		{"jpeg->png", jpegBytes(t, src), FormatPNG, 20, 10},
		{"jpeg->jpeg", jpegBytes(t, src), FormatJPEG, 20, 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, info, err := Normalize(context.Background(), bytes.NewReader(tc.input), DefaultConfig(tc.out))
			require.NoError(t, err)
			assert.Equal(t, tc.out, info.Format)
			assert.Equal(t, tc.wantW, info.Width)
			assert.Equal(t, tc.wantH, info.Height)

			// Output must actually decode as the target format.
			_, gotFormat, derr := image.DecodeConfig(bytes.NewReader(out))
			require.NoError(t, derr)
			if tc.out == FormatJPEG {
				assert.Equal(t, "jpeg", gotFormat)
			} else {
				assert.Equal(t, "png", gotFormat)
			}
		})
	}
}

func TestNormalize_InvalidOutputFormat(t *testing.T) {
	src := pngBytes(t, solidImage(4, 4, color.White))
	_, _, err := Normalize(context.Background(), bytes.NewReader(src), Config{Output: "webp"})
	require.ErrorIs(t, err, ErrInvalidConfig)
}

func TestNormalize_PixelCap(t *testing.T) {
	// A legitimately-encoded but large image; cap set below its pixel count.
	src := pngBytes(t, solidImage(100, 100, color.White))
	cfg := DefaultConfig(FormatPNG)
	cfg.MaxPixels = 100*100 - 1 // one under

	_, _, err := Normalize(context.Background(), bytes.NewReader(src), cfg)
	require.ErrorIs(t, err, ErrTooLarge)
}

func TestNormalize_ByteCap(t *testing.T) {
	src := pngBytes(t, solidImage(50, 50, color.White))
	cfg := DefaultConfig(FormatPNG)
	cfg.MaxBytes = int64(len(src) - 1)

	_, _, err := Normalize(context.Background(), bytes.NewReader(src), cfg)
	require.ErrorIs(t, err, ErrTooLarge)
}

func TestNormalize_UnsupportedFormat(t *testing.T) {
	_, _, err := Normalize(context.Background(), bytes.NewReader([]byte("not an image at all")), DefaultConfig(FormatPNG))
	require.ErrorIs(t, err, ErrUnsupported)
}

func TestNormalize_TruncatedInput(t *testing.T) {
	full := pngBytes(t, solidImage(30, 30, color.White))
	truncated := full[:len(full)/2]
	_, _, err := Normalize(context.Background(), bytes.NewReader(truncated), DefaultConfig(FormatPNG))
	require.Error(t, err) // either Unsupported (bad header) or a wrapped decode error; must not panic
}

func TestNormalize_SingleFrameGIF(t *testing.T) {
	var b bytes.Buffer
	require.NoError(t, gif.Encode(&b, solidImage(8, 8, color.RGBA{G: 255, A: 255}), nil))

	out, info, err := Normalize(context.Background(), bytes.NewReader(b.Bytes()), DefaultConfig(FormatPNG))
	require.NoError(t, err)
	assert.Equal(t, FormatPNG, info.Format)
	assert.NotEmpty(t, out)
}

func TestNormalize_AnimatedGIFRejected(t *testing.T) {
	// Build a 2-frame GIF.
	g := &gif.GIF{}
	for range 2 {
		pal := image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.Black, color.White})
		g.Image = append(g.Image, pal)
		g.Delay = append(g.Delay, 10)
	}
	var b bytes.Buffer
	require.NoError(t, gif.EncodeAll(&b, g))

	_, _, err := Normalize(context.Background(), bytes.NewReader(b.Bytes()), DefaultConfig(FormatPNG))
	require.ErrorIs(t, err, ErrAnimated)
}

func TestNormalize_StripsMetadata(t *testing.T) {
	// A JPEG carrying an EXIF APP1 segment; after re-encode the output must not
	// contain the "Exif" marker.
	withExif := jpegWithOrientation(t, solidImage(12, 8, color.RGBA{B: 255, A: 255}), 1)
	require.True(t, bytes.Contains(withExif, []byte("Exif")), "fixture should contain EXIF")

	out, _, err := Normalize(context.Background(), bytes.NewReader(withExif), DefaultConfig(FormatJPEG))
	require.NoError(t, err)
	assert.False(t, bytes.Contains(out, []byte("Exif")), "re-encoded output must have EXIF stripped")
}

func TestFormatExt(t *testing.T) {
	assert.Equal(t, "jpg", FormatJPEG.Ext())
	assert.Equal(t, "png", FormatPNG.Ext())
	assert.Empty(t, Format("webp").Ext())
}

func TestNormalize_ContextAlreadyCancelled(t *testing.T) {
	src := pngBytes(t, solidImage(10, 10, color.White))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := Normalize(ctx, bytes.NewReader(src), DefaultConfig(FormatPNG))
	// Acquire fails on a cancelled ctx → ErrTimeout wrapper.
	require.ErrorIs(t, err, ErrTimeout)
}

// FuzzNormalize throws arbitrary bytes at the decoder; the invariant is "never
// panics, always returns", which -race + the fuzz harness enforce.
func FuzzNormalize(f *testing.F) {
	f.Add(pngBytes(&testing.T{}, solidImage(4, 4, color.White)))
	f.Add([]byte("GIF89a"))
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE1}) // JPEG SOI + truncated APP1
	f.Add([]byte("\x89PNG\r\n\x1a\n"))    // PNG magic only
	f.Fuzz(func(_ *testing.T, data []byte) {
		cfg := DefaultConfig(FormatPNG)
		cfg.Timeout = 2 * time.Second
		// Must return (no panic) regardless of input; result is ignored.
		_, _, _ = Normalize(context.Background(), bytes.NewReader(data), cfg)
	})
}

func TestSentinelsAreDistinct(t *testing.T) {
	all := []error{ErrTooLarge, ErrUnsupported, ErrAnimated, ErrInvalidConfig, ErrTimeout}
	for i := range all {
		for j := range all {
			if i != j {
				assert.NotErrorIs(t, all[i], all[j])
			}
		}
	}
}
