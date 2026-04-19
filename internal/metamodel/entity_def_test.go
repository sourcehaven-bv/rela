package metamodel

import (
	"reflect"
	"testing"
)

func TestEntityDef_EncryptedProperties_Empty(t *testing.T) {
	e := &EntityDef{Properties: map[string]PropertyDef{
		"title":  {Type: "string"},
		"status": {Type: "string"},
	}}
	got := e.EncryptedProperties()
	if got == nil {
		t.Fatal("EncryptedProperties should return empty map, not nil")
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestEntityDef_EncryptedProperties_Mixed(t *testing.T) {
	e := &EntityDef{Properties: map[string]PropertyDef{
		"title":       {Type: "string"},
		"description": {Type: "string", Encrypted: "engineering"},
		"notes":       {Type: "string", Encrypted: "exec"},
	}}
	got := e.EncryptedProperties()
	want := map[string]string{
		"description": "engineering",
		"notes":       "exec",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEntityDef_PropertiesByGroup(t *testing.T) {
	e := &EntityDef{Properties: map[string]PropertyDef{
		"title":       {Type: "string"},
		"description": {Type: "string", Encrypted: "engineering"},
		"notes":       {Type: "string", Encrypted: "engineering"},
		"secret":      {Type: "string", Encrypted: "exec"},
	}}
	got := e.PropertiesByGroup()
	want := map[string][]string{
		"engineering": {"description", "notes"}, // sorted
		"exec":        {"secret"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEntityDef_PropertiesByGroup_Empty(t *testing.T) {
	e := &EntityDef{Properties: map[string]PropertyDef{
		"title": {Type: "string"},
	}}
	got := e.PropertiesByGroup()
	if got == nil {
		t.Fatal("PropertiesByGroup should return empty map, not nil")
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestEntityDef_BodyGroup_Cleartext(t *testing.T) {
	e := &EntityDef{}
	group, encrypted := e.BodyGroup()
	if encrypted {
		t.Errorf("empty EncryptedBody should be cleartext; got group=%q", group)
	}
	if group != "" {
		t.Errorf("group = %q, want empty", group)
	}
}

func TestEntityDef_BodyGroup_Encrypted(t *testing.T) {
	e := &EntityDef{EncryptedBody: "exec"}
	group, encrypted := e.BodyGroup()
	if !encrypted {
		t.Fatal("EncryptedBody=exec should report encrypted")
	}
	if group != "exec" {
		t.Errorf("group = %q, want %q", group, "exec")
	}
}
