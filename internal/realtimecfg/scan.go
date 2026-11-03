package realtimecfg

import (
	"errors"
	"slices"

	"gopkg.in/yaml.v3"
)

// ScanPermissions returns every value of a `permission:` key in the
// contents of realtime.yaml, sorted and without duplicates. It does not
// validate the file, so `rela acl audit` still counts the permissions of a
// realtime.yaml that refers to a type the schema dropped. It errs only when
// the YAML does not parse, or expands beyond the node bound [Parse] uses.
//
// It may over-count, for example a `permission:` under a misspelled key.
// That is the safe direction for the audit: a permission counted as used is
// at worst not reported dead, while one missed would be reported dead and
// might be deleted.
func ScanPermissions(data []byte) ([]string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	s := &permissionScan{seen: map[string]bool{}}
	s.walk(&root)
	if s.visits > maxKeyNodes {
		return nil, errors.New("realtime.yaml expands to too many nodes")
	}
	out := make([]string, 0, len(s.seen))
	for p := range s.seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out, nil
}

type permissionScan struct {
	seen   map[string]bool
	visits int
}

func (s *permissionScan) walk(n *yaml.Node) {
	if n == nil || s.visits > maxKeyNodes {
		return
	}
	s.visits++
	switch n.Kind {
	case yaml.AliasNode:
		s.walk(n.Alias)
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Value == "permission" && v.Kind == yaml.ScalarNode && v.Value != "" {
				s.seen[v.Value] = true
			}
			s.walk(v)
		}
	default:
		for _, c := range n.Content {
			s.walk(c)
		}
	}
}
