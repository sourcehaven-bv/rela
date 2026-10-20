package dataentryconfig

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// fakeChannelSchema declares channel type room with message types text
// and note.
type fakeChannelSchema struct{}

func (fakeChannelSchema) ChannelTypeNames() []string { return []string{"room"} }

func (fakeChannelSchema) MessageTypeNames(ct string) []string {
	if ct != "room" {
		return nil
	}
	return []string{"note", "text"}
}

func (fakeChannelSchema) MessageProperties(ct, mt string) []ChannelProperty {
	switch {
	case ct != "room":
		return nil
	case mt == "text":
		return []ChannelProperty{{Name: "body", Type: "markdown", Required: true}}
	case mt == "note":
		return []ChannelProperty{
			{Name: "summary", Type: "string", Required: true},
			{Name: "level", Type: "integer"},
			{Name: "about", Type: "ref"},
			{Name: "pick", Type: "enum"},
		}
	}
	return nil
}

// TestChannelsPresentation_Validate pins the `channels:` checks: the shape
// checks ValidateConfig runs alone, and the realtime.yaml checks of
// ValidateChannels.
func TestChannelsPresentation_Validate(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string // substring; "" means success
	}{
		{"absent", ``, ""},
		{"full", `channels:
  room:
    messages:
      note:
        fields:
          - { property: summary, label: Decision, widget: textarea, help: One line, placeholder: x }
          - { property: level, span: 6, widget: number }
          - { property: pick, widget: select }
          - { property: about, hidden: true }
      text:
        fields: [{ property: body, widget: textarea }]`, ""},
		{"unknown channel key", "channels: {room: {message: {}}}", `channels.room: unknown key "message"`},
		{"unknown message key", "channels: {room: {messages: {text: {feilds: []}}}}",
			`channels.room.messages.text: unknown key "feilds"`},
		{"unknown field key", "channels: {room: {messages: {text: {fields: [{property: body, color: red}]}}}}",
			`fields[0]: unknown key "color"`},
		{"refused visible_when", "channels: {room: {messages: {text: {fields: [{property: body, visible_when: x}]}}}}",
			`"visible_when" is not supported in a channel field`},
		{"refused required", "channels: {room: {messages: {text: {fields: [{property: body, required: true}]}}}}",
			`"required" is not supported`},
		{"refused default", "channels: {room: {messages: {text: {fields: [{property: body, default: x}]}}}}",
			`"default" is not supported`},
		{"no property", "channels: {room: {messages: {text: {fields: [{label: x}]}}}}", `fields[0]: property is required`},
		{"twice", "channels: {room: {messages: {text: {fields: [{property: body}, {property: body}]}}}}",
			`fields[1]: property "body" is listed twice`},
		{"form widget", "channels: {room: {messages: {text: {fields: [{property: body, widget: rrule}]}}}}",
			`widget "rrule" cannot be used in a channel`},
		{"span", "channels: {room: {messages: {text: {fields: [{property: body, span: 13}]}}}}", `span 13 is out of range`},

		{"unknown channel type", "channels: {hall: {messages: {}}}", `realtime.yaml declares no channel type "hall"`},
		{"unknown message type", "channels: {room: {messages: {poll: {}}}}", `declares no message type "poll"`},
		{"unknown property", "channels: {room: {messages: {text: {fields: [{property: title}]}}}}",
			`message type text declares no property "title"`},
		{"hidden required", "channels: {room: {messages: {text: {fields: [{property: body, hidden: true}]}}}}",
			`property "body" is required, so it cannot be hidden`},
		{"widget type", "channels: {room: {messages: {note: {fields: [{property: level, widget: checkbox}]}}}}",
			`widget "checkbox" cannot edit property "level" of type integer`},
		{"widget on ref", "channels: {room: {messages: {note: {fields: [{property: about, widget: text}]}}}}",
			`cannot edit property "about" of type ref`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("version: \"1.0\"\n" + tc.yaml)
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			var got []string
			if err := ValidateConfig(data, &cfg, testMetamodel()); err != nil {
				got = append(got, err.Error())
			}
			errs, warnings := ValidateChannels(&cfg, fakeChannelSchema{})
			got = append(got, errs...)
			if len(warnings) > 0 {
				t.Errorf("warnings with realtime on: %v", warnings)
			}
			all := strings.Join(got, "\n")
			if tc.wantErr == "" {
				if all != "" {
					t.Fatalf("unexpected errors: %s", all)
				}
				return
			}
			if !strings.Contains(all, tc.wantErr) {
				t.Fatalf("errors %q do not contain %q", all, tc.wantErr)
			}
		})
	}
}

// TestValidateChannels_RealtimeOff pins that a `channels:` section with
// realtime off is a warning, not an error, and that no section says
// nothing.
func TestValidateChannels_RealtimeOff(t *testing.T) {
	cfg := &Config{Channels: map[string]ChannelPresentation{"room": {}}}
	errs, warnings := ValidateChannels(cfg, nil)
	if len(errs) != 0 || len(warnings) != 1 || !strings.Contains(warnings[0], "realtime is off") {
		t.Fatalf("errs %v, warnings %v; want one realtime-off warning", errs, warnings)
	}
	errs, warnings = ValidateChannels(&Config{}, nil)
	if errs != nil || warnings != nil {
		t.Fatalf("no section: errs %v, warnings %v", errs, warnings)
	}
}

// TestChannelWidgetTypes_KnownWidgets pins that every channel widget is a
// widget the form widget table knows.
func TestChannelWidgetTypes_KnownWidgets(t *testing.T) {
	for w := range channelWidgetTypes {
		if _, ok := sectionFieldWidgetTypes[w]; !ok {
			t.Errorf("channel widget %q is not a known widget", w)
		}
	}
	if !slices.Contains(channelWidgetTypes[WidgetTextarea], "markdown") {
		t.Error("textarea must edit markdown")
	}
}

// TestChannelsUnknownKeys_Sorted pins that the unknown-key errors come out
// in the order of the channel and message type names, not map order.
func TestChannelsUnknownKeys_Sorted(t *testing.T) {
	data := []byte("channels:\n" +
		"  zed: {x: 1, messages: {b: {y: 1}, a: {y: 1}}}\n" +
		"  alpha: {x: 1}\n" +
		"  mid: {messages: {q: {y: 1}, c: {y: 1}}}\n")
	want := []string{
		`channels.alpha: unknown key "x" (valid keys: messages)`,
		`channels.mid.messages.c: unknown key "y" (valid keys: fields)`,
		`channels.mid.messages.q: unknown key "y" (valid keys: fields)`,
		`channels.zed: unknown key "x" (valid keys: messages)`,
		`channels.zed.messages.a: unknown key "y" (valid keys: fields)`,
		`channels.zed.messages.b: unknown key "y" (valid keys: fields)`,
	}
	for range 20 {
		if got := channelsUnknownKeys(data); !slices.Equal(got, want) {
			t.Fatalf("errors =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}
