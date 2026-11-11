package realtimecfg

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// objectSrc is a channel type whose ask type holds an object list of
// options. prop replaces the options declaration.
func objectSrc(prop string) string {
	return "version: 1\nchannel_types:\n  room:\n    attach_to: [datalek]\n    roles: {m: {from: attached}}\n" +
		"    message_types:\n      ask:\n        post: [m]\n        properties:\n          options: " + prop + "\n"
}

const objectKey = "channel_types.room.message_types.ask.properties.options"

const optionsProp = "{ type: object, list: true, max_items: 4, properties: { " +
	"label: { type: string, required: true, max: 40 }, pros: { type: string, list: true }, " +
	"cons: { type: string, list: true }, recommended: { type: boolean }, weight: { type: integer }, " +
	"risk: { type: severity } } }"

// objectOptions parses objectSrc(optionsProp) and returns the options
// property.
func objectOptions(t *testing.T) Property {
	t.Helper()
	c, err := Parse([]byte(objectSrc(optionsProp)), testMeta(t), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, _ := c.ChannelType("room")
	return ct.MessageTypes["ask"].Properties["options"]
}

// TestParse_ObjectProperty pins AC 1 of TKT-GR1MXL5 at load: an object
// list compiles with its fields, and each bad declaration is refused at
// its key.
func TestParse_ObjectProperty(t *testing.T) {
	p := objectOptions(t)
	if !p.IsObjectList() || !p.List || p.MaxItems != 4 {
		t.Fatalf("options = %+v; want an object list with max_items 4", p)
	}
	if got := p.TypeName(); got != "list of object" {
		t.Errorf("TypeName = %q", got)
	}
	label := p.Fields["label"]
	if label.Type != PropertyString || !label.Required || label.Max != 40 {
		t.Errorf("label = %+v", label)
	}
	if pros := p.Fields["pros"]; !pros.List || pros.Max != realtimelimits.DefaultListItemRunes {
		t.Errorf("pros = %+v; want a string list", pros)
	}
	if risk := p.Fields["risk"]; risk.Type != PropertyEnum {
		t.Errorf("risk = %+v; want an enum", risk)
	}

	t.Run("default max_items", func(t *testing.T) {
		c, err := Parse([]byte(objectSrc("{ type: object, list: true, properties: { a: { type: string } } }")),
			testMeta(t), nil)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		ct, _ := c.ChannelType("room")
		if got := ct.MessageTypes["ask"].Properties["options"].MaxItems; got != realtimelimits.MaxListItems {
			t.Errorf("MaxItems = %d", got)
		}
	})

	many := make([]string, realtimelimits.MaxObjectFields+1)
	for i := range many {
		many[i] = "f" + strings.Repeat("x", i) + ": { type: string }"
	}
	refused := []struct {
		name, prop, key, reason string
	}{
		{"not a list", "{ type: object, properties: { a: { type: string } } }", objectKey + ".list",
			"an object property is a list of objects; add list: true"},
		{"no fields", "{ type: object, list: true }", objectKey + ".properties",
			"an object property declares its fields under properties"},
		{"too many fields", "{ type: object, list: true, properties: { " + strings.Join(many, ", ") + " } }",
			objectKey + ".properties", "an object has at most 16 fields"},
		{"max on the list", "{ type: object, list: true, max: 3, properties: { a: { type: string } } }",
			objectKey + ".max", "an object property has no max"},
		{"to on the list", "{ type: object, list: true, to: [datalek], properties: { a: { type: string } } }",
			objectKey + ".to", "only a ref property has to"},
		{"notify on the list", "{ type: object, list: true, notify: false, properties: { a: { type: string } } }",
			objectKey + ".notify", "only a ref property has notify"},
		{"max_items over the cap", "{ type: object, list: true, max_items: 21, properties: { a: { type: string } } }",
			objectKey + ".max_items", "must be between 1 and 20"},
		{"nested object", "{ type: object, list: true, properties: { a: { type: object, list: true, " +
			"properties: { b: { type: string } } } } }",
			objectKey + ".properties.a.type", "objects nest one level only"},
		{"markdown field", "{ type: object, list: true, properties: { a: { type: markdown } } }",
			objectKey + ".properties.a.type", "not markdown"},
		{"ref field", "{ type: object, list: true, properties: { a: { type: ref, to: [datalek] } } }",
			objectKey + ".properties.a.type", "not ref"},
		{"bad field name", "{ type: object, list: true, properties: { A: { type: string } } }",
			objectKey + ".properties.A", "property name must match"},
		{"properties on a string", "{ type: string, properties: { a: { type: string } } }",
			objectKey + ".properties", "only an object property has properties"},
		{"unknown field key", "{ type: object, list: true, properties: { a: { type: string, shade: red } } }",
			objectKey + ".properties.a.shade", "unknown key"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(objectSrc(tc.prop)), testMeta(t), nil)
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestNormalize_ObjectList pins AC 1 of TKT-GR1MXL5 on post: valid items
// keep only their set fields, normalized, and each bad value is refused
// with a reason that names the item and the field.
func TestNormalize_ObjectList(t *testing.T) {
	p := objectOptions(t)
	var in []any
	if err := json.Unmarshal([]byte(`[
		{"label": "A", "pros": ["fast", ""], "recommended": true, "weight": 2, "risk": "high", "cons": []},
		{"label": "B", "cons": ["slow"], "pros": null}
	]`), &in); err != nil {
		t.Fatal(err)
	}
	// An empty string in a string list is refused by normalizeList; drop it
	// for the valid case.
	in[0].(map[string]any)["pros"] = []any{"fast"}
	got, err := p.Normalize(in)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	b, _ := json.Marshal(got)
	want := `[{"label":"A","pros":["fast"],"recommended":true,"risk":"high","weight":2},{"cons":["slow"],"label":"B"}]`
	if string(b) != want {
		t.Errorf("Normalize = %s; want %s", b, want)
	}

	t.Run("Go maps", func(t *testing.T) {
		if _, err := p.Normalize([]map[string]any{{"label": "A"}}); err != nil {
			t.Errorf("Normalize: %v", err)
		}
	})

	refused := []struct {
		name   string
		in     any
		reason string
	}{
		{"not a list", "A", "must be a list of objects"},
		{"item not an object", []any{"A"}, "item 1 must be an object"},
		{"unknown field", []any{map[string]any{"label": "A", "shade": "red"}},
			`item 1 has field "shade", which options does not declare`},
		{"missing required field", []any{map[string]any{"pros": []any{"x"}}}, `item 1 needs field "label"`},
		{"empty item", []any{map[string]any{}}, "item 1 is empty"},
		{"only nil fields", []any{map[string]any{"label": nil}}, "item 1 is empty"},
		{"wrong field type", []any{map[string]any{"label": "A", "weight": "two"}},
			`item 1 field "weight" must be a whole number`},
		{"enum outside values", []any{map[string]any{"label": "A", "risk": "huge"}},
			`item 1 field "risk" must be one of`},
		{"field over max", []any{map[string]any{"label": strings.Repeat("x", 41)}},
			`item 1 field "label" must be at most 40 characters`},
		{"list field item", []any{map[string]any{"label": "A", "pros": []any{1}}},
			`item 1 field "pros" item 1 must be a string`},
		{"too many items", []any{
			map[string]any{"label": "A"}, map[string]any{"label": "B"}, map[string]any{"label": "C"},
			map[string]any{"label": "D"}, map[string]any{"label": "E"},
		}, "must have at most 4 items"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Normalize(tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("Normalize = %v; want %q", err, tc.reason)
			}
		})
	}

	t.Run("duplicate labels are stored without a key", func(t *testing.T) {
		if _, err := p.Normalize([]any{map[string]any{"label": "A"}, map[string]any{"label": "A"}}); err != nil {
			t.Errorf("Normalize: %v", err)
		}
	})
}

// objectChoicesSrc is choicesBase with an object list of options and
// choices.label.
func objectChoicesSrc(t *testing.T, edits ...replace) string {
	t.Helper()
	base := []replace{
		{"options:  { type: string, list: true, max: 80, max_items: 4 }",
			"options:  { type: object, list: true, max_items: 4, properties: { label: { type: string, " +
				"required: true, max: 60 }, pros: { type: string, list: true } } }"},
		{"choices: { from: options, ", "choices: { from: options, label: label, "},
	}
	return choicesSrc(t, append(base, edits...)...)
}

// TestChoices_ObjectLabel pins AC 2 of TKT-GR1MXL5: choices on an object
// list load with label, which becomes the list's key, and each bad label
// is refused.
func TestChoices_ObjectLabel(t *testing.T) {
	c, err := Parse([]byte(objectChoicesSrc(t)), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}
	ct, _ := c.ChannelType("run")
	ask := ct.MessageTypes["ask"]
	if ask.Choices.Label != "label" {
		t.Errorf("Choices = %+v", ask.Choices)
	}
	p := ask.Properties["options"]
	if p.Key != "label" {
		t.Errorf("options.Key = %q; want label", p.Key)
	}

	t.Run("the key refuses duplicate labels, folded", func(t *testing.T) {
		_, err := p.Normalize([]any{map[string]any{"label": "Go"}, map[string]any{"label": " Go "}})
		if err == nil || !strings.Contains(err.Error(), "item 2 repeats the label of an earlier item") {
			t.Errorf("Normalize = %v", err)
		}
	})
	t.Run("the key is checked as a list item", func(t *testing.T) {
		_, err := p.Normalize([]any{map[string]any{"label": "a\u202eb"}})
		if err == nil || !strings.Contains(err.Error(), `field "label" holds U+202E`) {
			t.Errorf("Normalize = %v", err)
		}
		_, err = p.Normalize([]any{map[string]any{"label": "  "}})
		if err == nil || !strings.Contains(err.Error(), `field "label" is empty`) {
			t.Errorf("Normalize = %v", err)
		}
	})

	cases := []struct {
		name   string
		edits  []replace
		key    string
		reason string
	}{
		{"label missing", []replace{{"label: label, ", ""}}, choicesKey + ".label",
			"property options is a list of objects; name the field that holds the button text"},
		{"label names no field", []replace{{"label: label, ", "label: name, "}}, choicesKey + ".label",
			`property options has no field "name"`},
		{"label is a list", []replace{{"label: label, ", "label: pros, "}}, choicesKey + ".label",
			"field pros is of type list of string"},
		{"label is optional", []replace{{"label: { type: string, required: true, max: 60 }",
			"label: { type: string, max: 60 }"}}, choicesKey + ".label", "add required: true"},
		{"label is an integer", []replace{{"label: { type: string, required: true, max: 60 }",
			"label: { type: integer, required: true }"}}, choicesKey + ".label",
			"field label is of type integer"},
		{"into too short for the label", []replace{{"properties: { body: { type: string, required: true } }",
			"properties: { body: { type: string, required: true, max: 10 } }"}}, choicesKey + ".into",
			"an option of up to 60 characters does not fit into body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(objectChoicesSrc(t, tc.edits...)), testMeta(t), testPolicy(t))
			assertProblem(t, err, tc.key, tc.reason)
		})
	}

	t.Run("label on a string list", func(t *testing.T) {
		src := choicesSrc(t, replace{"choices: { from: options, ", "choices: { from: options, label: x, "})
		_, err := Parse([]byte(src), testMeta(t), testPolicy(t))
		assertProblem(t, err, choicesKey+".label", "property options is a list of strings")
	})

	t.Run("long labels warn", func(t *testing.T) {
		c, err := Parse([]byte(objectChoicesSrc(t, replace{"max: 60 }", "max: 300 }"})), testMeta(t), testPolicy(t))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if w := fmt.Sprint(c.Warnings()); !strings.Contains(w, "options of up to 300 characters") {
			t.Errorf("warnings = %v", c.Warnings())
		}
	})
}

// TestObjectList_SchemaAndPredicate pins that an object list is an array
// of closed objects in JSON Schema, and a list of records to a `when:`.
func TestObjectList_SchemaAndPredicate(t *testing.T) {
	p := objectOptions(t)
	s := p.schema()
	if s.Type != "array" || s.MaxItems != 4 || s.Items == nil || s.Items.Type != "object" {
		t.Fatalf("schema = %+v", s)
	}
	if s.Items.AdditionalProperties == nil || *s.Items.AdditionalProperties {
		t.Errorf("items must be closed: %+v", s.Items)
	}
	if strings.Join(s.Items.Required, ",") != "label" || s.Items.Properties["pros"].Type != "array" {
		t.Errorf("items = %+v", s.Items)
	}
	b, err := json.Marshal(s)
	if err != nil || !strings.Contains(string(b), `"additionalProperties":false`) {
		t.Errorf("JSON = %s, %v", b, err)
	}

	rec := messageRecordType(map[string]Property{"options": p})
	if _, ok := rec["options"].(predicate.ListType); !ok {
		t.Fatalf("record type = %#v", rec["options"])
	}
	v := p.PredicateValue([]any{map[string]any{"label": "A", "weight": int64(2)}})
	list, ok := v.(predicate.List)
	if !ok || len(list.Elems()) != 1 {
		t.Fatalf("PredicateValue = %#v", v)
	}

	// A string-list function on an object list is refused at load, not
	// silently fed an empty list.
	src := strings.Replace(objectSrc(optionsProp), "        post: [m]\n",
		"        post: [m]\n        when: \"len(message.options) > 1\"\n", 1)
	_, err = Parse([]byte(src), testMeta(t), nil)
	assertProblem(t, err, "channel_types.room.message_types.ask.when", "does not compile")
}

// TestParse_ObjectListEphemeral pins that an ephemeral type, which shows as
// one line of text, cannot hold an object list.
func TestParse_ObjectListEphemeral(t *testing.T) {
	src := strings.Replace(objectSrc(optionsProp), "        post: [m]\n",
		"        post: [m]\n        persist: ephemeral\n        ttl: 5s\n", 1)
	_, err := Parse([]byte(src), testMeta(t), nil)
	assertProblem(t, err, objectKey, "cannot hold an object list")
}
