package dataentryconfig

import (
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/ingestspec"
)

// TestWebhookMapping_CarriesEveryMappingField pins that Webhook declares every
// ingestspec.Mapping field with the same type and tags, and that Mapping()
// copies each one. A field added to Mapping for another source would otherwise
// be silently dropped for webhooks.
func TestWebhookMapping_CarriesEveryMappingField(t *testing.T) {
	hookType := reflect.TypeFor[Webhook]()
	hook := Webhook{
		Find:            &WebhookFind{Type: "a"},
		CreateIfMissing: &WebhookCreate{Type: "b"},
		Then:            []WebhookStep{{Set: map[string]string{"k": "v"}}},
	}
	got := reflect.ValueOf(hook.Mapping())
	want := reflect.ValueOf(hook)

	for _, f := range reflect.VisibleFields(reflect.TypeFor[ingestspec.Mapping]()) {
		hf, ok := hookType.FieldByName(f.Name)
		if !ok {
			t.Errorf("Webhook has no field %s", f.Name)
			continue
		}
		if hf.Type != f.Type || hf.Tag != f.Tag {
			t.Errorf("Webhook.%s = %v `%s`, want %v `%s`", f.Name, hf.Type, hf.Tag, f.Type, f.Tag)
		}
		if !reflect.DeepEqual(got.FieldByName(f.Name).Interface(), want.FieldByName(f.Name).Interface()) {
			t.Errorf("Mapping() does not copy %s", f.Name)
		}
	}
}
