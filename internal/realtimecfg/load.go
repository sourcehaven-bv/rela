// Package realtimecfg loads realtime.yaml: the channel types, message types,
// lifecycle and triggers of rela's realtime channels.
//
// The load path parses strictly (an unknown key fails), checks every
// reference against schema.yaml and acl.yaml, compiles every predicate, and
// returns an immutable [*Compiled]. A file that fails any check does not
// load; [*Error] lists each problem by key path.
//
// The file is optional. A project without it has a nil *Compiled, and
// realtime is off.
//
// schema.yaml never refers to this file, and the metamodel package must not
// import this one (arch-lint): realtime is a consumer of the schema, not
// part of it.
package realtimecfg

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// FileName is the project-root file this package loads.
const FileName = "realtime.yaml"

// Problem is one reason realtime.yaml does not load. Key is the key path,
// such as "channel_types.incident-room.attach_to[0]"; it is empty for a
// problem with the file as a whole.
type Problem struct {
	Key    string
	Reason string
}

func (p Problem) String() string {
	if p.Key == "" {
		return p.Reason
	}
	return p.Key + ": " + p.Reason
}

// Error lists every problem found in realtime.yaml. Configuration is not a
// secret, so the message names keys and values freely; it never carries a
// host path.
type Error struct {
	Problems []Problem
}

func (e *Error) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.String()
	}
	return FileName + ": " + strings.Join(parts, "; ")
}

// Parse compiles the contents of realtime.yaml against meta and policy.
//
// Nil: meta is rejected. policy is accepted and means the project has no
// acl.yaml; the checks that read the policy then have nothing to refuse.
func Parse(data []byte, meta *metamodel.Metamodel, policy *acl.Policy) (*Compiled, error) {
	if meta == nil {
		return nil, errors.New("realtimecfg: Parse: metamodel is required")
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, &Error{Problems: []Problem{{Reason: "invalid YAML: " + err.Error()}}}
	}
	var problems []Problem
	checkKeys(&root, reflect.TypeFor[rawFile](), "", func(key, reason string) {
		problems = append(problems, Problem{Key: key, Reason: reason})
	})
	if len(problems) > 0 {
		return nil, &Error{Problems: problems}
	}
	var raw rawFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
		return nil, &Error{Problems: []Problem{{Reason: err.Error()}}}
	}
	v := &validator{meta: meta, policy: policy, root: &root}
	c := v.compile(&raw)
	if len(v.problems) > 0 {
		sort.SliceStable(v.problems, func(i, j int) bool { return v.problems[i].Key < v.problems[j].Key })
		return nil, &Error{Problems: v.problems}
	}
	return c, nil
}
