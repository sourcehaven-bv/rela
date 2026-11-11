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
	// Session presents the channel type's session header. Nil: the header
	// folds by width, as without the key.
	Session *SessionPresentation `yaml:"session,omitempty" json:"session,omitempty"`
	// Placement says where the channel type sits on an entity page
	// (TKT-VXBE16H). [ChannelPlacementAboveRelations] puts it before the
	// first view section whose source is not the entry. Empty keeps it in
	// the channels block at the end of the page.
	Placement ChannelPlacement `yaml:"placement,omitempty" json:"placement,omitempty"`
}

// ChannelPlacement is where a channel type sits on an entity page; see
// [ChannelPresentation.Placement]. The empty value is the end of the page.
type ChannelPlacement string

// ChannelPlacementAboveRelations places a channel type before the first
// view section whose source is not the entry: after the properties and the
// body of a generated view, before its relation sections.
const ChannelPlacementAboveRelations ChannelPlacement = "above_relations"

// channelPlacementValues lists the valid non-empty values of
// [ChannelPlacement].
var channelPlacementValues = []ChannelPlacement{ChannelPlacementAboveRelations}

// SessionPresentation is how the web app shows a channel type's session
// header (TKT-56TPOTE). [SessionDisplayCompact] folds the header at every
// width: the current state with its previous and next states, and the
// full list behind a toggle. Empty folds it only on a narrow panel.
type SessionPresentation struct {
	Display SessionDisplay `yaml:"display,omitempty" json:"display,omitempty"`
}

// SessionDisplay is how the web app shows a session header; see
// [SessionPresentation]. The empty value folds it by width.
type SessionDisplay string

// SessionDisplayCompact folds the session header at every width.
const SessionDisplayCompact SessionDisplay = "compact"

// sessionDisplayValues lists the valid non-empty values of [SessionDisplay].
var sessionDisplayValues = []SessionDisplay{SessionDisplayCompact}

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
//
// Offer set to false says people do not post the type in the web app
// (TKT-WC63SNN), for a type only agents or integrations post: the
// composer's type picker leaves it out, it gets no automatic session
// decision button, and it does not make Reply show. Nil offers it as
// usual. Its messages show as usual, and a `button:` preset, a compose
// decision, an Other answer or an edit that names the type still opens
// it. This is presentation only, not access control: the type stays in
// `_actions.post` and postable through the API, MCP and the CLI.
type MessagePresentation struct {
	Fields  []FormField    `yaml:"fields,omitempty" json:"fields,omitempty"`
	Display MessageDisplay `yaml:"display,omitempty" json:"display,omitempty"`
	Offer   *bool          `yaml:"offer,omitempty" json:"offer,omitempty"`
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

// displayList joins display values as the error text names them.
func displayList[D ~string](values []D) string {
	names := make([]string, len(values))
	for i, d := range values {
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
	// MessageType reports what the check needs to know of a message type;
	// false for an unknown channel or message type.
	MessageType(channelType, messageType string) (MessageFacts, bool)
	// BusyFacts reports what a busy field or column needs to know of a channel
	// type; false for an unknown type.
	BusyFacts(channelType string) (BusyFacts, bool)
}

// MessageFacts is what [ValidateChannels] checks a message type's
// presentation against.
type MessageFacts struct {
	// Properties lists the properties of the message type.
	Properties []ChannelProperty
	// CountsUnread reports whether the type's messages count toward a
	// reader's unread: a stored type without `unread: false`.
	CountsUnread bool
	// HasPresetButtons reports whether a session transition of the type
	// declares a `button:` preset, which the web app shows to people.
	HasPresetButtons bool
}

// BusyFacts is what [ValidateChannels] checks a `busy:` card field or column
// against, and a session display (`channels.<type>.session.display`) through
// HasSession.
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
	channelPresentationKeys = []string{"messages", "session", "placement"}
	messagePresentationKeys = []string{"fields", "display", "offer"}
	sessionPresentationKeys = []string{"display"}
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
// property twice, a known widget, a span in range, and a known display and
// placement value. data is the raw YAML, needed for the key checks.
func validateChannelsShape(data []byte, cfg *Config) []string {
	errs := channelsUnknownKeys(data)
	for _, ctName := range sortedMapKeys(cfg.Channels) {
		ct := cfg.Channels[ctName]
		if ct.Placement != "" && !slices.Contains(channelPlacementValues, ct.Placement) {
			errs = append(errs, fmt.Sprintf("channels.%s.placement: unknown value %q (valid values: %s)",
				ctName, ct.Placement, displayList(channelPlacementValues)))
		}
		if sp := ct.Session; sp != nil && sp.Display != "" && !slices.Contains(sessionDisplayValues, sp.Display) {
			errs = append(errs, fmt.Sprintf("channels.%s.session.display: unknown value %q (valid values: %s)",
				ctName, sp.Display, displayList(sessionDisplayValues)))
		}
		for _, mtName := range sortedMapKeys(ct.Messages) {
			if d := ct.Messages[mtName].Display; d != "" && !slices.Contains(messageDisplayValues, d) {
				errs = append(errs, fmt.Sprintf("channels.%s.messages.%s.display: unknown value %q (valid values: %s)",
					ctName, mtName, d, displayList(messageDisplayValues)))
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
		session, _ := ct["session"].(map[string]any)
		errs = append(errs, unknownKeysIn(prefix+".session", session, sessionPresentationKeys)...)
		messages, _ := ct["messages"].(map[string]any)
		for _, mtName := range sortedMapKeys(messages) {
			mt := messages[mtName]
			mprefix := prefix + ".messages." + mtName
			m, _ := mt.(map[string]any)
			errs = append(errs, unknownKeysIn(mprefix, m, messagePresentationKeys)...)
			// The decoder reads YAML 1.1 spellings such as `no` or `"off"` into
			// a bool, so only a real true or false is accepted here.
			if v, ok := m["offer"]; ok && v != nil {
				if _, isBool := v.(bool); !isBool {
					errs = append(errs, fmt.Sprintf("%s.offer: %v is not a boolean (use true or false)", mprefix, v))
				}
			}
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
// A kanban `busy:` card field (TKT-21G6JS) or a list busy column
// (TKT-TU3XH4): the channel type must exist, declare a session with a
// working state and attach to the board's or the list's entity type. A
// type whose initial state reads working by default is a warning, because
// every new channel of it would show busy.
//
// With schema nil, realtime is off: a non-empty block, a busy field and a
// busy column are warnings, so a project can stage them before it turns
// realtime on.
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
			warnings = append(warnings, b.at+": realtime is off (no realtime.yaml), so this busy "+b.noun+" shows nothing")
		}
		return nil, warnings
	}
	errs, warnings = checkChannelBlock(cfg, schema)
	for _, b := range busy {
		e, w := busyFieldErrors(b, schema)
		errs, warnings = append(errs, e...), append(warnings, w...)
	}
	return errs, warnings
}

// HasRealtimePresentation reports whether cfg says anything that
// [ValidateChannels] checks against realtime.yaml: a non-empty `channels:`
// block, a kanban card field with `busy:` (TKT-21G6JS) or a list column
// with `busy:` (TKT-TU3XH4).
//
// Nil: cfg is accepted and reports false.
func HasRealtimePresentation(cfg *Config) bool {
	return cfg != nil && (len(cfg.Channels) > 0 || len(busyFields(cfg)) > 0)
}

// busyField is one kanban card field or list column with `busy:`, located
// for messages. noun is "field" or "column", for the realtime-off warning.
type busyField struct {
	at          string
	channelType string
	entityType  string
	noun        string
}

// busyFields lists the busy card fields of every kanban, by kanban id and
// field index, then the busy columns of every list, by list id and column
// index.
func busyFields(cfg *Config) []busyField {
	var out []busyField
	for _, id := range sortedMapKeys(cfg.Kanbans) {
		k := cfg.Kanbans[id]
		for i, f := range k.Card.Fields {
			if f.Busy != "" {
				out = append(out, busyField{
					at: fmt.Sprintf("kanban %q: card.fields[%d]", id, i), channelType: f.Busy, entityType: k.EntityType,
					noun: "field",
				})
			}
		}
	}
	for _, id := range sortedMapKeys(cfg.Lists) {
		l := cfg.Lists[id]
		for i, c := range l.Columns {
			if c.Busy != "" {
				out = append(out, busyField{
					at: fmt.Sprintf("list %q: column[%d]", id, i), channelType: c.Busy, entityType: l.EntityType,
					noun: "column",
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

// checkChannelBlock checks the `channels:` block against schema. A message
// type with `display: hidden` that realtime.yaml still counts as unread is
// a warning, not an error: a project may change the two files one at a
// time. A session display on a channel type without a session is a warning
// for the same reason; it has no effect.
func checkChannelBlock(cfg *Config, schema ChannelSchema) (errs, warnings []string) {
	ctNames := schema.ChannelTypeNames()
	for _, ctName := range sortedMapKeys(cfg.Channels) {
		if !slices.Contains(ctNames, ctName) {
			errs = append(errs, fmt.Sprintf("channels.%s: realtime.yaml declares no channel type %q", ctName, ctName))
			continue
		}
		ct := cfg.Channels[ctName]
		if ct.Session != nil && ct.Session.Display != "" {
			// BusyFacts carries HasSession; the name predates this second use.
			if facts, _ := schema.BusyFacts(ctName); !facts.HasSession {
				warnings = append(warnings, fmt.Sprintf("channels.%s.session.display: channel type %s declares "+
					"no session, so this has no effect", ctName, ctName))
			}
		}
		for _, mtName := range sortedMapKeys(ct.Messages) {
			prefix := fmt.Sprintf("channels.%s.messages.%s", ctName, mtName)
			facts, ok := schema.MessageType(ctName, mtName)
			if !ok {
				errs = append(errs, fmt.Sprintf("%s: channel type %s declares no message type %q", prefix, ctName, mtName))
				continue
			}
			mp := ct.Messages[mtName]
			errs = append(errs, channelFieldErrors(prefix+".fields", mtName, mp.Fields, facts.Properties)...)
			if mp.Display == MessageDisplayHidden && facts.CountsUnread {
				warnings = append(warnings, fmt.Sprintf("%s.display: hidden, but realtime.yaml counts %s as unread; "+
					"set unread: false on channel_types.%s.message_types.%s", prefix, mtName, ctName, mtName))
			}
			if mp.Offer != nil && !*mp.Offer && facts.HasPresetButtons {
				warnings = append(warnings, fmt.Sprintf("%s.offer: false, but realtime.yaml declares a button: preset "+
					"for %s, which the web app still shows to people", prefix, mtName))
			}
		}
	}
	return errs, warnings
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
