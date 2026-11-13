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
	// OutsideEdits is `outside_edits:`, compiled by [loader.outsideEdits].
	OutsideEdits yaml.Node `yaml:"outside_edits"`
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
	Writes     []string             `yaml:"writes"`
	Outputs    []rawOutput          `yaml:"outputs"`
	On         map[string]yaml.Node `yaml:"on"`
	After      yaml.Node            `yaml:"after"`
	// Join is `join:`: all, any, or a mapping with `when:`.
	Join yaml.Node `yaml:"join"`
	// Job is `job:`, compiled by [loader.job].
	Job yaml.Node `yaml:"job"`
}

// rawBranch is one entry of `on.<event>`, which is one mapping or a list
// of them.
type rawBranch struct {
	To    rawTargets `yaml:"to"`
	Roles []string   `yaml:"roles"`
	When  string     `yaml:"when"`
	Label string     `yaml:"label"`
}

// rawTargets is a branch's `to:`: one state, or a list of states for a
// fork. List reports the list form, so `to: [a]` is refused as a fork of
// one state rather than read as `to: a`.
type rawTargets struct {
	States []string
	List   bool
}

// UnmarshalYAML reads a scalar or a sequence of scalars.
func (t *rawTargets) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.SequenceNode {
		t.List = true
		return n.Decode(&t.States)
	}
	var one string
	if err := n.Decode(&one); err != nil {
		return err
	}
	if one != "" {
		t.States = []string{one}
	}
	return nil
}

// rawJoin is the mapping form of `join:`.
type rawJoin struct {
	When string `yaml:"when"`
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
