package websocket

// rela fork: this file is not part of upstream ygo.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/reearth/ygo/encoding"
)

// maxStampedEntries bounds how many entries stampAwareness decodes from one
// frame. The room's own caps refuse anything past a few clients per
// connection, so a frame this large is malformed or hostile.
const maxStampedEntries = 1024

// stampAwareness decodes an awareness update, passes every non-null state
// through stamp, and encodes the result in the same wire order. A state that
// is not a JSON object is an error, so the caller drops the whole frame.
func stampAwareness(frame []byte, stamp func(map[string]any) map[string]any) ([]byte, error) {
	dec := encoding.NewDecoder(frame)
	n, err := dec.ReadVarUint()
	if err != nil {
		return nil, fmt.Errorf("read entry count: %w", err)
	}
	if n > maxStampedEntries {
		return nil, fmt.Errorf("awareness frame has %d entries, limit %d", n, maxStampedEntries)
	}
	enc := encoding.NewEncoder()
	enc.WriteVarUint(n)
	for i := uint64(0); i < n; i++ {
		clientID, err := dec.ReadVarUint()
		if err != nil {
			return nil, fmt.Errorf("read client id: %w", err)
		}
		clock, err := dec.ReadVarUint()
		if err != nil {
			return nil, fmt.Errorf("read clock: %w", err)
		}
		raw, err := dec.ReadVarBytes()
		if err != nil {
			return nil, fmt.Errorf("read state: %w", err)
		}
		out, err := stampState(raw, stamp)
		if err != nil {
			return nil, err
		}
		enc.WriteVarUint(clientID)
		enc.WriteVarUint(clock)
		enc.WriteVarBytes(out)
	}
	return enc.Bytes(), nil
}

// maxStampedStateBytes and maxStampedDepth mirror the awareness package's
// per-state limits, so a state it would refuse is never decoded here.
const (
	maxStampedStateBytes = 1 << 20
	maxStampedDepth      = 20
)

// errStateTooLarge is returned for a state over the size or depth limit.
var errStateTooLarge = errors.New("awareness state over the size or depth limit")

// errStampNil is returned when the stamp returns nil, which would otherwise
// encode as a removal of the client.
var errStampNil = errors.New("awareness stamp returned nil")

// errStateNotObject is returned for a live awareness state that is not a
// JSON object.
var errStateNotObject = errors.New("awareness state is not a JSON object")

// stampState rewrites one encoded state. A null or empty state removes the
// client and passes through unchanged.
func stampState(raw []byte, stamp func(map[string]any) map[string]any) ([]byte, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return raw, nil
	}
	if len(raw) > maxStampedStateBytes || !shallowJSON(raw) {
		return nil, errStateTooLarge
	}
	// UseNumber keeps numbers exactly as sent; a float64 round trip could
	// change large integers inside cursor positions.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var state map[string]any
	if err := dec.Decode(&state); err != nil || state == nil || dec.More() {
		return nil, errStateNotObject
	}
	stamped := stamp(state)
	if stamped == nil {
		return nil, errStampNil
	}
	// No HTML escaping: it would grow the state, and peers parse it as JSON.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(stamped); err != nil {
		return nil, fmt.Errorf("encode stamped state: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// shallowJSON reports whether raw nests at most maxStampedDepth levels. It
// scans bytes without parsing, skipping brackets inside strings. It is the
// awareness package's checkJSONDepth (unexported upstream), so the limit is
// enforced before encoding/json, which has none, decodes the state.
func shallowJSON(raw []byte) bool {
	depth := 0
	inString := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if inString {
			switch c {
			case '\\':
				i++ // skip the escaped character
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxStampedDepth {
				return false
			}
		case '}', ']':
			depth--
		}
	}
	return !inString
}
