package imgproc

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// jpegWithOrientation encodes m as JPEG and splices in an EXIF APP1 segment
// carrying the given orientation value (1..8). This produces a real fixture the
// orientation parser must read, without committing binary test data.
func jpegWithOrientation(t *testing.T, m image.Image, orientation int) []byte {
	t.Helper()
	var body bytes.Buffer
	require.NoError(t, jpeg.Encode(&body, m, &jpeg.Options{Quality: 90}))
	jb := body.Bytes()

	app1 := buildEXIFAPP1(orientation)

	// Insert the APP1 segment right after SOI (FF D8).
	out := make([]byte, 0, len(jb)+len(app1))
	out = append(out, jb[:2]...) // SOI
	out = append(out, app1...)
	out = append(out, jb[2:]...)
	return out
}

// buildEXIFAPP1 builds a minimal, big-endian EXIF APP1 segment with a single
// IFD0 entry: Orientation (SHORT, count 1).
func buildEXIFAPP1(orientation int) []byte {
	// TIFF (big-endian) payload.
	var tiff bytes.Buffer
	tiff.WriteString("MM")                                    // byte order
	_ = binary.Write(&tiff, binary.BigEndian, uint16(0x002A)) // magic
	_ = binary.Write(&tiff, binary.BigEndian, uint32(8))      // IFD0 offset

	// IFD0: 1 entry.
	_ = binary.Write(&tiff, binary.BigEndian, uint16(1)) // entry count
	_ = binary.Write(&tiff, binary.BigEndian, uint16(orientationTagID))
	_ = binary.Write(&tiff, binary.BigEndian, uint16(3)) // type SHORT
	_ = binary.Write(&tiff, binary.BigEndian, uint32(1)) // count
	_ = binary.Write(&tiff, binary.BigEndian, uint16(orientation))
	_ = binary.Write(&tiff, binary.BigEndian, uint16(0)) // pad value field to 4 bytes
	_ = binary.Write(&tiff, binary.BigEndian, uint32(0)) // next IFD offset = 0

	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)

	seg := make([]byte, 0, len(payload)+4)
	seg = append(seg, 0xFF, 0xE1) // APP1 marker
	segLen := len(payload) + 2
	seg = binary.BigEndian.AppendUint16(seg, uint16(segLen))
	seg = append(seg, payload...)
	return seg
}

func TestExifOrientation_ParsesAllValues(t *testing.T) {
	src := solidImage(6, 4, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	for o := range 8 { // o = 0..7 → orientation 1..8
		orientation := o + 1
		data := jpegWithOrientation(t, src, orientation)
		got := exifOrientation(data)
		assert.Equalf(t, orientation, got, "orientation tag %d not read back", orientation)
	}
}

func TestExifOrientation_NoEXIF(t *testing.T) {
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, solidImage(4, 4, color.White), nil))
	assert.Equal(t, 0, exifOrientation(b.Bytes()))
}

func TestExifOrientation_Garbage(t *testing.T) {
	// Must never panic, always return 0 for non-JPEG / malformed input.
	cases := [][]byte{
		nil,
		{0x00},
		{0xFF, 0xD8},                   // SOI only
		{0xFF, 0xD8, 0xFF, 0xE1, 0x00}, // truncated APP1 length
		[]byte("not a jpeg"),
	}
	for _, c := range cases {
		assert.Equal(t, 0, exifOrientation(c))
	}
}

func TestApplyOrientation_DimensionsSwapForRotations(t *testing.T) {
	// A 6-wide, 4-tall image. Orientations 5..8 rotate 90°/270°, swapping dims.
	src := solidImage(6, 4, color.RGBA{R: 100, A: 255})

	tests := []struct {
		o          int
		wantW      int
		wantH      int
		swapExpect bool
	}{
		{1, 6, 4, false},
		{2, 6, 4, false}, // flip-H
		{3, 6, 4, false}, // rotate 180
		{4, 6, 4, false}, // flip-V
		{5, 4, 6, true},  // transpose
		{6, 4, 6, true},  // rotate 90 CW
		{7, 4, 6, true},  // transverse
		{8, 4, 6, true},  // rotate 90 CCW
	}
	for _, tc := range tests {
		got := applyOrientation(src, jpegWithOrientation(t, src, tc.o), "jpeg")
		b := got.Bounds()
		assert.Equalf(t, tc.wantW, b.Dx(), "orientation %d width", tc.o)
		assert.Equalf(t, tc.wantH, b.Dy(), "orientation %d height", tc.o)
	}
}

func TestApplyOrientation_NonJPEGIsNoop(t *testing.T) {
	src := solidImage(6, 4, color.White)
	got := applyOrientation(src, nil, "png")
	assert.Same(t, image.Image(src), got)
}

// TestApplyOrientation_Rotate90CW checks pixel placement, not just dimensions:
// a marker in the top-left of the source must land top-right after a 90° CW
// rotation (orientation 6).
func TestApplyOrientation_Rotate90CW(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	// Fill background black, mark top-left pixel red.
	for y := range 2 {
		for x := range 4 {
			src.Set(x, y, color.Black)
		}
	}
	red := color.RGBA{R: 255, A: 255}
	src.Set(0, 0, red)

	got := transformOrientation(src, 6) // rotate 90 CW → 2 wide, 4 tall
	b := got.Bounds()
	require.Equal(t, 2, b.Dx())
	require.Equal(t, 4, b.Dy())

	// Top-left (0,0) of a 4×2 image, rotated 90° CW, lands at the top-right
	// column, top row: (dw-1, 0) = (1, 0).
	got1 := color.RGBAModel.Convert(got.At(1, 0)).(color.RGBA)
	assert.Equal(t, uint8(255), got1.R, "red marker should be at rotated top-right")
}

// TestNormalize_AppliesOrientation_EndToEnd is the AC-2 guard: a JPEG tagged
// Orientation=6 (rotate 90 CW) must come out with swapped dimensions, i.e. the
// pixels were physically rotated upright, WITHOUT any orientation config knob.
func TestNormalize_AppliesOrientation_EndToEnd(t *testing.T) {
	src := solidImage(10, 4, color.RGBA{G: 200, A: 255}) // 10 wide, 4 tall
	tagged := jpegWithOrientation(t, src, 6)

	_, info, err := Normalize(context.Background(), bytes.NewReader(tagged), DefaultConfig(FormatJPEG))
	require.NoError(t, err)

	// After baking orientation 6, dimensions swap to 4 wide, 10 tall.
	assert.Equal(t, 4, info.Width, "orientation must be applied by default (no knob)")
	assert.Equal(t, 10, info.Height)
}

// TestNormalize_NoGoroutineLeak asserts that decoding many inputs — valid and
// malformed — leaves no leaked goroutines once decodes complete.
func TestNormalize_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	valid := pngBytes(t, solidImage(8, 8, color.White))
	garbage := []byte("definitely not an image")

	for i := range 20 {
		in := valid
		if i%2 == 0 {
			in = garbage
		}
		_, _, _ = Normalize(context.Background(), bytes.NewReader(in), DefaultConfig(FormatPNG))
	}
}
