package projectsetup

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// The move of schema `automations:` into flows.yaml `triggers:`
// (TKT-GWOBH2U). The conversion works on yaml.Node so comments travel with
// the moved entries; the strict decode into the legacy structs below only
// validates. Every refusal happens before the first write.

// flowsFile is flows.yaml. flowcfg owns the name, but projectsetup may not
// import flowcfg (.go-arch-lint.yml).
const flowsFile = "flows.yaml"

// The legacy automation shape, copied from the schema types it was declared
// in so the strict decode keeps working after those are deleted.
type legacyAutomation struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	On          legacyTrigger  `yaml:"on"`
	Do          []legacyAction `yaml:"do"`
	Validate    []legacyCheck  `yaml:"validate"`
}

type legacyTrigger struct {
	Entity          legacyStrings `yaml:"entity"`
	Property        string        `yaml:"property"`
	Becomes         string        `yaml:"becomes"`
	From            string        `yaml:"from"`
	Created         bool          `yaml:"created"`
	Updated         bool          `yaml:"updated"`
	RelationCreated string        `yaml:"relation_created"`
	RelationRemoved string        `yaml:"relation_removed"`
	Deleted         bool          `yaml:"deleted"`
	Faces           legacyStrings `yaml:"faces"`
	When            []string      `yaml:"when"`
	Condition       string        `yaml:"condition"`
	Copy            legacyStrings `yaml:"copy"`
}

type legacyAction struct {
	Set            string                `yaml:"set"`
	Value          string                `yaml:"value"`
	CreateRelation *legacyCreateRelation `yaml:"create_relation"`
	CreateEntity   *legacyCreateEntity   `yaml:"create_entity"`
	AddToPile      *legacyAddToPile      `yaml:"add_to_pile"`
	Notify         *legacyNotify         `yaml:"notify"`
	Lua            string                `yaml:"lua"`
	LuaFile        string                `yaml:"lua_file"`
	AllowACLBypass string                `yaml:"allow_acl_bypass"`
	Capabilities   *legacyCapabilities   `yaml:"capabilities"`
	Background     bool                  `yaml:"background"`
	RunAs          string                `yaml:"run_as"`
	Retry          string                `yaml:"retry"`
}

type legacyCreateRelation struct {
	Relation string `yaml:"relation"`
	To       string `yaml:"to"`
}

type legacyCreateEntity struct {
	Type       string            `yaml:"type"`
	Template   string            `yaml:"template"`
	Properties map[string]string `yaml:"properties"`
	Relation   string            `yaml:"relation"`
	IfExists   string            `yaml:"if_exists"`
}

type legacyAddToPile struct {
	Pile   string `yaml:"pile"`
	Owner  string `yaml:"owner"`
	Create *bool  `yaml:"create"`
}

type legacyNotify struct {
	Pile   string             `yaml:"pile"`
	To     legacyNotifyTarget `yaml:"to"`
	Reason string             `yaml:"reason"`
}

type legacyNotifyTarget struct {
	Via      legacyStrings `yaml:"via"`
	Type     string        `yaml:"type"`
	Property string        `yaml:"property"`
}

type legacyCapabilities struct {
	HTTP      bool     `yaml:"http"`
	AI        bool     `yaml:"ai"`
	Mail      bool     `yaml:"mail"`
	WriteFile bool     `yaml:"write_file"`
	Secrets   []string `yaml:"secrets"`
	Tokens    []string `yaml:"tokens"`
}

type legacyCheck struct {
	Check    string `yaml:"check"`
	Severity string `yaml:"severity"`
	Message  string `yaml:"message"`
}

// legacyStrings is a string or a list of strings.
type legacyStrings []string

// UnmarshalYAML accepts a scalar or a sequence of scalars.
func (s *legacyStrings) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*s = []string{n.Value}
		return nil
	}
	var list []string
	if err := n.Decode(&list); err != nil {
		return err
	}
	*s = list
	return nil
}

// legacySchemaFile is the strict-decode target for a whole schema file: the
// automations are checked field by field, every other top-level key is
// accepted as is. Decoding the file rather than each entry keeps the line
// numbers in an error pointing at the operator's file.
type legacySchemaFile struct {
	Automations []legacyAutomation `yaml:"automations"`
	Rest        map[string]any     `yaml:",inline"`
}

// automationSource is one schema file that declares automations.
type automationSource struct {
	path    string // filesystem path
	display string // name used in messages
	data    []byte
	root    *yaml.Node // top-level mapping
	keyIdx  int        // index of the automations key in root.Content
	entries []*yaml.Node
	decoded []legacyAutomation
}

// convertedTrigger is one automation in its flows.yaml form.
type convertedTrigger struct {
	name   string
	key    *yaml.Node
	value  *yaml.Node
	source string // file display name
}

// convertAutomationsYAML returns a flows.yaml document holding only
// `triggers:`, converted from the `automations:` of the given schema. It
// ignores includes. A schema without automations yields `triggers: {}`.
// Only tests call it, through export_test.go.
func convertAutomationsYAML(schema []byte) ([]byte, error) {
	src, found, err := parseAutomationSource("schema.yaml", "schema.yaml", schema)
	if err != nil {
		return nil, err
	}
	var sources []*automationSource
	if found {
		sources = append(sources, src)
	}
	triggers, _, err := convertSources(sources, nil)
	if err != nil {
		return nil, err
	}
	entries := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, t := range triggers {
		entries.Content = append(entries.Content, t.key, t.value)
	}
	if len(entries.Content) == 0 {
		entries.Style = yaml.FlowStyle
	}
	doc := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar("triggers"), entries}}
	return encodeNode(doc)
}

// pendingAutomationMoves reports the include files that still declare
// automations. The root schema is reported by the registered
// automations-to-flows detection; includes are not migrated per file, so
// they are listed here. Read-only.
func pendingAutomationMoves(ctx *project.Context, fs storage.FS) []MigrateDetection {
	var out []MigrateDetection
	desc := (&migration.AutomationsToFlowsMigration{}).Description()
	for _, inc := range collectIncludes(ctx, fs) {
		if !inc.hasAutomations {
			continue
		}
		out = append(out, MigrateDetection{
			File: MigrateFile{Path: inc.path, Name: inc.display, FileType: migration.FileTypeMetamodel},
			Migrations: []migration.DetectionResult{{
				Migration:   &migration.AutomationsToFlowsMigration{},
				Description: desc,
			}},
		})
	}
	return out
}

// moveAutomationsToFlows moves every schema automation (root first, then
// includes in include order) to flows.yaml `triggers:` and cuts the
// `automations:` blocks from the schema files. It returns the number of
// automations cut from the schema, including those a crashed earlier run
// already wrote to flows.yaml.
func moveAutomationsToFlows(ctx *project.Context, fs storage.FS) (int, error) {
	includes := collectIncludes(ctx, fs)
	if err := prepareIncludes(ctx, fs, includes); err != nil {
		return 0, err
	}
	sources, err := readAutomationSources(ctx, fs, includes)
	if err != nil || len(sources) == 0 {
		return 0, err
	}

	flowsPath := filepath.Join(ctx.Root, flowsFile)
	flowsData, err := fs.ReadFile(flowsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("reading %s: %w", flowsFile, err)
	}
	flows, err := parseFlows(flowsData)
	if err != nil {
		return 0, err
	}

	triggers, recovered, err := convertSources(sources, flows.existing)
	if err != nil {
		return 0, err
	}

	if len(triggers) > 0 {
		out, err := flows.withTriggers(flowsData, triggers)
		if err != nil {
			return 0, err
		}
		if err := writeFile(fs, flowsPath, out); err != nil {
			return 0, fmt.Errorf("writing %s: %w", flowsFile, err)
		}
	}
	// Cut the last file first and the root last. An unnamed automation is
	// named after its position across all files, so the files a crash
	// leaves uncut must stay a prefix of the list. Then a re-run gives them
	// the same names, finds the equal triggers in flows.yaml and only cuts.
	for i := len(sources) - 1; i >= 0; i-- {
		src := sources[i]
		if err := writeFile(fs, src.path, src.cut()); err != nil {
			return 0, fmt.Errorf("writing %s: %w", src.display, err)
		}
	}
	return len(triggers) + recovered, nil
}

// prepareIncludes refuses an include the move may not rewrite, then
// rewrites old automation syntax (acl-bypass-enum and friends) in the
// includes that declare automations, as the per-file loop did for the root.
func prepareIncludes(ctx *project.Context, fs storage.FS, includes []includeFile) error {
	for _, inc := range includes {
		if !inc.hasAutomations {
			continue
		}
		if !inc.inProject {
			return fmt.Errorf("%s: declares automations: but lies outside the project directory, "+
				"so 'rela migrate' will not rewrite it (it may be shared); move its automations "+
				"to %s under triggers: by hand", inc.display, flowsFile)
		}
		if inc.namespaced {
			return fmt.Errorf("%s: declares automations: in a file with namespace:, whose qualified "+
				"names are not valid trigger names; move its automations to %s under triggers: by hand "+
				"and rename them", inc.display, flowsFile)
		}
	}

	mm := loadMigrationMetamodel(ctx, fs)
	for _, inc := range includes {
		if !inc.hasAutomations {
			continue
		}
		if _, err := migration.ApplyWithMetamodel(inc.path, migration.FileTypeMetamodel, fs, mm); err != nil {
			return fmt.Errorf("migrating %s: %w", inc.display, err)
		}
	}
	return nil
}

// readAutomationSources parses the root schema and every include that
// declares automations, in that order, and returns those that have some.
func readAutomationSources(ctx *project.Context, fs storage.FS, includes []includeFile) ([]*automationSource, error) {
	files := []struct{ path, display string }{{ctx.SchemaPath, filepath.Base(ctx.SchemaPath)}}
	for _, inc := range includes {
		if inc.hasAutomations {
			files = append(files, struct{ path, display string }{inc.path, inc.display})
		}
	}
	var sources []*automationSource
	for _, f := range files {
		data, err := fs.ReadFile(f.path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", f.display, err)
		}
		src, found, err := parseAutomationSource(f.path, f.display, data)
		if err != nil {
			return nil, err
		}
		if found {
			sources = append(sources, src)
		}
	}
	return sources, nil
}

// writeFile writes through the filesystem, which is atomic for SafeFS.
func writeFile(fs storage.FS, path string, data []byte) error {
	perm := os.FileMode(0o644)
	if info, err := fs.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	return fs.WriteFile(path, data, perm)
}

// includeFile is one schema include.
type includeFile struct {
	path           string
	display        string // the include path as written
	inProject      bool
	namespaced     bool
	hasAutomations bool
}

// collectIncludes resolves the root schema's includes recursively, relative
// to the schema's directory as the metamodel loader does, in the loader's
// merge order (depth-first, a file after its own includes). A missing or
// unparsable include is skipped: the loader reports it.
func collectIncludes(ctx *project.Context, fs storage.FS) []includeFile {
	data, err := fs.ReadFile(ctx.SchemaPath)
	if err != nil {
		return nil
	}
	rootDir := filepath.Dir(ctx.SchemaPath)
	projectRoot := absPath(ctx.Root)
	seen := map[string]bool{absPath(ctx.SchemaPath): true}
	var out []includeFile

	var walk func(data []byte)
	walk = func(data []byte) {
		root := documentRoot(data)
		if root == nil {
			return
		}
		incs := migration.GetMapValue(root, "includes")
		if incs == nil || incs.Kind != yaml.SequenceNode {
			return
		}
		for _, n := range incs.Content {
			if n.Kind != yaml.ScalarNode {
				continue
			}
			full := absPath(filepath.Join(rootDir, n.Value))
			if seen[full] {
				continue
			}
			seen[full] = true
			incData, err := fs.ReadFile(full)
			if err != nil {
				continue
			}
			walk(incData)
			inc := includeFile{path: full, display: n.Value}
			rel, err := filepath.Rel(projectRoot, full)
			inc.inProject = err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
			if r := documentRoot(incData); r != nil {
				if ns := migration.GetMapValue(r, "namespace"); ns != nil && ns.Kind == yaml.ScalarNode && ns.Value != "" {
					inc.namespaced = true
				}
				var d yaml.Node
				if yaml.Unmarshal(incData, &d) == nil {
					inc.hasAutomations = (&migration.AutomationsToFlowsMigration{}).Detect(&d)
				}
			}
			out = append(out, inc)
		}
	}
	walk(data)
	return out
}

func absPath(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return filepath.Clean(p) // coverage-ignore: defensive: Abs fails only when Getwd does
}

// documentRoot parses data and returns its top-level mapping, or nil.
func documentRoot(data []byte) *yaml.Node {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	root := migration.GetDocumentRoot(&doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}

// parseAutomationSource parses a schema file. found is false when the
// automations-to-flows migration does not detect the file. An automations:
// value that is not a list is refused, so it is never dropped.
func parseAutomationSource(path, display string, data []byte) (src *automationSource, found bool, err error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, false, fmt.Errorf("%s: parsing YAML: %w", display, err)
	}
	if !(&migration.AutomationsToFlowsMigration{}).Detect(&doc) {
		return nil, false, nil
	}
	root := migration.GetDocumentRoot(&doc)
	src = &automationSource{path: path, display: display, data: data, root: root}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "automations" {
			continue
		}
		v := root.Content[i+1]
		if v.Kind != yaml.SequenceNode {
			return nil, false, fmt.Errorf("%s:%d: automations: is not a list of automations; write each "+
				"automation as a list item (- name: ...), or remove the key, then run 'rela migrate' again",
				display, v.Line)
		}
		src.keyIdx = i
		src.entries = v.Content
	}
	for i, e := range src.entries {
		if err := refuseAnchors(e, ""); err != nil {
			return nil, false, fmt.Errorf("%s: automation %s: %w; write the value out in full", display,
				entryLabel(e, i), err)
		}
	}
	if err := src.decodeStrict(); err != nil {
		return nil, false, err
	}
	return src, true, nil
}

var (
	yamlLineRE     = regexp.MustCompile(`^line (\d+): (.*)$`)
	unknownFieldRE = regexp.MustCompile(`field (\S+) not found in type \S+`)
)

// decodeStrict decodes the automations with known fields only, refusing
// with the automation and the key.
func (s *automationSource) decodeStrict() error {
	dec := yaml.NewDecoder(bytes.NewReader(s.data))
	dec.KnownFields(true)
	var f legacySchemaFile
	err := dec.Decode(&f)
	if err == nil {
		s.decoded = f.Automations
		return nil
	}
	var te *yaml.TypeError
	if !errors.As(err, &te) || len(te.Errors) == 0 {
		return fmt.Errorf("%s: %w", s.display, err)
	}
	msgs := make([]string, 0, len(te.Errors))
	for _, e := range te.Errors {
		line, text := 0, e
		if m := yamlLineRE.FindStringSubmatch(e); m != nil {
			line, _ = strconv.Atoi(m[1])
			text = m[2]
		}
		text = unknownFieldRE.ReplaceAllString(text, `unknown key "$1"`)
		label := "?"
		for i, entry := range s.entries {
			if entry.Line <= line {
				label = entryLabel(entry, i)
			}
		}
		msgs = append(msgs, fmt.Sprintf("%s:%d: automation %s: %s", s.display, line, label, text))
	}
	return fmt.Errorf("%s; fix the automation (a key the old loader ignored has no effect and can be "+
		"removed), then run 'rela migrate' again", strings.Join(msgs, "; "))
}

// entryLabel names an automation for a message: its quoted name, or its
// position when unnamed.
func entryLabel(entry *yaml.Node, i int) string {
	if n := migration.GetMapValue(entry, "name"); n != nil && n.Kind == yaml.ScalarNode && n.Value != "" {
		return strconv.Quote(n.Value)
	}
	return fmt.Sprintf("#%d (unnamed)", i+1)
}

// refuseAnchors fails on an anchor or alias anywhere under n.
func refuseAnchors(n *yaml.Node, path string) error {
	if n.Kind == yaml.AliasNode {
		return fmt.Errorf("key %q uses a YAML alias", strings.TrimPrefix(path, "."))
	}
	if n.Anchor != "" {
		return fmt.Errorf("key %q declares a YAML anchor", strings.TrimPrefix(path, "."))
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			p := path + "." + n.Content[i].Value
			if err := refuseAnchors(n.Content[i], p); err != nil {
				return err
			}
			if err := refuseAnchors(n.Content[i+1], p); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			if err := refuseAnchors(c, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	default:
		// A scalar has no children. Document and alias nodes do not occur
		// here: an alias was refused above and entries are never documents.
	}
	return nil
}

// pendingTrigger is one converted automation before it is named.
type pendingTrigger struct {
	src   *automationSource
	idx   int
	named bool
	conv  convertedTrigger
}

// convertSources converts every automation and assigns trigger names.
// existing maps a trigger name already in flows.yaml to its value; an
// automation whose converted value equals it was written by an earlier run
// and is only counted in recovered.
func convertSources(sources []*automationSource, existing map[string]*yaml.Node) (
	out []convertedTrigger, recovered int, err error,
) {
	all, explicit, err := collectPending(sources)
	if err != nil {
		return nil, 0, err
	}

	used := map[string]bool{}
	var conflicts []string
	for pos, p := range all {
		name := p.src.decoded[p.idx].Name
		if !p.named {
			name = unnamedTriggerName(pos, p.conv.value, explicit, used, existing)
		}
		used[name] = true
		if ex, ok := existing[name]; ok {
			if sameValue(ex, p.conv.value) {
				recovered++
				continue
			}
			conflicts = append(conflicts, strconv.Quote(name))
			continue
		}
		p.conv.name = name
		if p.named {
			p.conv.key = withComments(migration.GetMapValue(p.src.entries[p.idx], "name"), p.conv.key)
		} else {
			p.conv.key = withComments(scalar(name), p.conv.key)
		}
		out = append(out, p.conv)
	}
	if len(conflicts) > 0 {
		return nil, 0, fmt.Errorf("%s already declares triggers %s that differ from the schema automations of "+
			"the same name; remove or rename one side, then run 'rela migrate' again",
			flowsFile, strings.Join(conflicts, ", "))
	}
	return out, recovered, nil
}

// collectPending converts every automation in source order and checks the
// explicit names. explicit maps each explicit name to its file.
func collectPending(sources []*automationSource) (all []pendingTrigger, explicit map[string]string, err error) {
	explicit = map[string]string{}
	for _, src := range sources {
		for i, entry := range src.entries {
			conv, err := convertAutomation(entry, src.decoded[i])
			if err != nil {
				return nil, nil, fmt.Errorf("%s: automation %s: %w", src.display, entryLabel(entry, i), err)
			}
			conv.source = src.display
			name := src.decoded[i].Name
			if err := checkExplicitName(src.display, name, explicit); err != nil {
				return nil, nil, err
			}
			if name != "" {
				explicit[name] = src.display
			}
			all = append(all, pendingTrigger{src: src, idx: i, named: name != "", conv: conv})
		}
	}
	return all, explicit, nil
}

// checkExplicitName refuses an explicit name that is not a valid trigger
// name or that another automation already uses. An empty name passes.
func checkExplicitName(display, name string, explicit map[string]string) error {
	if name == "" {
		return nil
	}
	if strings.ContainsAny(name, ".:") {
		return fmt.Errorf("%s: automation %q: a trigger name may not contain '.' or ':'; "+
			"rename it before running 'rela migrate' (check background jobs and audit queries "+
			"that use the old name)", display, name)
	}
	if first, dup := explicit[name]; dup {
		return fmt.Errorf("%s: automation %q: the name is already used in %s; "+
			"trigger names must be unique, rename one of them", display, name, first)
	}
	return nil
}

// unnamedTriggerName names the unnamed automation at position pos (0-based,
// across all files) automation-<pos+1>, or the next free number. A number is
// taken when an explicit name, an earlier automation or a different trigger
// in flows.yaml uses it. An equal trigger in flows.yaml does not take it:
// that one was written by an earlier run.
func unnamedTriggerName(pos int, value *yaml.Node, explicit map[string]string, used map[string]bool,
	existing map[string]*yaml.Node,
) string {
	for n := pos + 1; ; n++ {
		cand := fmt.Sprintf("automation-%d", n)
		if _, taken := explicit[cand]; taken || used[cand] {
			continue
		}
		if ex, ok := existing[cand]; ok && !sameValue(ex, value) {
			continue
		}
		return cand
	}
}

// withComments returns a copy of name carrying the comments collected on
// carrier (the automation's head comment).
func withComments(name, carrier *yaml.Node) *yaml.Node {
	k := *name
	k.Kind = yaml.ScalarNode
	if carrier != nil && carrier.HeadComment != "" {
		k.HeadComment = joinComments(carrier.HeadComment, k.HeadComment)
	}
	k.Line, k.Column = 0, 0
	return &k
}

func joinComments(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "\n" + b
}

// sameValue compares two trigger values by their decoded form.
func sameValue(a, b *yaml.Node) bool {
	var va, vb any
	if a.Decode(&va) != nil || b.Decode(&vb) != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}

// convertAutomation builds the trigger value for one automation entry. The
// returned key carries the comments that sat above the entry; the caller
// replaces it with the name node.
func convertAutomation(entry *yaml.Node, dec legacyAutomation) (convertedTrigger, error) {
	value := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", FootComment: entry.FootComment}
	carrier := &yaml.Node{HeadComment: entry.HeadComment}
	var outputs, checks []*yaml.Node
	var outputsKey *yaml.Node
	outSeq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}

	for i := 0; i+1 < len(entry.Content); i += 2 {
		k, v := entry.Content[i], entry.Content[i+1]
		switch k.Value {
		case "name":
			carrier.HeadComment = joinComments(carrier.HeadComment, k.HeadComment)
		case "description":
			value.Content = append(value.Content, k, v)
		case "on":
			value.Content = append(value.Content, k, convertOn(v))
		case "do":
			if outputsKey == nil {
				outputsKey = renamed(k, "outputs")
				copySeqComments(outSeq, v)
			}
			for j, item := range v.Content {
				outs, err := convertDoItem(item, dec.Do[j])
				if err != nil {
					return convertedTrigger{}, fmt.Errorf("do: item %d: %w", j+1, err)
				}
				outputs = append(outputs, outs...)
			}
		case "validate":
			if outputsKey == nil {
				outputsKey = renamed(k, "outputs")
				copySeqComments(outSeq, v)
			}
			for j, item := range v.Content {
				if j == 0 && k.HeadComment != "" {
					item = shallowWithHead(item, k.HeadComment)
				}
				checks = append(checks, convertCheck(item))
			}
		}
	}
	if outputsKey != nil {
		outputs = append(outputs, checks...)
		outSeq.Content = outputs
		value.Content = append(value.Content, outputsKey, outSeq)
	}
	return convertedTrigger{key: carrier, value: value}, nil
}

func copySeqComments(dst, src *yaml.Node) {
	dst.HeadComment, dst.LineComment, dst.FootComment = src.HeadComment, src.LineComment, src.FootComment
	dst.Style = src.Style &^ yaml.FlowStyle
}

func shallowWithHead(n *yaml.Node, head string) *yaml.Node {
	c := *n
	c.HeadComment = joinComments(head, c.HeadComment)
	return &c
}

// convertOn renames when -> where and condition -> when, keeping order.
func convertOn(on *yaml.Node) *yaml.Node {
	if on.Kind != yaml.MappingNode {
		return on
	}
	out := *on
	out.Content = make([]*yaml.Node, 0, len(on.Content))
	for i := 0; i+1 < len(on.Content); i += 2 {
		k, v := on.Content[i], on.Content[i+1]
		switch k.Value {
		case "when":
			k = renamed(k, "where")
		case "condition":
			k = renamed(k, "when")
		}
		out.Content = append(out.Content, k, v)
	}
	return &out
}

// convertCheck turns a validate: item into a check: output.
func convertCheck(item *yaml.Node) *yaml.Node {
	body := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for i := 0; i+1 < len(item.Content); i += 2 {
		k, v := item.Content[i], item.Content[i+1]
		if k.Value == "check" {
			k = renamed(k, "where")
		}
		body.Content = append(body.Content, k, v)
	}
	out := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", HeadComment: item.HeadComment,
		FootComment: item.FootComment}
	out.Content = []*yaml.Node{scalar("check"), body}
	return out
}

// doActionOrder is the fixed order in which one do: item's actions become
// outputs.
var doActionOrder = []string{"set", "create_relation", "add_to_pile", "notify", "create_entity", "lua", "lua_file"}

// doItem is one do: item split into its keys.
type doItem struct {
	keys map[string][2]*yaml.Node // key -> key node and value node
	// The script modifiers, in their written order.
	common, background [][2]*yaml.Node
}

func splitDoItem(item *yaml.Node) doItem {
	d := doItem{keys: map[string][2]*yaml.Node{}}
	for i := 0; i+1 < len(item.Content); i += 2 {
		pair := [2]*yaml.Node{item.Content[i], item.Content[i+1]}
		d.keys[item.Content[i].Value] = pair
		switch item.Content[i].Value {
		case "allow_acl_bypass", "capabilities":
			d.common = append(d.common, pair)
		case "background", "run_as", "retry":
			d.background = append(d.background, pair)
		}
	}
	return d
}

// convertDoItem turns one do: item into one output per action key.
func convertDoItem(item *yaml.Node, dec legacyAction) ([]*yaml.Node, error) {
	if dec.CreateEntity != nil && (dec.Lua != "" || dec.LuaFile != "") {
		return nil, errors.New("create_entity and a Lua action share one do: item; " +
			"split them into two do: items")
	}
	d := splitDoItem(item)

	var outs []*yaml.Node
	for _, action := range doActionOrder {
		pair, ok := d.keys[action]
		if !ok {
			continue
		}
		outKey, body := d.convertAction(action, pair[0], pair[1])
		out := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{outKey, body}}
		if len(outs) == 0 {
			out.HeadComment = item.HeadComment
		}
		outs = append(outs, out)
	}
	if len(outs) > 0 {
		outs[len(outs)-1].FootComment = item.FootComment
	}
	return outs, nil
}

// convertAction returns the output key and body for one action key k with
// value v. action is one of doActionOrder.
func (d doItem) convertAction(action string, k, v *yaml.Node) (outKey, body *yaml.Node) {
	switch action {
	case "set":
		val := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle}
		if vp, ok := d.keys["value"]; ok {
			val = vp[1]
		}
		props := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{v, val}}
		return k, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar("properties"), props}}
	case "create_relation":
		return renamed(k, "relate"), renameKeys(v, map[string]string{"relation": "type"})
	case "create_entity":
		return renamed(k, "create"), convertCreateEntity(v)
	case "lua", "lua_file":
		return renamed(k, "script"), d.scriptBody(action, k, v)
	}
	// add_to_pile and notify keep their key and value.
	return k, v
}

// scriptBody builds the script: body for a lua or lua_file action. The
// background modifiers go to the lua_file script when the item has both.
func (d doItem) scriptBody(action string, k, v *yaml.Node) *yaml.Node {
	srcKey := renamed(k, "lua")
	if action == "lua_file" {
		srcKey = renamed(k, "file")
	}
	srcKey.HeadComment = ""
	body := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{srcKey, v}}
	for _, c := range d.common {
		body.Content = append(body.Content, c[0], c[1])
	}
	if _, hasFile := d.keys["lua_file"]; action == "lua_file" || !hasFile {
		for _, c := range d.background {
			body.Content = append(body.Content, c[0], c[1])
		}
	}
	return body
}

// convertCreateEntity maps relation: r to relate: {type: r, from: subject},
// keeping the other keys in place.
func convertCreateEntity(v *yaml.Node) *yaml.Node {
	if v.Kind != yaml.MappingNode {
		return v
	}
	out := *v
	out.Content = make([]*yaml.Node, 0, len(v.Content))
	for i := 0; i+1 < len(v.Content); i += 2 {
		k, val := v.Content[i], v.Content[i+1]
		if k.Value == "relation" {
			rel := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map",
				Content: []*yaml.Node{scalar("type"), val, scalar("from"), scalar("subject")}}
			out.Content = append(out.Content, renamed(k, "relate"), rel)
			continue
		}
		out.Content = append(out.Content, k, val)
	}
	return &out
}

func renameKeys(v *yaml.Node, names map[string]string) *yaml.Node {
	if v.Kind != yaml.MappingNode {
		return v
	}
	out := *v
	out.Content = make([]*yaml.Node, 0, len(v.Content))
	for i := 0; i+1 < len(v.Content); i += 2 {
		k := v.Content[i]
		if n, ok := names[k.Value]; ok {
			k = renamed(k, n)
		}
		out.Content = append(out.Content, k, v.Content[i+1])
	}
	return &out
}

func renamed(k *yaml.Node, name string) *yaml.Node {
	c := *k
	c.Value = name
	return &c
}

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func encodeNode(n *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err // coverage-ignore: defensive: Close flushes into a bytes.Buffer
	}
	return buf.Bytes(), nil
}

// lines splits data into lines that keep their newline.
func lines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	parts := strings.SplitAfter(string(data), "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func isTopComment(line string) bool { return strings.HasPrefix(line, "#") }

// blockRange returns the 0-based line range [start, end) of the top-level
// key at root.Content[keyIdx]: from the key line, with the comment lines
// directly above it, to the line before the next top-level key and the
// comment lines directly above that one (or EOF).
func blockRange(ls []string, root *yaml.Node, keyIdx int) (start, end int) {
	start = root.Content[keyIdx].Line - 1
	for start > 0 && isTopComment(ls[start-1]) {
		start--
	}
	end = len(ls)
	if keyIdx+2 < len(root.Content) {
		end = root.Content[keyIdx+2].Line - 1
		for end > start+1 && isTopComment(ls[end-1]) {
			end--
		}
	}
	return start, end
}

// cut returns the file with its automations: block removed.
func (s *automationSource) cut() []byte {
	ls := lines(s.data)
	start, end := blockRange(ls, s.root, s.keyIdx)
	return []byte(strings.Join(ls[:start], "") + strings.Join(ls[end:], ""))
}

// flowsDoc is a parsed flows.yaml.
type flowsDoc struct {
	root     *yaml.Node // top-level mapping, nil when the file has none
	trigIdx  int        // index of the triggers key, -1 when absent
	existing map[string]*yaml.Node
}

func parseFlows(data []byte) (*flowsDoc, error) {
	f := &flowsDoc{trigIdx: -1, existing: map[string]*yaml.Node{}}
	if len(bytes.TrimSpace(data)) == 0 {
		return f, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: parsing YAML: %w", flowsFile, err)
	}
	root := migration.GetDocumentRoot(&doc)
	if root == nil || (root.Kind == yaml.ScalarNode && root.Tag == "!!null") {
		return f, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: the top level is not a mapping; fix the file, then run 'rela migrate' again",
			flowsFile)
	}
	f.root = root
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "triggers" {
			continue
		}
		f.trigIdx = i
		v := root.Content[i+1]
		switch {
		case v.Kind == yaml.MappingNode && v.Style&yaml.FlowStyle != 0 && len(v.Content) > 0:
			return nil, fmt.Errorf("%s: triggers: is written in flow style ({...}); rewrite it as a block "+
				"mapping, then run 'rela migrate' again", flowsFile)
		case v.Kind == yaml.MappingNode:
			for j := 0; j+1 < len(v.Content); j += 2 {
				f.existing[v.Content[j].Value] = v.Content[j+1]
			}
		case v.Kind == yaml.ScalarNode && v.Tag == "!!null":
		default:
			return nil, fmt.Errorf("%s: triggers: is not a mapping; fix it, then run 'rela migrate' again", flowsFile)
		}
	}
	return f, nil
}

// withTriggers returns data with the triggers appended. Only the new
// entries are encoded; every existing byte is kept.
func (f *flowsDoc) withTriggers(data []byte, triggers []convertedTrigger) ([]byte, error) {
	entries := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, t := range triggers {
		entries.Content = append(entries.Content, t.key, t.value)
	}

	if f.trigIdx < 0 || len(f.root.Content[f.trigIdx+1].Content) == 0 {
		return f.withTriggersBlock(data, entries)
	}

	trig := f.root.Content[f.trigIdx+1]
	indent := strings.Repeat(" ", trig.Content[0].Column-1)
	enc, err := encodeNode(entries)
	if err != nil {
		return nil, err
	}
	var block strings.Builder
	for _, l := range lines(enc) {
		if strings.TrimSpace(l) != "" {
			block.WriteString(indent)
		}
		block.WriteString(l)
	}

	ls := lines(data)
	end := len(ls)
	if f.trigIdx+2 < len(f.root.Content) {
		end = f.root.Content[f.trigIdx+2].Line - 1
	}
	for end > 0 && (strings.TrimSpace(ls[end-1]) == "" || isTopComment(ls[end-1])) {
		end--
	}
	before := strings.Join(ls[:end], "")
	if !strings.HasSuffix(before, "\n") {
		before += "\n"
	}
	return []byte(before + block.String() + strings.Join(ls[end:], "")), nil
}

// withTriggersBlock writes a whole triggers: block holding entries. It
// replaces an empty triggers: key, or is appended when the file has none.
func (f *flowsDoc) withTriggersBlock(data []byte, entries *yaml.Node) ([]byte, error) {
	block, err := encodeNode(&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map",
		Content: []*yaml.Node{scalar("triggers"), entries}})
	if err != nil {
		return nil, err
	}
	if f.trigIdx >= 0 {
		ls := lines(data)
		start := f.root.Content[f.trigIdx].Line - 1
		end := len(ls)
		if f.trigIdx+2 < len(f.root.Content) {
			end = f.root.Content[f.trigIdx+2].Line - 1
			for end > start+1 && isTopComment(ls[end-1]) {
				end--
			}
		}
		return []byte(strings.Join(ls[:start], "") + string(block) + strings.Join(ls[end:], "")), nil
	}
	out := append([]byte(nil), data...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	if len(bytes.TrimSpace(out)) > 0 {
		out = append(out, '\n')
	}
	return append(out, block...), nil
}
