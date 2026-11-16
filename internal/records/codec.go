package records

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// EncodeFields returns the JSON form of a row's fields, the form every
// shipped backend stores.
func EncodeFields(f map[string]any) (string, error) {
	if f == nil {
		f = map[string]any{}
	}
	b, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("records: encode fields: %w", err)
	}
	return string(b), nil
}

// DecodeFields is the inverse of [EncodeFields]. A JSON number is an
// integer field, the only kind stored as a number, and decodes to int64; a
// decimal is stored as text and stays a string.
func DecodeFields(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var f map[string]any
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("records: decode fields: %w", err)
	}
	for k, v := range f {
		n, ok := v.(json.Number)
		if !ok {
			continue
		}
		i, err := strconv.ParseInt(n.String(), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("records: decode fields: %s: %w", k, err)
		}
		f[k] = i
	}
	if f == nil {
		f = map[string]any{}
	}
	return f, nil
}

// EncodeRecord returns rec as one JSON line, without the newline.
func EncodeRecord(rec Record) ([]byte, error) {
	b, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("records: encode: %w", err)
	}
	return b, nil
}

// DecodeRecord is the inverse of [EncodeRecord].
func DecodeRecord(line []byte) (Record, error) {
	var wire struct {
		Record
		Fields json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(line, &wire); err != nil {
		return Record{}, fmt.Errorf("records: decode: %w", err)
	}
	rec := wire.Record
	f, err := DecodeFields(wire.Fields)
	if err != nil {
		return Record{}, err
	}
	rec.Fields = f
	rec.TS = rec.TS.UTC()
	rec.RecordedAt = rec.RecordedAt.UTC()
	return rec, nil
}
