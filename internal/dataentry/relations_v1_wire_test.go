package dataentry

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestV1RelationsField_LegacyShape_Rejected(t *testing.T) {
	// The legacy IDs-only shape (`["<id>", ...]`) is no longer
	// accepted on the wire. Every relation value must be the JSON:API
	// §9 wrapper. The first occurrence wins so the error path is
	// deterministic.
	body := `{"tagged": ["L-001", "L-002"]}`
	var f V1RelationsField
	err := json.Unmarshal([]byte(body), &f)
	if err == nil {
		t.Fatal("expected error for legacy shape")
	}
	var werr *wireError
	if !errors.As(err, &werr) {
		t.Fatalf("error is not *wireError: %v", err)
	}
	if werr.Code != "legacy_shape_unsupported" {
		t.Errorf("code=%s, want legacy_shape_unsupported", werr.Code)
	}
	if werr.Path != "/relations/tagged" {
		t.Errorf("path=%s, want /relations/tagged", werr.Path)
	}
}

func TestV1RelationsField_ModernShape(t *testing.T) {
	body := `{
		"tagged": {"data": [
			{"type": "label", "id": "L-001", "meta": {"weight": 5}, "meta_unset": ["added_by"]},
			{"type": "label", "id": "L-002"}
		]}
	}`
	var f V1RelationsField
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	upd, ok := f.Modern["tagged"]
	if !ok {
		t.Fatalf("Modern[tagged] missing")
	}
	if !upd.DataPresent {
		t.Errorf("DataPresent should be true")
	}
	if len(upd.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(upd.Data))
	}
	if upd.Data[0].Type != "label" || upd.Data[0].ID != "L-001" {
		t.Errorf("Data[0] = %+v", upd.Data[0])
	}
	if v := upd.Data[0].Meta["weight"]; v != float64(5) {
		t.Errorf("Data[0].Meta[weight] = %v (%T), want 5 (float64)", v, v)
	}
	if len(upd.Data[0].MetaUnset) != 1 || upd.Data[0].MetaUnset[0] != "added_by" {
		t.Errorf("Data[0].MetaUnset = %v", upd.Data[0].MetaUnset)
	}
}

func TestV1RelationsField_DataAbsent_ReturnsDataRequiredAtCallSite(t *testing.T) {
	// Per the contract, the unmarshal succeeds with DataPresent=false
	// and the caller (handler) emits the 400. This test asserts the
	// state propagated correctly.
	body := `{"tagged": {}}`
	var f V1RelationsField
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		t.Fatalf("unmarshal should not error at this layer: %v", err)
	}
	upd, ok := f.Modern["tagged"]
	if !ok {
		t.Fatalf("Modern[tagged] missing")
	}
	if upd.DataPresent {
		t.Errorf("DataPresent should be false")
	}
	if len(upd.Data) != 0 {
		t.Errorf("Data should be empty, got %v", upd.Data)
	}
}

func TestV1RelationsField_DataNull_Rejected(t *testing.T) {
	body := `{"tagged": {"data": null}}`
	var f V1RelationsField
	err := json.Unmarshal([]byte(body), &f)
	if err == nil {
		t.Fatalf("expected error for data: null")
	}
	var werr *wireError
	if !errors.As(err, &werr) {
		t.Fatalf("error is not *wireError: %v", err)
	}
	if werr.Code != "data_required" {
		t.Errorf("code=%s, want data_required", werr.Code)
	}
}

func TestV1RelationsField_DataNonArray_Rejected(t *testing.T) {
	body := `{"tagged": {"data": "L-001"}}`
	var f V1RelationsField
	err := json.Unmarshal([]byte(body), &f)
	if err == nil {
		t.Fatalf("expected error for data: scalar")
	}
	var werr *wireError
	if !errors.As(err, &werr) {
		t.Fatalf("error is not *wireError: %v", err)
	}
	if werr.Code != "data_invalid_type" {
		t.Errorf("code=%s, want data_invalid_type", werr.Code)
	}
}

func TestV1RelationsField_RelationValueNull_Rejected(t *testing.T) {
	body := `{"tagged": null}`
	var f V1RelationsField
	err := json.Unmarshal([]byte(body), &f)
	if err == nil {
		t.Fatalf("expected error for null relation value")
	}
	var werr *wireError
	if !errors.As(err, &werr) {
		t.Fatalf("error is not *wireError: %v", err)
	}
	if werr.Code != "relation_value_null" {
		t.Errorf("code=%s, want relation_value_null", werr.Code)
	}
}

func TestV1RelationsField_RelationValueScalar_Rejected(t *testing.T) {
	cases := []string{
		`{"tagged": "L-001"}`,
		`{"tagged": 5}`,
		`{"tagged": true}`,
	}
	for _, body := range cases {
		var f V1RelationsField
		err := json.Unmarshal([]byte(body), &f)
		if err == nil {
			t.Errorf("body=%s: expected error", body)
			continue
		}
		var werr *wireError
		if !errors.As(err, &werr) {
			t.Errorf("body=%s: error is not *wireError: %v", body, err)
			continue
		}
		if werr.Code != "relation_value_invalid" {
			t.Errorf("body=%s: code=%s, want relation_value_invalid", body, werr.Code)
		}
	}
}

func TestV1RelationsField_UnknownSiblingKey_Rejected(t *testing.T) {
	body := `{"tagged": {"datas": [{"type":"label","id":"L-001"}]}}`
	var f V1RelationsField
	err := json.Unmarshal([]byte(body), &f)
	if err == nil {
		t.Fatalf("expected error for unknown sibling key")
	}
	var werr *wireError
	if !errors.As(err, &werr) {
		t.Fatalf("error is not *wireError: %v", err)
	}
	if werr.Code != "unknown_field" {
		t.Errorf("code=%s, want unknown_field", werr.Code)
	}
}

func TestV1RelationsField_MissingTypeOrID_Rejected(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"tagged": {"data": [{"id": "L-001"}]}}`, "type"},
		{`{"tagged": {"data": [{"type": "label"}]}}`, "id"},
	}
	for _, tc := range cases {
		var f V1RelationsField
		err := json.Unmarshal([]byte(tc.body), &f)
		if err == nil {
			t.Errorf("body=%s: expected error", tc.body)
			continue
		}
		var werr *wireError
		if !errors.As(err, &werr) {
			t.Errorf("body=%s: error is not *wireError: %v", tc.body, err)
			continue
		}
		if werr.Code != "field_required" {
			t.Errorf("body=%s: code=%s, want field_required", tc.body, werr.Code)
		}
	}
}

func TestV1RelationsField_NonStringInMetaUnset_Rejected(t *testing.T) {
	cases := []string{
		`{"tagged": {"data": [{"type":"label","id":"L-001","meta_unset":["x", null]}]}}`,
		`{"tagged": {"data": [{"type":"label","id":"L-001","meta_unset":["x", 5]}]}}`,
	}
	for _, body := range cases {
		var f V1RelationsField
		err := json.Unmarshal([]byte(body), &f)
		if err == nil {
			t.Errorf("body=%s: expected error", body)
			continue
		}
		var werr *wireError
		if !errors.As(err, &werr) {
			t.Errorf("body=%s: error is not *wireError: %v", body, err)
			continue
		}
		if werr.Code != "meta_unset_invalid" && werr.Code != "field_invalid_type" {
			t.Errorf("body=%s: code=%s, want meta_unset_invalid or field_invalid_type", body, werr.Code)
		}
	}
}

func TestV1RelationsField_MetaNull_TreatedAsAbsent(t *testing.T) {
	body := `{"tagged": {"data": [{"type":"label","id":"L-001","meta":null}]}}`
	var f V1RelationsField
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := f.Modern["tagged"].Data[0].Meta; got != nil {
		t.Errorf("Meta should be nil for meta:null, got %v", got)
	}
}

func TestV1RelationsField_ContentNull_TreatedAsAbsent(t *testing.T) {
	body := `{"tagged": {"data": [{"type":"label","id":"L-001","content":null}]}}`
	var f V1RelationsField
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := f.Modern["tagged"].Data[0].Content; got != nil {
		t.Errorf("Content should be nil for content:null, got %v", *got)
	}
}

func TestV1RelationsField_ContentEmptyString_PointerNotNil(t *testing.T) {
	body := `{"tagged": {"data": [{"type":"label","id":"L-001","content":""}]}}`
	var f V1RelationsField
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := f.Modern["tagged"].Data[0].Content
	if got == nil {
		t.Fatalf("Content should be non-nil for content:\"\"")
	}
	if *got != "" {
		t.Errorf("*Content = %q, want empty string", *got)
	}
}

func TestV1RelationsField_EmptyDataMeansRemoveAll(t *testing.T) {
	body := `{"tagged": {"data": []}}`
	var f V1RelationsField
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	upd := f.Modern["tagged"]
	if !upd.DataPresent {
		t.Errorf("DataPresent should be true for data: []")
	}
	if len(upd.Data) != 0 {
		t.Errorf("len(Data) = %d, want 0", len(upd.Data))
	}
}

func TestV1RelationsField_IsEmpty(t *testing.T) {
	var f V1RelationsField
	if !f.IsEmpty() {
		t.Error("zero-value should be empty")
	}
	f.Modern = map[string]V1RelationsUpdate{"a": {Data: []V1ResourceIdentifier{{Type: "x", ID: "y"}}, DataPresent: true}}
	if f.IsEmpty() {
		t.Error("modern populated should not be empty")
	}
}

func TestJSONPointerEscape(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"foo", "foo"},
		{"foo/bar", "foo~1bar"},
		{"foo~bar", "foo~0bar"},
		{"foo~/bar", "foo~0~1bar"}, // ~ replaced first, then /
		{"a/b/c", "a~1b~1c"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := jsonPointerEscape(tc.in); got != tc.want {
			t.Errorf("jsonPointerEscape(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
