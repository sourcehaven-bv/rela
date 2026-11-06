package flowcfg

import "gopkg.in/yaml.v3"

// rawFile is flows.yaml as written. Every struct is decoded with known
// fields only, so a misspelled key is a load error rather than a silent
// default.
type rawFile struct {
	Flows yaml.Node `yaml:"flows"`
}

// rawFlow is one entry of `flows:`.
type rawFlow struct {
	Label      string        `yaml:"label"`
	Subject    string        `yaml:"subject"`
	RunAs      *string       `yaml:"run_as"`
	PerSubject string        `yaml:"per_subject"`
	Deadline   string        `yaml:"deadline"`
	Retention  *rawRetention `yaml:"retention"`
	Start      []rawStart    `yaml:"start"`
	Initial    string        `yaml:"initial"`
	States     yaml.Node     `yaml:"states"`
}

// rawRetention is `retention:`.
type rawRetention struct {
	After  string `yaml:"after"`
	Action string `yaml:"action"`
}

// rawStart is one entry of `start:`.
type rawStart struct {
	On         string `yaml:"on"`
	When       string `yaml:"when"`
	Permission string `yaml:"permission"`
	Every      string `yaml:"every"`
}

// rawState is one entry of `states:`.
type rawState struct {
	Label      string               `yaml:"label"`
	Final      bool                 `yaml:"final"`
	Status     string               `yaml:"status"`
	WaitingFor []string             `yaml:"waiting_for"`
	Outputs    []rawOutput          `yaml:"outputs"`
	On         map[string]yaml.Node `yaml:"on"`
	After      yaml.Node            `yaml:"after"`
}

// rawBranch is one entry of `on.<event>`, which is one mapping or a list
// of them.
type rawBranch struct {
	To    string   `yaml:"to"`
	Roles []string `yaml:"roles"`
	When  string   `yaml:"when"`
	Label string   `yaml:"label"`
}

// rawTimer is one entry of `after:`.
type rawTimer struct {
	Period string `yaml:"period"`
	To     string `yaml:"to"`
	Since  string `yaml:"since"`
	Label  string `yaml:"label"`
}

// rawOutput is one entry of a state's `outputs:`.
type rawOutput struct {
	Set    *rawSet    `yaml:"set"`
	Create *rawCreate `yaml:"create"`
}

// rawSet is a `set:` output.
type rawSet struct {
	Properties map[string]yaml.Node `yaml:"properties"`
}

// rawCreate is a `create:` output.
type rawCreate struct {
	Type       string               `yaml:"type"`
	Properties map[string]yaml.Node `yaml:"properties"`
	Relate     *rawRelate           `yaml:"relate"`
}

// rawRelate is the `relate:` of a create output.
type rawRelate struct {
	Type string `yaml:"type"`
}
