package model

import "testing"

func TestPropertyValueEqual_NumericNormalization(t *testing.T) {
	cases := []struct {
		name string
		a, b interface{}
		want bool
	}{
		{"int vs float64", int(5), float64(5), true},
		{"int64 vs float64", int64(5), float64(5), true},
		{"int32 vs int", int32(5), int(5), true},
		{"float32 vs float64", float32(5), float64(5), true},
		{"int vs string", int(5), "5", false},
		{"bool vs string", true, "true", false},
		{"int vs bool", int(1), true, false},
		{"different ints", int(5), int(6), false},
		{"same string", "hello", "hello", true},
		{"different strings", "hello", "world", false},
		{"nil vs nil", nil, nil, true},
		{"nil vs zero", nil, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PropertyValueEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("PropertyValueEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestPropertyValueEqual_NestedStructures(t *testing.T) {
	a := map[string]interface{}{"k": []interface{}{1, 2, 3}}
	b := map[string]interface{}{"k": []interface{}{1, 2, 3}}
	if !PropertyValueEqual(a, b) {
		t.Error("equal nested maps should compare equal")
	}
	c := map[string]interface{}{"k": []interface{}{1, 2, 4}}
	if PropertyValueEqual(a, c) {
		t.Error("differing nested maps should not compare equal")
	}
}

func TestPropertyMapsEqual(t *testing.T) {
	cases := []struct {
		name string
		a, b map[string]interface{}
		want bool
	}{
		{"both nil", nil, nil, true},
		{"both empty", map[string]interface{}{}, map[string]interface{}{}, true},
		{"nil vs empty", nil, map[string]interface{}{}, true},
		{"int vs float", map[string]interface{}{"x": int(5)}, map[string]interface{}{"x": float64(5)}, true},
		{"different keys", map[string]interface{}{"x": 1}, map[string]interface{}{"y": 1}, false},
		{"different lengths", map[string]interface{}{"x": 1}, map[string]interface{}{"x": 1, "y": 2}, false},
		{"different values", map[string]interface{}{"x": 1}, map[string]interface{}{"x": 2}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PropertyMapsEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("PropertyMapsEqual = %v, want %v", got, tc.want)
			}
		})
	}
}
