// Package v1 holds the v1 data-entry API wire vocabulary: the JSON-shaped
// request/response types the server serializes to and that an in-process native
// bridge can consume directly. It is a pure CONTRACT package — it depends only
// on stdlib and the domain types, never on the data-entry server package.
// A future API version lives in a sibling package (apiwire/v2).
package v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// RelationsField is the top-level value of the `relations` key in a
// PATCH /api/v1/{plural}/{id} or POST /api/v1/{plural} request body.
// Every relation type's value must be the JSON:API §9-shaped wrapper:
// `{"data": [{type, id, meta?, meta_unset?, content?}, ...]}`.
//
// The legacy IDs-only shape (`{"relations": {"<type>": ["<id>", ...]}}`)
// is rejected at unmarshal time with a stable `legacy_shape_unsupported`
// error. The SPA emits modern shape exclusively; external clients should
// follow suit.
type RelationsField struct {
	// Modern holds the JSON:API §9-shaped form, the only shape the
	// wire accepts.
	Modern map[string]RelationsUpdate
}

// IsEmpty reports whether the relations field was absent or `{}` in the
// request body.
func (f RelationsField) IsEmpty() bool {
	return len(f.Modern) == 0
}

// RelationsUpdate is the JSON:API §9 wrapper for one relation type's
// desired state. The wrapper has exactly one field, `data`, which is the
// full desired set of edges of this relation type.
//
// `DataPresent` distinguishes three wire-level cases:
//   - `{"tagged": {}}`           → DataPresent=false → 400 `data_required`
//   - `{"tagged": {"data": null}}` → DataPresent=false → 400 `data_required`
//   - `{"tagged": {"data": []}}` → DataPresent=true, Data=[] → remove all
//   - `{"tagged": {"data": [...]}}` → DataPresent=true, Data populated
//
// Sending `data: []` removes every edge of this relation type from the
// entity. See docs/data-entry/api-reference.md for the full footgun
// callout.
type RelationsUpdate struct {
	Data        []ResourceIdentifier
	DataPresent bool
}

// ResourceIdentifier is the per-edge resource identifier in a JSON:API
// §9-shaped relation update. `Type` and `ID` identify the target;
// `Meta`, `MetaUnset`, and `Content` carry per-edge upsert data.
type ResourceIdentifier struct {
	Type      string                 `json:"type"`
	ID        string                 `json:"id"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
	MetaUnset []string               `json:"meta_unset,omitempty"`
	// Content is a pointer so a missing field (leave alone) can be
	// distinguished from an explicit empty string (clear the body).
	Content *string `json:"content,omitempty"`
}

// WireError is the structured error returned from RelationsField's
// unmarshal. Code is stable for clients; Path is an RFC 6901 JSON
// Pointer; Detail is human-readable.
type WireError struct {
	Code   string
	Path   string
	Detail string
}

func (e *WireError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s (path: %s)", e.Code, e.Detail, e.Path)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

// UnmarshalJSON decodes the relations field. Every value must be the
// JSON:API §9-shaped wrapper `{"data": [...]}`. Array-shaped values
// (the legacy IDs-only form) are rejected with `legacy_shape_unsupported`
// so callers see a clear "update your client" error rather than a
// generic JSON decode failure.
func (f *RelationsField) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	modern := make(map[string]RelationsUpdate)

	for relType, val := range raw {
		trimmed := bytes.TrimLeftFunc(val, unicode.IsSpace)
		if len(trimmed) == 0 {
			return &WireError{
				Code:   "relation_value_invalid",
				Path:   "/relations/" + JSONPointerEscape(relType),
				Detail: "relation type value is empty",
			}
		}
		switch trimmed[0] {
		case '[':
			return &WireError{
				Code: "legacy_shape_unsupported",
				Path: "/relations/" + JSONPointerEscape(relType),
				Detail: "legacy IDs-only relation shape (`[\"<id>\", ...]`) is no longer accepted; " +
					"use the JSON:API §9 wrapper `{\"data\": [{\"type\": \"...\", \"id\": \"...\"}, ...]}`",
			}
		case '{':
			update, err := decodeRelationsUpdate(relType, val)
			if err != nil {
				return err
			}
			modern[relType] = update
		case 'n':
			if string(trimmed) == "null" {
				return &WireError{
					Code:   "relation_value_null",
					Path:   "/relations/" + JSONPointerEscape(relType),
					Detail: "relation type value cannot be null; use \"data\": [] to clear all edges",
				}
			}
			return &WireError{
				Code:   "relation_value_invalid",
				Path:   "/relations/" + JSONPointerEscape(relType),
				Detail: "relation type value must be the JSON:API §9 wrapper `{\"data\": [...]}`",
			}
		default:
			return &WireError{
				Code:   "relation_value_invalid",
				Path:   "/relations/" + JSONPointerEscape(relType),
				Detail: "relation type value must be the JSON:API §9 wrapper `{\"data\": [...]}`",
			}
		}
	}

	if len(modern) > 0 {
		f.Modern = modern
	}
	return nil
}

// decodeRelationsUpdate handles the modern `{"data": [...]}` wrapper.
// Returns a WireError on any malformed input.
func decodeRelationsUpdate(relType string, raw json.RawMessage) (RelationsUpdate, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return RelationsUpdate{}, &WireError{
			Code:   "wrapper_invalid",
			Path:   "/relations/" + JSONPointerEscape(relType),
			Detail: "wrapper must be a JSON object with a `data` array",
		}
	}

	for k := range fields {
		if k != "data" {
			return RelationsUpdate{}, &WireError{
				Code:   "unknown_field",
				Path:   "/relations/" + JSONPointerEscape(relType) + "/" + JSONPointerEscape(k),
				Detail: fmt.Sprintf("unknown field %q on relation wrapper (only `data` is allowed)", k),
			}
		}
	}

	dataRaw, present := fields["data"]
	if !present {
		return RelationsUpdate{DataPresent: false}, nil
	}

	trimmed := bytes.TrimLeftFunc(dataRaw, unicode.IsSpace)
	if len(trimmed) == 0 {
		return RelationsUpdate{}, &WireError{
			Code:   "data_required",
			Path:   "/relations/" + JSONPointerEscape(relType) + "/data",
			Detail: "`data` must be an array",
		}
	}
	if string(trimmed) == "null" {
		// Treated identically to the data-absent case (per RR-UZ8LX).
		return RelationsUpdate{}, &WireError{
			Code:   "data_required",
			Path:   "/relations/" + JSONPointerEscape(relType) + "/data",
			Detail: "`data` cannot be null; use [] to clear all edges of this type",
		}
	}
	if trimmed[0] != '[' {
		return RelationsUpdate{}, &WireError{
			Code:   "data_invalid_type",
			Path:   "/relations/" + JSONPointerEscape(relType) + "/data",
			Detail: "`data` must be an array of resource identifiers",
		}
	}

	// Validate meta_unset arrays element-by-element BEFORE the
	// struct unmarshal — Go's json.Unmarshal coerces `null` into the
	// empty string when decoding `[]string`, masking what should be
	// a wire-format error.
	if err := validateMetaUnsetElements(relType, dataRaw); err != nil {
		return RelationsUpdate{}, err
	}

	var refs []ResourceIdentifier
	dec := json.NewDecoder(bytes.NewReader(dataRaw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&refs); err != nil {
		return RelationsUpdate{}, translateRefDecodeError(relType, dataRaw, err)
	}

	for i, ref := range refs {
		if ref.Type == "" {
			return RelationsUpdate{}, &WireError{
				Code:   "field_required",
				Path:   fmt.Sprintf("/relations/%s/data/%d/type", JSONPointerEscape(relType), i),
				Detail: "`type` is required on each resource identifier",
			}
		}
		if ref.ID == "" {
			return RelationsUpdate{}, &WireError{
				Code:   "field_required",
				Path:   fmt.Sprintf("/relations/%s/data/%d/id", JSONPointerEscape(relType), i),
				Detail: "`id` is required on each resource identifier",
			}
		}
	}

	return RelationsUpdate{Data: refs, DataPresent: true}, nil
}

// validateMetaUnsetElements scans the raw JSON for the data array and
// rejects meta_unset arrays that contain non-string elements. Run
// before the struct decode because json.Unmarshal silently coerces
// `null` to the empty string when target type is string, and we want
// to fail loudly on that case.
func validateMetaUnsetElements(relType string, dataRaw json.RawMessage) error {
	var rawRefs []json.RawMessage
	if err := json.Unmarshal(dataRaw, &rawRefs); err != nil {
		// Not an array — the main decoder will surface the right error
		// in a WireError shape; we deliberately swallow this one to
		// avoid double-reporting.
		return nil //nolint:nilerr // see comment
	}
	for i, rawRef := range rawRefs {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawRef, &fields); err != nil {
			continue // not an object; main decoder will fail
		}
		muRaw, ok := fields["meta_unset"]
		if !ok {
			continue
		}
		trimmed := bytes.TrimLeftFunc(muRaw, unicode.IsSpace)
		if len(trimmed) == 0 || string(trimmed) == "null" {
			continue // null/absent treated as absent
		}
		if trimmed[0] != '[' {
			return &WireError{
				Code:   "meta_unset_invalid",
				Path:   fmt.Sprintf("/relations/%s/data/%d/meta_unset", JSONPointerEscape(relType), i),
				Detail: "`meta_unset` must be an array of strings",
			}
		}
		var elems []json.RawMessage
		if err := json.Unmarshal(muRaw, &elems); err != nil {
			return &WireError{
				Code:   "meta_unset_invalid",
				Path:   fmt.Sprintf("/relations/%s/data/%d/meta_unset", JSONPointerEscape(relType), i),
				Detail: err.Error(),
			}
		}
		for j, e := range elems {
			t := bytes.TrimLeftFunc(e, unicode.IsSpace)
			if len(t) == 0 || t[0] != '"' {
				return &WireError{
					Code: "meta_unset_invalid",
					Path: fmt.Sprintf("/relations/%s/data/%d/meta_unset/%d",
						JSONPointerEscape(relType), i, j),
					Detail: "`meta_unset` elements must be strings",
				}
			}
		}
	}
	return nil
}

// translateRefDecodeError maps json.Decoder errors on a resource
// identifier slice into a structured WireError. Covers unknown-field
// rejection and the most common malformed-shape cases.
func translateRefDecodeError(relType string, dataRaw json.RawMessage, err error) error {
	msg := err.Error()
	// Best-effort path inference: when DisallowUnknownFields rejects a
	// field, the message names which one. We don't get a stable
	// element index, so we cite the array generically.
	if strings.Contains(msg, "unknown field") {
		return &WireError{
			Code:   "unknown_field",
			Path:   "/relations/" + JSONPointerEscape(relType) + "/data",
			Detail: msg,
		}
	}
	// Best-effort: catch the meta_unset non-string element case, which
	// json reports as "cannot unmarshal X into Go struct field
	// ResourceIdentifier.meta_unset of type string".
	if strings.Contains(msg, ".meta_unset of type string") {
		return &WireError{
			Code:   "meta_unset_invalid",
			Path:   "/relations/" + JSONPointerEscape(relType) + "/data",
			Detail: "`meta_unset` must contain only strings",
		}
	}
	// Fallback: generic body-invalid.
	_ = dataRaw // reserved for future richer parsing
	var jsonErr *json.UnmarshalTypeError
	if errors.As(err, &jsonErr) {
		return &WireError{
			Code:   "field_invalid_type",
			Path:   "/relations/" + JSONPointerEscape(relType) + "/data/" + jsonErr.Field,
			Detail: msg,
		}
	}
	return &WireError{
		Code:   "data_invalid",
		Path:   "/relations/" + JSONPointerEscape(relType) + "/data",
		Detail: msg,
	}
}

// JSONPointerEscape applies RFC 6901 JSON Pointer escaping to a single
// reference token: `~` is encoded as `~0` and `/` as `~1`. The
// substitutions are NOT commutative — `~` must be replaced before `/`.
func JSONPointerEscape(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}
