---
id: TKT-EK1JO8
type: ticket
title: 'SchemaOutput / EntityOutput views: typed JSON-serialisation surface off Metamodel and EntityDef, delete 4 dead EntityDef methods (25 → 18 and 24 → 13 exported)'
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
status: backlog
---

Sub-ticket of [[TKT-N0IKN9]]. Follows the migration SchemaAdapter ticket. Clears
BOTH metamodel offenders: `Metamodel` (types.go:22) and `EntityDef`
(types.go:278). The model to copy is `AttachmentPolicy`
(`internal/metamodel/attachments.go:86-96`) — the extraction the epic already
records as "the ratchet working as intended".

## The smell

`internal/metamodel/schema_output.go` is 16 methods of field getters for two
consumers: `internal/cli/schema_json.go` (which declares its own local
interfaces at :17-33 to bind them) and `internal/mcp/{tools_schema.go:31,
resources.go:52, prompts.go:285}`. Three return `any` (`GetEntities`,
`GetRelations`, `GetTypes`) and one more on EntityDef (`GetProperties() any`) —
CLAUDE.md's "don't leak interface{}" rule violated on the producer side. Six
EntityDef getters (`GetAliases`, `GetIDPatterns`, `GetProperties`, `GetRDFType`,
`GetColor`, `GetBorderColor`) have exactly one caller each — the same caller,
`schema_json.go:94-104` — and five are one-line returns of an already-exported
field.

## Shape

```go
// SchemaOutput is a focused view over the JSON-serialisable surface: the
// `rela schema --json` writer and the MCP schema resource. Consumers depend
// on the narrow output shape, not the whole schema.
type SchemaOutput struct{ m *Metamodel }
func NewSchemaOutput(m *Metamodel) SchemaOutput          // m non-nil
func (s SchemaOutput) Version() string
func (s SchemaOutput) Namespace() string
func (s SchemaOutput) Entities() map[string]EntityDef    // was any
func (s SchemaOutput) Relations() map[string]RelationDef // was any
func (s SchemaOutput) Types() map[string]CustomType      // was any
func (s SchemaOutput) TypeDefault(name string) string    // was GetTypeDefault
func (s SchemaOutput) WidgetForType(t string) string     // was ResolveWidgetFromType (presentation concern)
func (s SchemaOutput) EntityType(name string) (EntityOutput, bool)

type EntityOutput struct{ e *EntityDef }
func (o EntityOutput) Aliases() []string
func (o EntityOutput) IDPatterns() []string
func (o EntityOutput) Properties() map[string]PropertyDef // was any
func (o EntityOutput) RDFType() string
func (o EntityOutput) Color() string
func (o EntityOutput) BorderColor() string
```

The two local interfaces in `schema_json.go:17-33` are then deleted — there is
one implementation and a concrete value type is what the consumer wanted.

## Delete (zero external callers, verify in-package/test use first)

`EntityDef.GetLabelPlural` (entity_def.go:87), `IsSequentialID` (:290),
`HasPattern` (:319), `HasContent` (types.go:576). `MatchesID` (:324) has an
in-package caller → unexport.

## Not in this pass

An `IDPolicy` view (`GetIDType`/`GetIDCaps`/`GetIDPrefixes`/`IsShortID`/
`IsManualID`/`MatchesID` — the genuinely cohesive "how are ids minted and
recognised" cluster, consumed mostly by entitymanager). It would take EntityDef
to 7 and give entitymanager a narrow thing to bind, but is not needed to clear
the line. Record as the next ratchet step if EntityDef grows.

## Result

`Metamodel` 18 exported, `EntityDef` 13 — both directives DELETED (under 20).
Golden tests for `rela schema --json` and the MCP schema resource pin the output
bytes; run them.
