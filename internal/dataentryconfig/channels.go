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
//
// Display tones the message type down in the web app (TKT-03DMXH):
// [MessageDisplayCollapsed] folds a run of such messages into one line,
// [MessageDisplayHidden] leaves them out until the reader shows all
// messages. Empty shows them in full. This is presentation only: the
// messages stay readable through the API, MCP and the CLI.
type MessagePresentation struct {
	Fields  []FormField    `yaml:"fields,omitempty" json:"fields,omitempty"`
	Display MessageDisplay `yaml:"display,omitempty" json:"display,omitempty"`
}

// MessageDisplay is how the web app shows the messages of one type; see
// [MessagePresentation]. The empty value shows them in full.
type MessageDisplay string

// The values of [MessageDisplay].
const (
	MessageDisplayCollapsed MessageDisplay = "collapsed"
	MessageDisplayHidden    MessageDisplay = "hidden"
)

// messageDisplayValues lists the valid non-empty values of [MessageDisplay].
var messageDisplayValues = []MessageDisplay{MessageDisplayCollapsed, MessageDisplayHidden}

// messageDisplayList is messageDisplayValues as the error text names them.
func messageDisplayList() string {
	names := make([]string, len(messageDisplayValues))
	for i, d := range messageDisplayValues {
		names[i] = string(d)
	}
	return strings.Join(names, ", ")
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
	// BusyFacts reports what a kanban busy field needs to know of a channel
	// type; false for an unknown type.
	BusyFacts(channelType string) (BusyFacts, bool)
}

// BusyFacts is what [ValidateChannels] checks a kanban `busy:` card field
// against.
type BusyFacts struct {
	// AttachTo lists the entity types the channel type attaches to.
	AttachTo []string
	// HasSession reports whether the channel type declares a session.
	HasSession bool
	// HasWorkingState reports whether the session has a state whose status
	// reads working, declared or by default. Without one no channel of the
	// type is ever busy.
	HasWorkingState bool
	// InitialDefaultWorking reports whether the session's initial state
	// reads working by default: it is not final, has no waiting_for and
	// sets no status. Every new channel of the type then shows busy.
	InitialDefaultWorking bool
}

var (
	channelPresentationKeys = []string{"messages"}
	messagePresentationKeys = []string{"fields", "display"}
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
// property twice, a known widget, a span in range and a known display
// value. data is the raw YAML, needed for the key checks.
func validateChannelsShape(data []byte, cfg *Config) []string {
	errs := channelsUnknownKeys(data)
	for _, ctName := range sortedMapKeys(cfg.Channels) {
		ct := cfg.Channels[ctName]
		for _, mtName := range sortedMapKeys(ct.Messages) {
			if d := ct.Messages[mtName].Display; d != "" && !slices.Contains(messageDisplayValues, d) {
				errs = append(errs, fmt.Sprintf("channels.%s.messages.%s.display: unknown value %q (valid values: %s)",
					ctName, mtName, d, messageDisplayList()))
			}
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

// ValidateChannels checks what data-entry.yaml says about realtime.yaml.
//
// The `channels:` block: every channel type, message type and property must
// exist, a widget must suit its property's type, and a required property
// must not be hidden, because the composer could never send it.
//
// A kanban `busy:` card field (TKT-21G6JS): the channel type must exist,
// declare a session with a working state and attach to the board's entity
// type. A type whose
// initial state reads working by default is a warning, because every new
// channel of it would show busy.
//
// With schema nil, realtime is off: a non-empty block and a busy field are
// warnings, so a project can stage them before it turns realtime on.
func ValidateChannels(cfg *Config, schema ChannelSchema) (errs, warnings []string) {
	if !HasRealtimePresentation(cfg) {
		return nil, nil
	}
	busy := busyFields(cfg)
	if schema == nil {
		if len(cfg.Channels) > 0 {
			warnings = append(warnings, "channels: realtime is off (no realtime.yaml), so this section has no effect")
		}
		for _, b := range busy {
			warnings = append(warnings, b.at+": realtime is off (no realtime.yaml), so this busy field shows nothing")
		}
		return nil, warnings
	}
	errs = channelBlockErrors(cfg, schema)
	for _, b := range busy {
		e, w := busyFieldErrors(b, schema)
		errs, warnings = append(errs, e...), append(warnings, w...)
	}
	return errs, warnings
}

// HasRealtimePresentation reports whether cfg says anything that
// [ValidateChannels] checks against realtime.yaml: a non-empty `channels:`
// block or a kanban card field with `busy:` (TKT-21G6JS).
//
// Nil: cfg is accepted and reports false.
func HasRealtimePresentation(cfg *Config) bool {
	return cfg != nil && (len(cfg.Channels) > 0 || len(busyFields(cfg)) > 0)
}

// busyField is one kanban card field with `busy:`, located for messages.
type busyField struct {
	at          string
	channelType string
	entityType  string
}

// busyFields lists the busy card fields of every kanban, by kanban id and
// field index.
func busyFields(cfg *Config) []busyField {
	var out []busyField
	for _, id := range sortedMapKeys(cfg.Kanbans) {
		k := cfg.Kanbans[id]
		for i, f := range k.Card.Fields {
			if f.Busy != "" {
				out = append(out, busyField{
					at: fmt.Sprintf("kanban %q: card.fields[%d]", id, i), channelType: f.Busy, entityType: k.EntityType,
				})
			}
		}
	}
	return out
}

func busyFieldErrors(b busyField, schema ChannelSchema) (errs, warnings []string) {
	facts, ok := schema.BusyFacts(b.channelType)
	switch {
	case !ok:
		return []string{fmt.Sprintf("%s: realtime.yaml declares no channel type %q", b.at, b.channelType)}, nil
	case !facts.HasSession:
		return []string{fmt.Sprintf("%s: channel type %s declares no session, so it is never busy",
			b.at, b.channelType)}, nil
	case !facts.HasWorkingState:
		return []string{fmt.Sprintf("%s: the session of channel type %s has no state with status working, "+
			"so it is never busy", b.at, b.channelType)}, nil
	case !slices.Contains(facts.AttachTo, b.entityType):
		return []string{fmt.Sprintf("%s: channel type %s does not attach to entity type %q",
			b.at, b.channelType, b.entityType)}, nil
	case facts.InitialDefaultWorking:
		return nil, []string{fmt.Sprintf("%s: the initial state of channel type %s reads working by default, "+
			"so every new channel of it shows busy; give that state waiting_for or an explicit status",
			b.at, b.channelType)}
	}
	return nil, nil
}

// channelBlockErrors checks the `channels:` block against schema.
func channelBlockErrors(cfg *Config, schema ChannelSchema) []string {
	var errs []string
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
	return errs
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
