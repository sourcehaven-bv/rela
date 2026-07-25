package imgproc

import (
	"bytes"
	"encoding/binary"
	"image"
)

// EXIF orientation is a single tag (0x0112) with values 1..8 describing how the
// stored pixels must be rotated/flipped to be displayed upright. Cameras and
// phones set it instead of physically rotating the pixels; a decoder that
// ignores it stores the image sideways. Since [Normalize] re-encodes and drops
// all metadata, we MUST bake the orientation into the pixels here — otherwise a
// portrait phone photo would be stored rotated with no tag left to correct it.
//
// We read only this one tag, from the EXIF APP1 segment of a JPEG (the only
// place orientation appears among the formats this package accepts; PNG/GIF/
// WebP-in-x/image carry no EXIF orientation). The parser is intentionally
// minimal: it walks the TIFF header → IFD0 → the orientation entry and stops.
// Anything malformed yields orientation 1 (no transform), never a panic.

const orientationTagID = 0x0112

// EXIF orientation values (tag 0x0112). See transformOrientation for the
// geometry of each.
const (
	orientNormal     = 1 // upright, no transform
	orientFlipH      = 2 // mirror horizontal
	orientRotate180  = 3
	orientFlipV      = 4 // mirror vertical
	orientTranspose  = 5 // flip along the main diagonal
	orientRotate90CW = 6
	orientTransverse = 7 // flip along the anti-diagonal
	orientRotate90CC = 8 // rotate 90° counter-clockwise
	// orientMax is the highest defined value; orientRotateSwapMin is the first
	// value whose transform swaps width and height.
	orientMax           = orientRotate90CC
	orientRotateSwapMin = orientTranspose

	jpegMarkerPrefix = 0xFF // every JPEG marker starts with this byte
	markerSOIlo      = 0xD8 // SOI second byte (FF D8 starts a JPEG)
	markerEOI        = 0xD9 // end of image (standalone)
	markerSOS        = 0xDA // start of scan — entropy data follows, stop scanning
	markerAPP1       = 0xE1 // APP1 segment (holds EXIF)
	tiffMagic        = 0x002A
	tiffHeaderLen    = 8 // "II"/"MM" + magic(2) + IFD0 offset(4)

	exifHeaderLen = 6 // "Exif\0\0"
)

// applyOrientation returns img transformed per the EXIF orientation tag found
// in data. format is the decoded format name (from image.DecodeConfig); only
// "jpeg" carries EXIF here. Any parse failure is treated as orientation 1.
func applyOrientation(img image.Image, data []byte, format string) image.Image {
	if format != "jpeg" {
		return img
	}
	o := exifOrientation(data)
	if o <= orientNormal || o > orientMax {
		return img // 1 (or absent/invalid) = already upright
	}
	return transformOrientation(img, o)
}

// exifOrientation extracts the orientation value (1..8) from a JPEG's EXIF
// APP1 segment, or 0 if not present/parseable.
func exifOrientation(jpegData []byte) int {
	seg := exifAPP1(jpegData)
	if seg == nil {
		return 0
	}
	return orientationFromTIFF(seg)
}

// exifAPP1 returns the TIFF payload (after the "Exif\0\0" header) of the first
// EXIF APP1 marker in a JPEG, or nil. It scans the marker structure only; it
// never decodes image data.
func exifAPP1(b []byte) []byte {
	// JPEG starts with SOI (FF D8). Markers follow as FF <marker> <len hi> <len lo>.
	if len(b) < 2 || b[0] != jpegMarkerPrefix || b[1] != markerSOIlo {
		return nil
	}
	i := 2
	for i+4 <= len(b) {
		if b[i] != jpegMarkerPrefix {
			return nil // not at a marker; give up rather than guess
		}
		marker := b[i+1]
		// EOI is standalone (no length); SOS begins entropy-coded data we won't
		// scan past. Either way, stop before the EXIF-bearing APP1 segments end.
		if marker == markerEOI || marker == markerSOS {
			return nil
		}
		segLen := int(binary.BigEndian.Uint16(b[i+2 : i+4]))
		if segLen < 2 || i+2+segLen > len(b) {
			return nil
		}
		payload := b[i+4 : i+2+segLen]
		if marker == markerAPP1 && isEXIFPayload(payload) {
			return payload[exifHeaderLen:]
		}
		i += 2 + segLen
	}
	return nil
}

// isEXIFPayload reports whether an APP1 payload begins with the "Exif\0\0"
// header that precedes the TIFF-structured EXIF data.
func isEXIFPayload(payload []byte) bool {
	return len(payload) >= exifHeaderLen && bytes.Equal(payload[:exifHeaderLen], []byte("Exif\x00\x00"))
}

// orientationFromTIFF parses a TIFF byte stream (EXIF payload) far enough to
// read the orientation tag from IFD0. Returns 0 on any malformation.
func orientationFromTIFF(t []byte) int {
	if len(t) < tiffHeaderLen {
		return 0
	}
	var bo binary.ByteOrder
	switch {
	case t[0] == 'I' && t[1] == 'I':
		bo = binary.LittleEndian
	case t[0] == 'M' && t[1] == 'M':
		bo = binary.BigEndian
	default:
		return 0
	}
	if bo.Uint16(t[2:4]) != tiffMagic {
		return 0
	}
	ifdOff := int(bo.Uint32(t[4:tiffHeaderLen]))
	if ifdOff < tiffHeaderLen || ifdOff+2 > len(t) {
		return 0
	}
	count := int(bo.Uint16(t[ifdOff : ifdOff+2]))
	entry := ifdOff + 2
	const entrySize = 12 // TIFF IFD entry: tag(2)+type(2)+count(4)+value(4)
	// valueOffset is where a SHORT value sits within a 12-byte entry.
	const valueOffset = 8
	for n := range count {
		off := entry + n*entrySize
		if off+entrySize > len(t) {
			return 0
		}
		tag := bo.Uint16(t[off : off+2])
		if tag != orientationTagID {
			continue
		}
		// type SHORT (3), count 1 → value in the first 2 bytes of the value field.
		val := int(bo.Uint16(t[off+valueOffset : off+valueOffset+2]))
		if val >= orientNormal && val <= orientMax {
			return val
		}
		return 0
	}
	return 0
}

// transformOrientation returns a new image with the pixels rotated/flipped so
// the result is upright for the given EXIF orientation (2..8). It renders into
// a fresh RGBA image; orientation 1 never reaches here.
//
// The 8 EXIF orientations:
//
//	1: normal          2: flip-H
//	3: rotate 180      4: flip-V
//	5: transpose       6: rotate 90 CW
//	7: transverse      8: rotate 90 CCW
func transformOrientation(src image.Image, o int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	// Orientations 5-8 swap width and height (90°/270° rotations).
	swap := o >= orientRotateSwapMin
	dw, dh := w, h
	if swap {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))

	for y := range h {
		for x := range w {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			var nx, ny int
			switch o {
			case orientFlipH:
				nx, ny = w-1-x, y
			case orientRotate180:
				nx, ny = w-1-x, h-1-y
			case orientFlipV:
				nx, ny = x, h-1-y
			case orientTranspose:
				nx, ny = y, x
			case orientRotate90CW:
				nx, ny = h-1-y, x
			case orientTransverse:
				nx, ny = h-1-y, w-1-x
			case orientRotate90CC:
				nx, ny = y, w-1-x
			default:
				nx, ny = x, y
			}
			dst.Set(nx, ny, c)
		}
	}
	return dst
}
