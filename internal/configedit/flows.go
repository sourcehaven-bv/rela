package configedit

import (
	"gopkg.in/yaml.v3"
)

// flowsProblems checks an edited flows.yaml beyond the allowlist: written
// is the file the save would write. The `flows:` section must come out as it
// went in, or the save would change the version of flows the editor cannot
// touch. What the triggers read and write is judged against acl.yaml by the
// [TriggerChecker], once the draft validates.
func flowsProblems(cur *current, written []byte) []Problem {
	was, errWas := flowsSection(cur.flows)
	now, errNow := flowsSection(written)
	if errWas != nil || errNow != nil || was != now {
		return []Problem{{
			Code: "unsupported_change", File: FlowsFile, Path: "flows",
			Message: "Saving this would rewrite the flows: section of flows.yaml. Edit flows.yaml directly.",
		}}
	}
	return nil
}

// flowsSection returns the `flows:` section of flows.yaml as the flow loader
// sees it, without comments or positions, so two files can be compared. A
// flow's version hashes the same form, so equal sections keep every flow's
// version.
func flowsSection(data []byte) (string, error) {
	var doc struct {
		Flows yaml.Node `yaml:"flows"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", err
	}
	if doc.Flows.Kind == 0 {
		return "", nil
	}
	out, err := yaml.Marshal(withoutComments(&doc.Flows))
	return string(out), err
}

func withoutComments(n *yaml.Node) *yaml.Node {
	c := *n
	c.HeadComment, c.LineComment, c.FootComment = "", "", ""
	c.Line, c.Column = 0, 0
	if len(n.Content) > 0 {
		c.Content = make([]*yaml.Node, len(n.Content))
		for i, child := range n.Content {
			c.Content[i] = withoutComments(child)
		}
	}
	return &c
}

// holdsNoValue reports whether v is made of empty mappings and lists only.
func holdsNoValue(v any) bool {
	switch t := v.(type) {
	case *Map:
		for _, x := range t.Values {
			if !holdsNoValue(x) {
				return false
			}
		}
		return true
	case []any:
		return len(t) == 0
	}
	return false
}
