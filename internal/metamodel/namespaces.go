package metamodel

// Namespaces and variants (TKT-C91W7H4, RES-TFI5YCG) are sugar the loader
// expands. After loading, only ordinary concrete entity types exist, so the
// store, tracer, search, ACL evaluation and every backend see nothing new.
//
//   - An included file may declare `namespace: isms`. Every name it declares
//     gets the prefix `isms.`: entity types, plurals, aliases, relation
//     types, inverse names, custom types, validations and automations.
//   - Inside that file a name resolves locally first, then in the root. A
//     qualified name (`projects.task`) always means itself.
//   - The dot is reserved for that prefix: no hand-written name contains one.
//   - `abstract: true` declares a shape that stores no entities. A concrete
//     type with `extends: <abstract>` is a variant: the loader merges the
//     parent's shape into it.
//   - A bare abstract name expands to its variants wherever the schema takes
//     a set of types. Where it takes exactly one type, an abstract name is a
//     load error that lists the variants.

import (
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// NamespaceSeparator joins a namespace and a local name (`isms.task`).
const NamespaceSeparator = "."

// namespaceNamePattern is what `namespace:` may hold: one plain segment.
var namespaceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// AbstractType is an abstract entity type the loader removed from
// [Metamodel.Entities] after expanding it.
type AbstractType struct {
	// Name is the type's name as written (qualified when it was declared in
	// a namespaced file).
	Name string
	// Def is the declared shape, before any variant merged it.
	Def EntityDef
	// Variants lists the concrete types that extend it, sorted.
	Variants []string
}

// AbstractTypes returns m's abstract types sorted by name.
// Nil: accepted, returns nil.
func AbstractTypes(m *Metamodel) []AbstractType {
	if m == nil || len(m.abstracts) == 0 {
		return nil
	}
	out := make([]AbstractType, 0, len(m.abstracts))
	for _, name := range sortedKeys(m.abstracts) {
		a := m.abstracts[name]
		a.Variants = slices.Clone(a.Variants)
		out = append(out, a)
	}
	return out
}

// VariantsOf returns the variants of the abstract type name, and whether
// name is abstract. Nil: accepted for m, reports false.
func VariantsOf(m *Metamodel, name string) ([]string, bool) {
	if m == nil {
		return nil, false
	}
	a, ok := m.abstracts[name]
	if !ok {
		return nil, false
	}
	return slices.Clone(a.Variants), true
}

// NamespaceOf returns the namespace part of a qualified name, or "" for a
// name declared in the root.
func NamespaceOf(name string) string {
	ns, _, ok := strings.Cut(name, NamespaceSeparator)
	if !ok {
		return ""
	}
	return ns
}

// AbstractTypeError reports that a caller named an abstract type where a
// concrete one is needed. At runtime only concrete types exist.
type AbstractTypeError struct {
	Name     string
	Variants []string
}

func (e *AbstractTypeError) Error() string {
	return fmt.Sprintf("entity type %q is abstract and stores no entities; use one of its variants: %s",
		e.Name, strings.Join(e.Variants, ", "))
}

// CheckConcreteType returns an [*AbstractTypeError] when name is an abstract
// type of m, and nil otherwise. Callers use it to turn an "unknown type"
// answer into one that lists the variants.
// Nil: accepted for m, returns nil.
func CheckConcreteType(m *Metamodel, name string) error {
	if variants, ok := VariantsOf(m, name); ok {
		return &AbstractTypeError{Name: name, Variants: variants}
	}
	return nil
}

// UnknownEntityTypeError is the error for a type name m does not declare:
// an [*AbstractTypeError] listing the variants when name is abstract,
// otherwise "unknown entity type: <name>".
// Nil: accepted for m.
func UnknownEntityTypeError(m *Metamodel, name string) error {
	if err := CheckConcreteType(m, name); err != nil {
		return err
	}
	return fmt.Errorf("unknown entity type: %s", name)
}

// sourcePos is where a definition was written.
type sourcePos struct {
	file string
	line int
}

func (p sourcePos) String() string {
	switch {
	case p.file != "" && p.line > 0:
		return fmt.Sprintf("%s:%d", p.file, p.line)
	case p.line > 0:
		return fmt.Sprintf("line %d", p.line)
	default:
		return p.file
	}
}

// describe names an entity type with its source, for errors.
func (e *EntityDef) describe(name string) string {
	if s := e.source.String(); s != "" {
		return fmt.Sprintf("entity %q (%s)", name, s)
	}
	return fmt.Sprintf("entity %q", name)
}

// recordFileEntityInfo records, for an included file, the property order
// and the source line of every entity type it declares. The root file gets
// both from parseRaw.
func recordFileEntityInfo(data []byte, file string, entities map[string]EntityDef) {
	var root yaml.Node
	if yaml.Unmarshal(data, &root) != nil {
		return
	}
	doc, ok := documentMapping(&root)
	if !ok {
		return
	}
	node, ok := mappingValue(doc, "entities")
	if !ok || node.Kind != yaml.MappingNode {
		return
	}
	extractEntityPropertyOrder(node, &Metamodel{Entities: entities})
	recordEntitySources(node, file, entities)
}

// recordEntitySources records the file and line of every entity type the
// `entities:` mapping node declares.
func recordEntitySources(node *yaml.Node, file string, entities map[string]EntityDef) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		def, ok := entities[key.Value]
		if !ok {
			continue
		}
		def.source = sourcePos{file: file, line: key.Line}
		entities[key.Value] = def
	}
}

// setSourceFile names the file of every entity whose source has a line but
// no file yet: the root file, whose name parseRaw does not know.
func setSourceFile(m *Metamodel, file string) {
	for name, def := range m.Entities {
		if def.source.file == "" {
			def.source.file = file
			m.Entities[name] = def
		}
	}
}

// checkDeclaredNames rejects a dot in any name a schema file writes by hand.
// The dot is reserved for the namespace prefix, so a hand-written dotted
// name would be indistinguishable from a qualified one.
func checkDeclaredNames(
	file string,
	types map[string]CustomType,
	entities map[string]EntityDef,
	relations map[string]RelationDef,
	validations []ValidationRule,
	automations []AutomationDef,
) []string {
	var errs []string
	where := ""
	if file != "" {
		where = " in " + file
	}
	bad := func(kind, name string) {
		errs = append(errs, fmt.Sprintf(
			"%s %q%s: names must not contain %q; it separates a namespace from a name (isms.task)",
			kind, name, where, NamespaceSeparator))
	}
	// A leading dot is left to the directory-name checks, whose message
	// (hidden directory, parent escape) is the more useful one.
	dotted := func(s string) bool { return strings.Index(s, NamespaceSeparator) > 0 }
	for _, name := range sortedKeys(types) {
		if dotted(name) {
			bad("type", name)
		}
	}
	for _, name := range sortedKeys(entities) {
		def := entities[name]
		if dotted(name) {
			bad("entity", name)
		}
		if dotted(def.Plural) {
			bad("plural of entity", name)
		}
		for _, alias := range def.Aliases {
			if dotted(alias) {
				bad("alias", alias)
			}
		}
	}
	for _, name := range sortedKeys(relations) {
		def := relations[name]
		if dotted(name) {
			bad("relation", name)
		}
		if def.Inverse != nil && dotted(def.Inverse.ID) {
			bad("inverse of relation", name)
		}
	}
	for _, v := range validations {
		if dotted(v.Name) {
			bad("validation", v.Name)
		}
	}
	for _, a := range automations {
		if dotted(a.Name) {
			bad("automation", a.Name)
		}
	}
	return errs
}

// qualifyPartial prefixes every name a namespaced file declares and resolves
// the names it references locally first. A file without `namespace:` is left
// as written.
func qualifyPartial(p *partialMetamodel) error {
	ns := p.Namespace
	if ns == "" {
		return nil
	}
	if !namespaceNamePattern.MatchString(ns) {
		return &SchemaValidationError{Errors: []string{fmt.Sprintf(
			"%s: namespace %q must match %s", p.sourcePath, ns, namespaceNamePattern)}}
	}
	q := newQualifier(ns, p)
	q.file = p.sourcePath
	types := make(map[string]CustomType, len(p.Types))
	for name, def := range p.Types {
		types[q.name(name)] = def
	}
	p.Types = types
	p.Entities = q.entities(p.Entities)
	p.Relations = q.relations(p.Relations)
	for i := range p.Validations {
		q.validation(&p.Validations[i])
	}
	for i := range p.Automations {
		q.automation(&p.Automations[i])
	}
	if errs := *q.errs; len(errs) > 0 {
		sort.Strings(errs)
		return &SchemaValidationError{Errors: errs}
	}
	return nil
}

// qualifier prefixes the names one namespaced file declares and resolves
// the names it references: a name the file declares means the local one,
// any other name is left as written (a root name, or a qualified one).
type qualifier struct {
	ns, file                      string
	errs                          *[]string
	localEntities, localRelations map[string]bool
	localTypes                    map[string]bool
}

func newQualifier(ns string, p *partialMetamodel) qualifier {
	q := qualifier{
		ns:             ns,
		errs:           new([]string),
		localEntities:  make(map[string]bool, len(p.Entities)),
		localRelations: make(map[string]bool, len(p.Relations)),
		localTypes:     make(map[string]bool, len(p.Types)),
	}
	for name := range p.Entities {
		q.localEntities[name] = true
	}
	for name, def := range p.Relations {
		q.localRelations[name] = true
		if def.Inverse != nil && def.Inverse.ID != "" {
			q.localRelations[def.Inverse.ID] = true
		}
	}
	for name := range p.Types {
		q.localTypes[name] = true
	}
	return q
}

func (q qualifier) name(name string) string { return q.ns + NamespaceSeparator + name }

func (q qualifier) ref(local map[string]bool, name string) string {
	if local[name] {
		return q.name(name)
	}
	return name
}

func (q qualifier) entityRef(name string) string   { return q.ref(q.localEntities, name) }
func (q qualifier) relationRef(name string) string { return q.ref(q.localRelations, name) }

// optRelationRef resolves a relation reference that may be empty.
func (q qualifier) optRelationRef(name string) string {
	if name == "" {
		return ""
	}
	return q.relationRef(name)
}

// checkExpr refuses a quoted local name inside an expression, such as
// `related('assigned-to')` or `entity.type == 'task'`. The loader does not
// rewrite expressions, so an unqualified local name there would silently
// mean the root type or relation of the same name.
func (q qualifier) checkExpr(where, expr string) {
	if expr == "" {
		return
	}
	for _, local := range []map[string]bool{q.localEntities, q.localRelations} {
		for _, name := range sortedKeys(local) {
			if strings.Contains(expr, "'"+name+"'") || strings.Contains(expr, `"`+name+`"`) {
				*q.errs = append(*q.errs, fmt.Sprintf(
					"%s: %s names %q, which this file declares; an expression is not qualified "+
						"automatically, so write %q", q.file, where, name, q.name(name)))
			}
		}
	}
}

func (q qualifier) props(props map[string]PropertyDef) map[string]PropertyDef {
	if props == nil {
		return nil
	}
	out := make(map[string]PropertyDef, len(props))
	for name, pd := range props {
		q.checkExpr(fmt.Sprintf("property %q computed", name), pd.Computed)
		pd.Type = q.ref(q.localTypes, pd.Type)
		if pd.Rollup != nil {
			r := *pd.Rollup
			r.Via = q.relationRef(r.Via)
			if r.Type != "" {
				r.Type = q.entityRef(r.Type)
			}
			pd.Rollup = &r
		}
		out[name] = pd
	}
	return out
}

func (q qualifier) entities(in map[string]EntityDef) map[string]EntityDef {
	out := make(map[string]EntityDef, len(in))
	for name, def := range in {
		// `extends: task` inside a file that declares `task` names the root
		// task: a type cannot extend itself.
		if def.Extends != "" && def.Extends != name {
			def.Extends = q.entityRef(def.Extends)
		}
		if def.Plural != "" {
			def.Plural = q.name(def.Plural)
		}
		def.Aliases = mapNames(def.Aliases, q.name)
		def.Properties = q.props(def.Properties)
		for _, scope := range sortedKeys(def.QueryScopes) {
			q.checkExpr(fmt.Sprintf("entity %q query_scopes.%s", name, scope), def.QueryScopes[scope])
		}
		out[q.name(name)] = def
	}
	return out
}

func (q qualifier) relations(in map[string]RelationDef) map[string]RelationDef {
	out := make(map[string]RelationDef, len(in))
	for name, def := range in {
		def.From = mapNames(def.From, q.entityRef)
		def.To = mapNames(def.To, q.entityRef)
		if def.Inverse != nil && def.Inverse.ID != "" {
			inv := *def.Inverse
			inv.ID = q.name(inv.ID)
			def.Inverse = &inv
		}
		def.Properties = q.props(def.Properties)
		out[q.name(name)] = def
	}
	return out
}

func (q qualifier) validation(v *ValidationRule) {
	if v.EntityType != "" {
		v.EntityType = q.entityRef(v.EntityType)
	}
	q.checkExpr(fmt.Sprintf("validation %q when_condition", v.Name), v.WhenCondition)
	q.checkExpr(fmt.Sprintf("validation %q then_condition", v.Name), v.ThenCondition)
	v.Name = q.name(v.Name)
	if len(v.Relations) > 0 {
		rels := make(map[string]RelationConstraint, len(v.Relations))
		for name, c := range v.Relations {
			if c.TargetType != "" {
				c.TargetType = q.entityRef(c.TargetType)
			}
			rels[q.relationRef(name)] = c
		}
		v.Relations = rels
	}
}

func (q qualifier) automation(a *AutomationDef) {
	q.checkExpr(fmt.Sprintf("automation %q on.condition", a.Name), a.On.Condition)
	a.Name = q.name(a.Name)
	a.On.Entity = StringOrSlice(mapNames(a.On.Entity, q.entityRef))
	a.On.RelationCreated = q.optRelationRef(a.On.RelationCreated)
	a.On.RelationRemoved = q.optRelationRef(a.On.RelationRemoved)
	for j := range a.Do {
		qualifyAction(&a.Do[j], q.entityRef, q.relationRef)
	}
}

func qualifyAction(act *AutomationAction, entityRef, relationRef func(string) string) {
	if act.CreateRelation != nil {
		c := *act.CreateRelation
		c.Relation = relationRef(c.Relation)
		act.CreateRelation = &c
	}
	if act.CreateEntity != nil {
		c := *act.CreateEntity
		c.Type = entityRef(c.Type)
		if c.Relation != "" {
			c.Relation = relationRef(c.Relation)
		}
		act.CreateEntity = &c
	}
	if act.Notify != nil {
		n := *act.Notify
		n.To.Via = StringOrSlice(mapNames(n.To.Via, relationRef))
		if n.To.Type != "" {
			n.To.Type = entityRef(n.To.Type)
		}
		act.Notify = &n
	}
}

func mapNames(names []string, f func(string) string) []string {
	if names == nil {
		return nil
	}
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = f(n)
	}
	return out
}

// normalizeIRI moves the legacy root `namespace:` to `iri:`. The root key
// only ever held an ontology identifier, so every value moves, IRI-shaped
// or not; a root file has no namespace of its own.
func normalizeIRI(m *Metamodel) error {
	if m.Namespace == "" {
		return nil
	}
	if m.IRI == "" {
		m.IRI = m.Namespace
	}
	m.Namespace = ""
	return nil
}

// expandSchema runs after includes are merged and before validation. It
// merges variants into their parents' shape, removes abstract types, expands
// abstract names in every schema surface, and generates opt-in conversions.
func expandSchema(m *Metamodel) error {
	if err := normalizeIRI(m); err != nil {
		return err
	}
	if errs := expandVariants(m); len(errs) > 0 {
		return &SchemaValidationError{Errors: errs}
	}
	if len(m.abstracts) == 0 {
		return nil
	}
	errs := expandAbstractReferences(m)
	errs = append(errs, generateVariantConversions(m)...)
	if len(errs) > 0 {
		return &SchemaValidationError{Errors: errs}
	}
	return nil
}

// expandVariants merges each variant with its abstract parent and moves the
// abstract types out of m.Entities.
func expandVariants(m *Metamodel) []string {
	var errs []string
	abstracts := map[string]AbstractType{}
	for _, name := range sortedKeys(m.Entities) {
		def := m.Entities[name]
		if !def.Abstract.Set {
			continue
		}
		if def.Extends != "" {
			errs = append(errs, def.describe(name)+
				": an abstract type cannot extend another type (one level of variants)")
		}
		if len(def.Aliases) > 0 {
			errs = append(errs, def.describe(name)+
				": an abstract type stores no entities, so it cannot have aliases")
		}
		abstracts[name] = AbstractType{Name: name, Def: def}
	}
	// A concrete type's alias or plural equal to an abstract name would make
	// the name mean the variants in one place and that type in another.
	for _, name := range sortedKeys(m.Entities) {
		def := m.Entities[name]
		for _, alias := range append(slices.Clone(def.Aliases), def.Plural) {
			if _, clash := abstracts[alias]; clash && alias != "" && !def.Abstract.Set {
				errs = append(errs, fmt.Sprintf(
					"%s: alias or plural %q is the name of an abstract type", def.describe(name), alias))
			}
		}
	}
	for _, name := range sortedKeys(m.Entities) {
		def := m.Entities[name]
		if def.Abstract.Set || def.Extends == "" {
			continue
		}
		parent, ok := abstracts[def.Extends]
		if !ok {
			if _, exists := m.Entities[def.Extends]; exists {
				errs = append(errs, fmt.Sprintf(
					"%s extends %q, which is not abstract; only an abstract type (abstract: true) can be extended",
					def.describe(name), def.Extends))
			} else {
				errs = append(errs, fmt.Sprintf(
					"%s extends %q, which is not declared", def.describe(name), def.Extends))
			}
			continue
		}
		m.Entities[name] = mergeVariant(name, parent.Def, def)
		parent.Variants = append(parent.Variants, name)
		abstracts[def.Extends] = parent
	}
	for _, name := range sortedKeys(abstracts) {
		a := abstracts[name]
		if len(a.Variants) == 0 {
			errs = append(errs, fmt.Sprintf(
				"%s is abstract but no type extends it; add `extends: %s` to a type or remove `abstract`",
				a.Def.describe(name), name))
		}
		sort.Strings(a.Variants)
		abstracts[name] = a
		delete(m.Entities, name)
	}
	if len(abstracts) > 0 {
		m.abstracts = abstracts
	}
	return errs
}

// mergeVariant returns the variant's definition with the parent's shape
// merged in. The variant inherits every property, face, query scope and
// setting; what it writes itself wins. A property the variant redeclares
// keeps the parent's settings it does not set. Nothing can be removed.
func mergeVariant(name string, parent, child EntityDef) EntityDef {
	out := parent
	out.Abstract = AbstractDef{}
	out.Aliases = child.Aliases
	out.Extends = child.Extends
	out.source = child.source
	overlayNonZero(reflect.ValueOf(&out).Elem(), reflect.ValueOf(child), map[string]bool{
		"Properties": true, "PropertyOrder": true, "Faces": true, "QueryScopes": true,
		"Aliases": true, "Abstract": true, "Extends": true, "Plural": true,
		"IDPrefix": true, "IDPrefixes": true,
	})

	// An id prefix is one setting written two ways; the variant's spelling
	// replaces the parent's whole.
	if child.IDPrefix != "" || len(child.IDPrefixes) > 0 {
		out.IDPrefix, out.IDPrefixes = child.IDPrefix, child.IDPrefixes
	}

	// The plural names a folder and a URL segment, so two variants cannot
	// share the parent's. A namespaced variant gets the parent's plural under
	// its namespace; otherwise the default (name + "s") applies.
	out.Plural = child.Plural
	if out.Plural == "" && parent.Plural != "" {
		if ns := NamespaceOf(name); ns != "" {
			out.Plural = ns + NamespaceSeparator + parent.Plural
		}
	}

	out.Properties = make(map[string]PropertyDef, len(parent.Properties)+len(child.Properties))
	maps.Copy(out.Properties, parent.Properties)
	for k, pd := range child.Properties {
		// A redeclared property that changes its type starts over: the
		// parent's values, labels and limits belong to the old type.
		if base, ok := parent.Properties[k]; ok && (pd.Type == "" || pd.Type == base.Type) {
			overlayNonZero(reflect.ValueOf(&base).Elem(), reflect.ValueOf(pd), nil)
			pd = base
		}
		out.Properties[k] = pd
	}
	out.PropertyOrder = mergeOrder(parent.PropertyOrder, child.PropertyOrder, parent.Properties, child.Properties)

	out.Faces = mergeMaps(parent.Faces, child.Faces)
	if out.Faces != nil {
		out.faceOrder = mergeOrder(faceOrderOfDef(parent), faceOrderOfDef(child), parent.Faces, child.Faces)
	}
	out.QueryScopes = mergeMaps(parent.QueryScopes, child.QueryScopes)
	cloneContainers(reflect.ValueOf(&out).Elem())
	return out
}

// cloneContainers gives each exported slice and map field of a struct its
// own copy, so sibling variants never share the parent's backing arrays.
func cloneContainers(v reflect.Value) {
	for i := range v.NumField() {
		if !v.Type().Field(i).IsExported() {
			continue
		}
		f := v.Field(i)
		switch {
		case f.Kind() == reflect.Slice && !f.IsNil():
			c := reflect.MakeSlice(f.Type(), f.Len(), f.Len())
			reflect.Copy(c, f)
			f.Set(c)
		case f.Kind() == reflect.Map && !f.IsNil():
			c := reflect.MakeMapWithSize(f.Type(), f.Len())
			iter := f.MapRange()
			for iter.Next() {
				c.SetMapIndex(iter.Key(), iter.Value())
			}
			f.Set(c)
		}
	}
}

// faceOrderOfDef returns def's faces in declaration order, or sorted when no
// order was recorded.
func faceOrderOfDef(def EntityDef) []string {
	if def.faceOrder != nil {
		return slices.Clone(def.faceOrder)
	}
	return sortedNames(def.Faces)
}

// overlayNonZero copies every exported, non-zero field of src onto dst,
// except the fields named in skip.
func overlayNonZero(dst, src reflect.Value, skip map[string]bool) {
	t := src.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() || skip[f.Name] {
			continue
		}
		if v := src.Field(i); !v.IsZero() {
			dst.Field(i).Set(v)
		}
	}
}

func mergeMaps[V any](parent, child map[string]V) map[string]V {
	if parent == nil && child == nil {
		return nil
	}
	out := make(map[string]V, len(parent)+len(child))
	maps.Copy(out, parent)
	maps.Copy(out, child)
	return out
}

// mergeOrder lists the parent's names in its order, then the names only the
// child declares in the child's order. A side without a recorded order
// contributes its names sorted.
func mergeOrder[V any](parentOrder, childOrder []string, parent, child map[string]V) []string {
	if parentOrder == nil && childOrder == nil && parent == nil && child == nil {
		return nil
	}
	if parentOrder == nil {
		parentOrder = sortedNames(parent)
	}
	if childOrder == nil {
		childOrder = sortedNames(child)
	}
	out := slices.Clone(parentOrder)
	for _, k := range childOrder {
		if _, inherited := parent[k]; !inherited {
			out = append(out, k)
		}
	}
	return out
}

// expandNames replaces each abstract name in names with its variants,
// keeping order and dropping duplicates.
func expandNames(m *Metamodel, names []string) []string {
	if names == nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, n := range names {
		if a, ok := m.abstracts[n]; ok {
			for _, v := range a.Variants {
				add(v)
			}
			continue
		}
		add(n)
	}
	return out
}

// expandAbstractReferences rewrites every place the schema names a type.
// Where a surface takes a set of types, an abstract name becomes its
// variants. Where it takes exactly one, an abstract name is an error.
func expandAbstractReferences(m *Metamodel) []string {
	for _, name := range sortedKeys(m.Relations) {
		def := m.Relations[name]
		def.From = expandNames(m, def.From)
		def.To = expandNames(m, def.To)
		m.Relations[name] = def
	}
	for i := range m.Validations {
		v := &m.Validations[i]
		if a, ok := m.abstracts[v.EntityType]; ok {
			v.variantTypes = slices.Clone(a.Variants)
		}
	}
	for i := range m.Automations {
		a := &m.Automations[i]
		a.On.Entity = StringOrSlice(expandNames(m, a.On.Entity))
	}
	if m.Comments != nil {
		m.Comments.On = expandNames(m, m.Comments.On)
	}
	for _, wname := range sortedKeys(m.Worlds) {
		w := m.Worlds[wname]
		w.Overrides = expandWorldOverrides(m, w.Overrides)
		m.Worlds[wname] = w
	}
	return singleTypeErrors(m)
}

// expandWorldOverrides copies an abstract type's face chain to each variant
// that has no chain of its own.
func expandWorldOverrides(m *Metamodel, in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return in
	}
	out := make(map[string][]string, len(in))
	for typ, chain := range in {
		if _, ok := m.abstracts[typ]; !ok {
			out[typ] = chain
		}
	}
	for _, typ := range sortedKeys(in) {
		a, ok := m.abstracts[typ]
		if !ok {
			continue
		}
		for _, v := range a.Variants {
			if _, explicit := out[v]; !explicit {
				out[v] = slices.Clone(in[typ])
			}
		}
	}
	return out
}

// singleTypeErrors reports each setting that takes one concrete type and
// names an abstract one.
func singleTypeErrors(m *Metamodel) []string {
	var errs []string
	single := func(where, name string) {
		if a, ok := m.abstracts[name]; ok {
			errs = append(errs, fmt.Sprintf(
				"%s: %q is abstract and this setting takes one concrete type; name one of %s",
				where, name, strings.Join(a.Variants, ", ")))
		}
	}
	for _, a := range m.Automations {
		for j, act := range a.Do {
			if act.CreateEntity != nil {
				single(fmt.Sprintf("automation %q: do[%d].create_entity.type", a.Name, j), act.CreateEntity.Type)
			}
		}
	}
	for _, name := range sortedKeys(m.Conversions) {
		d := m.Conversions[name]
		single(fmt.Sprintf("conversion %q: from", name), d.From)
		single(fmt.Sprintf("conversion %q: to", name), d.To)
	}
	for _, name := range sortedKeys(m.Copies) {
		d := m.Copies[name]
		for side, addr := range map[string]string{"from": d.From, "to": d.To} {
			if t, err := ParseCopyTarget(addr); err == nil {
				single(fmt.Sprintf("copy %q: %s", name, side), t.Type)
			}
		}
	}
	for _, name := range sortedKeys(m.TaskItemConversions) {
		d := m.TaskItemConversions[name]
		single(fmt.Sprintf("task_item_conversion %q: from", name), d.From)
		single(fmt.Sprintf("task_item_conversion %q: to", name), d.To)
	}
	for _, typ := range sortedKeys(m.Entities) {
		for _, prop := range sortedKeys(m.Entities[typ].Properties) {
			if r := m.Entities[typ].Properties[prop].Rollup; r != nil && r.Type != "" {
				single(fmt.Sprintf("entity %q: property %q: rollup.type", typ, prop), r.Type)
			}
		}
	}
	sort.Strings(errs)
	return errs
}

// variantConversionName names the generated conversion from one variant to
// another.
func variantConversionName(from, to string) string { return from + "-to-" + to }

// generateVariantConversions adds a conversion between each ordered pair of
// variants of an abstract type that opted in with `abstract: {conversions:
// true}`. A conversion declared by hand for the same pair wins.
//
//   - A property maps to itself when both sides declare it with the same
//     type and list-ness, and the target can be written by a conversion.
//   - The body is carried over.
//   - A relation maps to itself when the target type is allowed at each end
//     the source uses; otherwise it is dropped. The convert preview shows
//     what a conversion drops before the user confirms.
//   - Faces map by name. A source face the target lacks is a load error
//     that asks for a hand-written conversion.
func generateVariantConversions(m *Metamodel) []string {
	var errs []string
	declared := map[[2]string]bool{}
	for _, d := range m.Conversions {
		declared[[2]string{d.From, d.To}] = true
	}
	for _, aname := range sortedKeys(m.abstracts) {
		a := m.abstracts[aname]
		if !a.Def.Abstract.Conversions {
			continue
		}
		for _, from := range a.Variants {
			for _, to := range a.Variants {
				if from == to || declared[[2]string{from, to}] {
					continue
				}
				d, err := variantConversion(m, from, to)
				if err != "" {
					errs = append(errs, fmt.Sprintf("abstract %q: conversions: %s", aname, err))
					continue
				}
				if m.Conversions == nil {
					m.Conversions = map[string]ConversionDef{}
				}
				name := variantConversionName(from, to)
				if _, clash := m.Conversions[name]; clash {
					errs = append(errs, fmt.Sprintf(
						"abstract %q: conversions: a conversion named %q is already declared", aname, name))
					continue
				}
				m.Conversions[name] = d
			}
		}
	}
	return errs
}

func variantConversion(m *Metamodel, from, to string) (def ConversionDef, problem string) {
	src, dst := m.Entities[from], m.Entities[to]
	d := ConversionDef{
		From:      from,
		To:        to,
		Label:     "Convert to " + dst.Label,
		Fields:    map[string]string{},
		Content:   "{{source.content}}",
		Relations: map[string]string{},
	}
	for _, name := range sortedKeys(dst.Properties) {
		tp := dst.Properties[name]
		if tp.Computed != "" || tp.Rollup != nil || tp.Type == PropertyTypeExternalRef {
			continue
		}
		sp, ok := src.Properties[name]
		if !ok || sp.Type != tp.Type || sp.List != tp.List || sp.Computed != "" || sp.Rollup != nil {
			continue
		}
		d.Fields[name] = "{{source." + name + "}}"
	}
	for _, rname := range sortedKeys(m.Relations) {
		rel := m.Relations[rname]
		fromEnd := containsTypeOrAny(rel.From, from)
		toEnd := containsTypeOrAny(rel.To, from)
		if !fromEnd && !toEnd {
			continue
		}
		keep := (!fromEnd || containsTypeOrAny(rel.From, to)) && (!toEnd || containsTypeOrAny(rel.To, to))
		if keep {
			d.Relations[rname] = rname
		} else {
			d.Relations[rname] = ConversionDrop
		}
	}
	if len(src.Faces) > 0 || len(dst.Faces) > 0 {
		d.Faces = map[string][]string{}
		for _, face := range faceOrderOfDef(src) {
			if _, ok := dst.Faces[face]; !ok {
				return ConversionDef{}, fmt.Sprintf(
					"%q has face %q, which %q lacks; declare the conversion %q by hand",
					from, face, to, variantConversionName(from, to))
			}
			d.Faces[face] = []string{face}
		}
	}
	return d, ""
}
