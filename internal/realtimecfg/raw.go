package realtimecfg

// The raw* types mirror realtime.yaml key for key. Every value that needs
// parsing (durations, rates, enums) is read as a string and checked by the
// validator, so a decode error is only ever a shape mismatch.
//
// The key checker walks these types by reflection to name unknown keys, so a
// field added here is a key the file accepts.

type rawFile struct {
	Version      int                       `yaml:"version"`
	Limits       *rawLimits                `yaml:"limits"`
	ChannelTypes map[string]rawChannelType `yaml:"channel_types"`
	Triggers     map[string]rawTrigger     `yaml:"triggers"`
}

type rawLimits struct {
	PostRate          string `yaml:"post_rate"`
	SignalRate        string `yaml:"signal_rate"`
	MessageBytes      *int   `yaml:"message_bytes"`
	ChannelsPerEntity *int   `yaml:"channels_per_entity"`
	PageLimit         *int   `yaml:"page_limit"`
}

type rawChannelType struct {
	Label             string                    `yaml:"label"`
	AttachTo          []string                  `yaml:"attach_to"`
	Anchor            string                    `yaml:"anchor"`
	Cardinality       string                    `yaml:"cardinality"`
	Create            *rawCreate                `yaml:"create"`
	Read              *rawRole                  `yaml:"read"`
	Roles             map[string]rawRole        `yaml:"roles"`
	MessageTypes      map[string]rawMessageType `yaml:"message_types"`
	Lifecycle         *rawLifecycle             `yaml:"lifecycle"`
	MessageOps        *rawMessageOps            `yaml:"message_ops"`
	OnEntityDelete    string                    `yaml:"on_entity_delete"`
	DetachedRetention string                    `yaml:"detached_retention"`
	Session           *rawSession               `yaml:"session"`
	Typing            bool                      `yaml:"typing"`
}

// rawSession is `channel_types.<name>.session`. A Go map loses the order
// of states, so [Parse] reads it from the YAML node (see mappingKeys).
type rawSession struct {
	Label   string                     `yaml:"label"`
	Initial string                     `yaml:"initial"`
	States  map[string]rawSessionState `yaml:"states"`
}

type rawSessionState struct {
	Label string                     `yaml:"label"`
	Final bool                       `yaml:"final"`
	On    map[string]rawTransition   `yaml:"on"`
	After map[string]rawSessionTimer `yaml:"after"`
}

// rawSessionTimer is one `after.<name>` entry of a session state.
type rawSessionTimer struct {
	Period string `yaml:"period"`
	Since  string `yaml:"since"`
	To     string `yaml:"to"`
	Label  string `yaml:"label"`
}

type rawTransition struct {
	To    string   `yaml:"to"`
	Roles []string `yaml:"roles"`
	By    string   `yaml:"by"`
	When  string   `yaml:"when"`
}

type rawCreate struct {
	Label string   `yaml:"label"`
	Roles []string `yaml:"roles"`
}

type rawRole struct {
	Label         string `yaml:"label"`
	From          string `yaml:"from"`
	Permission    string `yaml:"permission"`
	PrincipalType string `yaml:"principal_type"`
	Creator       bool   `yaml:"creator"`
}

type rawMessageType struct {
	Label      string                 `yaml:"label"`
	Persist    string                 `yaml:"persist"`
	TTL        string                 `yaml:"ttl"`
	Rate       string                 `yaml:"rate"`
	Properties map[string]rawProperty `yaml:"properties"`
	When       string                 `yaml:"when"`
	Post       []string               `yaml:"post"`
	PromoteTo  *rawPromote            `yaml:"promote_to"`
	Actions    map[string]rawAction   `yaml:"actions"`
}

type rawProperty struct {
	Label    string   `yaml:"label"`
	Type     string   `yaml:"type"`
	Required bool     `yaml:"required"`
	Max      *int     `yaml:"max"`
	To       []string `yaml:"to"`
}

type rawPromote struct {
	Type   string            `yaml:"type"`
	Face   string            `yaml:"face"`
	Map    map[string]string `yaml:"map"`
	Relate *rawRelate        `yaml:"relate"`
}

type rawRelate struct {
	Type string `yaml:"type"`
}

type rawAction struct {
	Label      string      `yaml:"label"`
	Permission string      `yaml:"permission"`
	Roles      []string    `yaml:"roles"`
	Denied     string      `yaml:"denied"`
	PromoteTo  *rawPromote `yaml:"promote_to"`
}

type rawLifecycle struct {
	Close   *rawLifecycleOp `yaml:"close"`
	Reopen  *rawLifecycleOp `yaml:"reopen"`
	Archive *rawLifecycleOp `yaml:"archive"`
	Delete  *rawLifecycleOp `yaml:"delete"`
}

type rawLifecycleOp struct {
	Label      string   `yaml:"label"`
	Roles      []string `yaml:"roles"`
	Permission string   `yaml:"permission"`
	Condition  string   `yaml:"condition"`
	Denied     string   `yaml:"denied"`
	From       string   `yaml:"from"`
	Auto       *rawAuto `yaml:"auto"`
	Retention  string   `yaml:"retention"`
}

type rawAuto struct {
	When  string `yaml:"when"`
	After string `yaml:"after"`
}

type rawMessageOps struct {
	EditOwn   *rawEditOwn   `yaml:"edit_own"`
	DeleteOwn *rawDeleteOwn `yaml:"delete_own"`
	Remove    *rawRemove    `yaml:"remove"`
}

type rawEditOwn struct {
	Roles  []string `yaml:"roles"`
	Within string   `yaml:"within"`
}

type rawDeleteOwn struct {
	Roles []string `yaml:"roles"`
	From  string   `yaml:"from"`
}

type rawRemove struct {
	Roles          []string `yaml:"roles"`
	ReasonRequired bool     `yaml:"reason_required"`
}

type rawTrigger struct {
	EntityType string         `yaml:"entity_type"`
	On         string         `yaml:"on"`
	When       string         `yaml:"when"`
	Channel    string         `yaml:"channel"`
	WhenClosed string         `yaml:"when_closed"`
	Post       rawTriggerPost `yaml:"post"`
}

type rawTriggerPost struct {
	Type  string            `yaml:"type"`
	Props map[string]string `yaml:"props"`
}
