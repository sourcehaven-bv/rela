package imgproc

import "encoding/binary"

// This file counts GIF frames by walking the block structure of the encoded
// stream WITHOUT decoding any pixels. gif.DecodeAll would materialize every
// frame into memory — for a multi-frame GIF that is a decompression bomb (a
// tiny LZW-compressible file can declare hundreds of large frames), and it runs
// before the pixel cap / semaphore / timeout can bound it. Counting frames from
// the header structure is allocation-free and lets Normalize reject an animated
// (or oversized-across-frames) GIF up front.
//
// GIF89a grammar (only the parts we need):
//
//	Header               6 bytes  "GIF87a" | "GIF89a"
//	Logical Screen Desc  7 bytes  width(2) height(2) packed(1) bg(1) aspect(1)
//	[Global Color Table] present iff packed bit 7; size 3 * 2^(1+packed&7)
//	Blocks, each introduced by a one-byte label:
//	  0x2C  Image Descriptor  -> one FRAME
//	  0x21  Extension         -> label(1) then sub-blocks
//	  0x3B  Trailer           -> end
//	Image data and extensions carry length-prefixed sub-blocks terminated by a
//	zero-length block.

const (
	gifImageSeparator byte = 0x2C
	gifExtension      byte = 0x21
	gifTrailer        byte = 0x3B

	gifHeaderLen     = 6
	gifScreenDescLen = 7
	gifGCTFlagBit    = 0x80 // packed field bit 7: global color table present
	gifCTSizeMask    = 0x07 // packed field low 3 bits: color-table size exponent
)

// gifInfo is what gifFrameCount extracts from a GIF header walk.
type gifInfo struct {
	width, height int
	frames        int
}

// gifFrameCount walks the GIF block structure and returns the logical-screen
// dimensions and the number of image frames, without decoding pixel data.
// ok is false when data is not a structurally-walkable GIF (caller then treats
// it as undecodable / falls back to the normal decoder for the error).
func gifFrameCount(data []byte) (info gifInfo, ok bool) {
	if len(data) < gifHeaderLen+gifScreenDescLen {
		return gifInfo{}, false
	}
	if string(data[:3]) != "GIF" {
		return gifInfo{}, false
	}

	p := gifHeaderLen
	info.width = int(binary.LittleEndian.Uint16(data[p : p+2]))
	info.height = int(binary.LittleEndian.Uint16(data[p+2 : p+4]))
	packed := data[p+4]
	p += gifScreenDescLen

	// Skip the global color table if present.
	if packed&gifGCTFlagBit != 0 {
		p += gifColorTableBytes(packed)
	}

	for {
		if p >= len(data) {
			// Truncated stream: return what we counted. The real decoder will
			// surface the truncation error later if this GIF is processed.
			return info, true
		}
		switch data[p] {
		case gifTrailer:
			return info, true
		case gifExtension:
			p++
			if p >= len(data) {
				return info, true
			}
			p++ // extension label
			np, okSkip := skipSubBlocks(data, p)
			if !okSkip {
				return info, true
			}
			p = np
		case gifImageSeparator:
			info.frames++
			np, okImg := skipImageDescriptor(data, p)
			if !okImg {
				return info, true
			}
			p = np
		default:
			// Unknown block label: give up walking but keep the count so far.
			return info, true
		}
	}
}

// gifColorTableBytes returns the byte size of a color table for a packed field:
// 3 * 2^(1 + (packed & 7)).
func gifColorTableBytes(packed byte) int {
	exp := int(packed&gifCTSizeMask) + 1
	return 3 * (1 << exp)
}

// skipImageDescriptor advances past one image descriptor at data[p] (the 0x2C
// separator): the 10-byte descriptor, an optional local color table, the LZW
// minimum-code-size byte, and the image's data sub-blocks. Returns the new
// offset and whether the walk can continue.
func skipImageDescriptor(data []byte, p int) (int, bool) {
	const imageDescLen = 10 // separator(1) left(2) top(2) width(2) height(2) packed(1)
	if p+imageDescLen > len(data) {
		return 0, false
	}
	packed := data[p+9]
	p += imageDescLen
	if packed&gifGCTFlagBit != 0 { // local color table present (same flag bit)
		p += gifColorTableBytes(packed)
	}
	// LZW minimum code size.
	if p >= len(data) {
		return 0, false
	}
	p++
	return skipSubBlocks(data, p)
}

// skipSubBlocks advances past a run of length-prefixed sub-blocks starting at
// data[p], ending after the zero-length terminator block. Returns the new
// offset and whether the run was well-formed enough to continue.
func skipSubBlocks(data []byte, p int) (int, bool) {
	for {
		if p >= len(data) {
			return 0, false
		}
		n := int(data[p])
		p++
		if n == 0 {
			return p, true // block terminator
		}
		p += n
		if p > len(data) {
			return 0, false
		}
	}
}
