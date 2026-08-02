package imgproc

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeGIF(t *testing.T, frames, w, h int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for range frames {
		p := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.Black, color.White})
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 0)
	}
	var b bytes.Buffer
	require.NoError(t, gif.EncodeAll(&b, g))
	return b.Bytes()
}

func TestGIFFrameCount(t *testing.T) {
	tests := []struct {
		name   string
		frames int
	}{
		{"single frame", 1},
		{"two frames", 2},
		{"many frames", 50},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := encodeGIF(t, tc.frames, 16, 12)
			gi, ok := gifFrameCount(data)
			require.True(t, ok, "should parse GIF structure")
			assert.Equal(t, tc.frames, gi.frames)
			assert.Equal(t, 16, gi.width)
			assert.Equal(t, 12, gi.height)
		})
	}
}

func TestGIFFrameCount_NotAGIF(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte("not a gif"),
		[]byte("GIF89a"), // header only, truncated
		{0x89, 'P', 'N', 'G'},
	}
	for _, c := range cases {
		gi, ok := gifFrameCount(c)
		// Either not-ok, or ok with zero frames — never a panic, never a bogus count.
		if ok {
			assert.Equal(t, 0, gi.frames)
		}
	}
}

// TestGIFBomb_RejectedWithoutDecode is the RR-YLYWUZ regression: a many-frame
// GIF that is tiny compressed and passes the logical-screen pixel cap must be
// rejected as animated CHEAPLY — without gif.DecodeAll materializing every
// frame. We assert both the verdict and that it is near-instant.
func TestGIFBomb_RejectedWithoutDecode(t *testing.T) {
	// 300 frames of 1000x1000: ~0.5 MiB compressed, 1 Mpx logical screen (under
	// the 64 Mpx cap), but ~1.2 GiB if every frame were decoded.
	data := encodeGIF(t, 300, 1000, 1000)
	require.Less(t, len(data), 2<<20, "fixture should be small compressed")

	start := time.Now()
	_, _, err := Normalize(context.Background(), bytes.NewReader(data), DefaultConfig(FormatPNG))
	elapsed := time.Since(start)

	require.ErrorIs(t, err, ErrAnimated)
	assert.Less(t, elapsed, 100*time.Millisecond,
		"animated GIF must be rejected from the header, not by decoding every frame (took %v)", elapsed)
}

// TestGIF_SingleFrameStillReencodes confirms the cheap frame count did not break
// the legitimate single-frame path.
func TestGIF_SingleFrameStillReencodes(t *testing.T) {
	data := encodeGIF(t, 1, 20, 10)
	out, info, err := Normalize(context.Background(), bytes.NewReader(data), DefaultConfig(FormatPNG))
	require.NoError(t, err)
	assert.Equal(t, FormatPNG, info.Format)
	assert.NotEmpty(t, out)
}

// TestGIFFrameCount_NoPanicOnMalformed throws truncated/garbage GIF-ish bytes at
// the header walker; it must never panic.
func TestGIFFrameCount_NoPanicOnMalformed(t *testing.T) {
	valid := encodeGIF(t, 3, 8, 8)
	for i := range valid {
		_, _ = gifFrameCount(valid[:i]) // every truncation
	}
	// Also a GIF header with a lying global-color-table flag but no table bytes.
	_, _ = gifFrameCount([]byte("GIF89a\x08\x00\x08\x00\xF7\x00\x00"))
}
