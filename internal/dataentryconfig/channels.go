package dataentryconfig

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChannelPresentation is one entry of the `channels:` block of
// data-entry.yaml, keyed by channel type: how the SPA presents the message
// types realtime.yaml declares. realtime.yaml stays structural, as
// schema.yaml does for entities; this block holds the field order, labels
// and widgets, as `forms:` does.
type ChannelPresentation struct {
	// Messages are keyed by message type.
	Messages map[string]MessagePresentation `yaml:"messages,omitempty" json:"messages,omitempty"`
}

// MessagePresentation orders and labels the properties of one message
// type. Fields is the composer order and the display order under a
// message. Properties it does not list follow, required first, then by
// name; a listed field with `hidden: true` is left out.
//
// A field takes the [FormField] keys property, label, placeholder, help,
// widget, span and hidden. The keys a channel composer has no behavior
// for are refused at load, see channelFieldRefusedKeys.
type MessagePresentation struct {
	Fields []FormField `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// ChannelProperty is a message property as the channel presentation check
// needs it. Type is the realtime.yaml property type: string, markdown,
// integer, boolean, date, datetime, ref or enum.
type ChannelProperty struct {
	Name     string
	Type     string
	Required bool
}

// ChannelSchema is what [ValidateChannels] needs to know of realtime.yaml.
// dataentryconfig does not read realtime.yaml itself; the wiring site
// supplies this from the compiled configuration.
type ChannelSchema interface {
	ChannelTypeNames() []string
	MessageTypeNames(channelType string) []string
	// MessageProperties lists the properties of a message type; nil for an
	// unknown channel or message type.
	MessageProperties(channelType, messageType string) []ChannelProperty
}

var (
	channelPresentationKeys = []string{"messages"}
	messagePresentationKeys = []string{"fields"}
	channelFieldKeys        = []string{"property", "label", "placeholder", "help", "widget", "span", "hidden"}
)

// channelFieldRefusedKeys are [FormField] keys a channel field does not
// take. `required` comes from realtime.yaml; the others name form
// behavior the composer does not have, so accepting them would promise
// something that never happens.
var channelFieldRefusedKeys = []string{
	"required", "required_when", "visible_when", "transitions", "default",
	"keep_on_add_another", "clear_when_hidden",
}

// channelWidgetTypes lists, per widget, the realtime.yaml property types it
// can edit. A ref property takes no widget: the composer always renders an
// entity picker for it.
var channelWidgetTypes = map[string][]string{
	WidgetText:     {"string"},
	WidgetTextarea: {"string", "markdown"},
	WidgetNumber:   {"integer"},
	WidgetCheckbox: {"boolean"},
	WidgetDate:     {"date"},
	WidgetDatetime: {"datetime"},
	WidgetSelect:   {"enum"},
}

// validateChannelsShape checks what the `channels:` block can be checked
// for without realtime.yaml: known keys, a property on every field, no
// property twice, a known widget and a span in range. data is the raw
// YAML, needed for the key checks.
func validateChannelsShape(data []byte, cfg *Config) []string {
	errs := channelsUnknownKeys(data)
	for _, ctName := range sortedMapKeys(cfg.Channels) {
		ct := cfg.Channels[ctName]
		for _, mtName := range sortedMapKeys(ct.Messages) {
			prefix := fmt.Sprintf("channels.%s.messages.%s.fields", ctName, mtName)
			seen := map[string]bool{}
			for i, f := range ct.Messages[mtName].Fields {
				at := fmt.Sprintf("%s[%d]", prefix, i)
				switch {
				case f.Property == "":
					errs = append(errs, at+": property is required")
				case seen[f.Property]:
					errs = append(errs, fmt.Sprintf("%s: property %q is listed twice", at, f.Property))
				}
				seen[f.Property] = true
				if _, ok := channelWidgetTypes[f.Widget]; f.Widget != "" && !ok {
					errs = append(errs, fmt.Sprintf("%s: widget %q cannot be used in a channel (valid widgets: %s)",
						at, f.Widget, strings.Join(sortedMapKeys(channelWidgetTypes), ", ")))
				}
				errs = append(errs, validateSpan(f.Span, at)...)
			}
		}
	}
	return errs
}

// channelsUnknownKeys reports keys the `channels:` block does not define,
// at every level. The decoder ignores unknown nested keys, so a typo such
// as `feilds:` would silently present nothing.
func channelsUnknownKeys(data []byte) []string {
	var raw struct {
		Channels map[string]map[string]any `yaml:"channels"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // the struct unmarshal reports malformed YAML
	}
	var errs []string
	for _, ctName := range sortedMapKeys(raw.Channels) {
		ct := raw.Channels[ctName]
		prefix := "channels." + ctName
		errs = append(errs, unknownKeysIn(prefix, ct, channelPresentationKeys)...)
		messages, _ := ct["messages"].(map[string]any)
		for _, mtName := range sortedMapKeys(messages) {
			mt := messages[mtName]
			mprefix := prefix + ".messages." + mtName
			m, _ := mt.(map[string]any)
			errs = append(errs, unknownKeysIn(mprefix, m, messagePresentationKeys)...)
			fields, _ := m["fields"].([]any)
			for i, f := range fields {
				fm, _ := f.(map[string]any)
				errs = append(errs, channelFieldKeyErrors(fmt.Sprintf("%s.fields[%d]", mprefix, i), fm)...)
			}
		}
	}
	return errs
}

func channelFieldKeyErrors(prefix string, field map[string]any) []string {
	var errs []string
	for key := range field {
		switch {
		case slices.Contains(channelFieldKeys, key):
		case slices.Contains(channelFieldRefusedKeys, key):
			errs = append(errs, fmt.Sprintf("%s: %q is not supported in a channel field (valid keys: %s)",
				prefix, key, strings.Join(channelFieldKeys, ", ")))
		default:
			errs = append(errs, fmt.Sprintf("%s: unknown key %q (valid keys: %s)",
				prefix, key, strings.Join(channelFieldKeys, ", ")))
		}
	}
	sort.Strings(errs)
	return errs
}

// ValidateChannels checks the `channels:` block against realtime.yaml:
// every channel type, message type and property must exist, a widget must
// suit its property's type, and a required property must not be hidden,
// because the composer could never send it. With schema nil, realtime is
// off: a non-empty block is a warning, so a project can stage it before it
// turns realtime on.
func ValidateChannels(cfg *Config, schema ChannelSchema) (errs, warnings []string) {
	if cfg == nil || len(cfg.Channels) == 0 {
		return nil, nil
	}
	if schema == nil {
		return nil, []string{"channels: realtime is off (no realtime.yaml), so this section has no effect"}
	}
	ctNames := schema.ChannelTypeNames()
	for _, ctName := range sortedMapKeys(cfg.Channels) {
		if !slices.Contains(ctNames, ctName) {
			errs = append(errs, fmt.Sprintf("channels.%s: realtime.yaml declares no channel type %q", ctName, ctName))
			continue
		}
		mtNames := schema.MessageTypeNames(ctName)
		ct := cfg.Channels[ctName]
		for _, mtName := range sortedMapKeys(ct.Messages) {
			prefix := fmt.Sprintf("channels.%s.messages.%s", ctName, mtName)
			if !slices.Contains(mtNames, mtName) {
				errs = append(errs, fmt.Sprintf("%s: channel type %s declares no message type %q", prefix, ctName, mtName))
				continue
			}
			props := schema.MessageProperties(ctName, mtName)
			errs = append(errs, channelFieldErrors(prefix+".fields", mtName, ct.Messages[mtName].Fields, props)...)
		}
	}
	return errs, nil
}

func channelFieldErrors(prefix, mtName string, fields []FormField, props []ChannelProperty) []string {
	var errs []string
	for i, f := range fields {
		at := fmt.Sprintf("%s[%d]", prefix, i)
		idx := slices.IndexFunc(props, func(p ChannelProperty) bool { return p.Name == f.Property })
		if idx < 0 {
			if f.Property != "" {
				errs = append(errs, fmt.Sprintf("%s: message type %s declares no property %q", at, mtName, f.Property))
			}
			continue
		}
		p := props[idx]
		if f.Hidden && p.Required {
			errs = append(errs, fmt.Sprintf(
				"%s: property %q is required, so it cannot be hidden: the composer could never send it", at, p.Name))
		}
		if f.Widget == "" {
			continue
		}
		if types, ok := channelWidgetTypes[f.Widget]; ok && !slices.Contains(types, p.Type) {
			errs = append(errs, fmt.Sprintf("%s: widget %q cannot edit property %q of type %s",
				at, f.Widget, p.Name, p.Type))
		}
	}
	return errs
}
