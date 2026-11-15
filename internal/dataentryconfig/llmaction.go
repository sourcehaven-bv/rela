package dataentryconfig

import (
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// LLMAction is the `llm:` block of an action (TKT-4USH1HO): a declarative
// LLM task that proposes changes to the entity the action is bound to.
//
// The prompt is static text. rela sends the entity data as a separate JSON
// message and never interpolates it into the prompt, so untrusted content
// stays out of the instruction channel. The reply is constrained to a JSON
// schema derived from the metamodel and is validated again on the server.
//
// The action only proposes. Nothing is written until the person accepts the
// proposal, through the ordinary write APIs.
type LLMAction struct {
	// Prompt is the operator's instruction. Required.
	Prompt string `yaml:"prompt"`
	// Model overrides the model of ai.yaml. Empty uses the configured one.
	Model string `yaml:"model,omitempty"`
	// MaxTokens caps the reply. Zero leaves it to the provider.
	MaxTokens int `yaml:"max_tokens,omitempty"`
	// Inputs names the data rela sends with the prompt.
	Inputs LLMInputs `yaml:"inputs"`
	// Outputs names what the model may propose.
	Outputs LLMOutputs `yaml:"outputs"`
}

// LLMInputs names the data an LLM action sends. Every value is read as the
// person running the action, so a field they cannot see is never sent.
type LLMInputs struct {
	// Properties are subject properties to send.
	Properties []string `yaml:"properties,omitempty"`
	// Content sends the subject's markdown body. Nil sends none.
	Content *LLMContentInput `yaml:"content,omitempty"`
	// Related sends entities related to the subject.
	Related []LLMRelatedInput `yaml:"related,omitempty"`
}

// LLMContentInput sends the subject's markdown body, cut to MaxChars.
type LLMContentInput struct {
	// MaxChars caps the body. Zero means DefaultLLMContentChars.
	MaxChars int `yaml:"max_chars,omitempty"`
}

// LLMRelatedInput sends the entities at the other end of one relation type.
type LLMRelatedInput struct {
	// Relation is the relation type. Required.
	Relation string `yaml:"relation"`
	// Direction is "outgoing" (the default) or "incoming".
	Direction string `yaml:"direction,omitempty"`
	// Fields are the properties to send for each related entity. The id,
	// type and title are always sent.
	Fields []string `yaml:"fields,omitempty"`
	// Limit caps the number of related entities. Zero means
	// DefaultLLMRelatedLimit.
	Limit int `yaml:"limit,omitempty"`
}

// LLMOutputs names what an LLM action may propose.
type LLMOutputs struct {
	// Set are subject properties the model may propose a value for.
	Set []string `yaml:"set,omitempty"`
	// Relate are outgoing relations the model may propose, each to an
	// entity picked from a candidate list.
	Relate []LLMRelateOutput `yaml:"relate,omitempty"`
}

// LLMRelateOutput lets the model propose relations of one type from the
// subject to entities picked from a candidate list.
type LLMRelateOutput struct {
	// Relation is the relation type. Required; the subject's type must be
	// one of its `from` types.
	Relation string `yaml:"relation"`
	// Candidates is a predicate on the target entity, in the condition
	// language of a list `condition:`. Empty admits every target the person
	// may read. `related(...)` is refused.
	Candidates string `yaml:"candidates,omitempty"`
	// Fields are the properties sent for each candidate. The id, type and
	// title are always sent.
	Fields []string `yaml:"fields,omitempty"`
	// Limit caps the candidate list. Zero means DefaultLLMCandidateLimit.
	Limit int `yaml:"limit,omitempty"`
	// Max caps how many relations the model may propose. Zero means 1.
	Max int `yaml:"max,omitempty"`
}

// Bounds for an LLM action. They keep the prompt, and so the cost of one
// run, bounded and visible in config.
const (
	DefaultLLMContentChars   = 8000
	MaxLLMContentChars       = 100000
	DefaultLLMRelatedLimit   = 20
	MaxLLMRelatedLimit       = 50
	DefaultLLMCandidateLimit = 100
	MaxLLMCandidateLimit     = 200
	// MaxLLMTotalItems caps the related entities and candidates of one
	// action together, so a run sends a bounded document to the model.
	MaxLLMTotalItems  = 400
	MaxLLMRelateMax   = 20
	MaxLLMPromptChars = 16000
	MaxLLMMaxTokens   = 32000

	LLMDirectionOutgoing = "outgoing"
	LLMDirectionIncoming = "incoming"
)

// ContentChars is the effective body cap.
func (c LLMContentInput) ContentChars() int {
	if c.MaxChars == 0 {
		return DefaultLLMContentChars
	}
	return c.MaxChars
}

// EffectiveLimit is the effective number of related entities.
func (r LLMRelatedInput) EffectiveLimit() int {
	if r.Limit == 0 {
		return DefaultLLMRelatedLimit
	}
	return r.Limit
}

// Outgoing reports whether the subject is the relation's from side.
func (r LLMRelatedInput) Outgoing() bool { return r.Direction != LLMDirectionIncoming }

// EffectiveLimit is the effective candidate list size.
func (r LLMRelateOutput) EffectiveLimit() int {
	if r.Limit == 0 {
		return DefaultLLMCandidateLimit
	}
	return r.Limit
}

// EffectiveMax is the effective number of relations the model may propose.
func (r LLMRelateOutput) EffectiveMax() int {
	if r.Max == 0 {
		return 1
	}
	return r.Max
}

// LLMCandidateConditionID keys the compiled `candidates:` predicate of relate
// output i of an action, for one target type.
func LLMCandidateConditionID(actionID string, i int, targetType string) string {
	return fmt.Sprintf("%s:relate[%d]:%s", actionID, i, targetType)
}

// LLMCandidateConditionType reads the target type back out of an
// [LLMCandidateConditionID].
func LLMCandidateConditionType(id string) string {
	return id[strings.LastIndexByte(id, ':')+1:]
}

// Subject is the one entity type an LLM action runs on: the only type of
// its available_on. Empty when the action has no such scope, which
// validation refuses.
func (a Action) Subject() string {
	if a.LLM == nil || a.AvailableOn == nil || len(a.AvailableOn.EntityTypes) != 1 {
		return ""
	}
	return a.AvailableOn.EntityTypes[0]
}

// validateLLMAction checks the structure of one action's `llm:` block
// against the metamodel. The candidates predicates are compiled by
// conditionlint, which knows the predicate language.
//
//nolint:gocognit // linear checklist over the block's keys; each branch is one rule.
func validateLLMAction(id string, action Action, meta *metamodel.Metamodel) []string {
	llm := action.LLM
	if llm == nil {
		return nil
	}
	where := fmt.Sprintf("actions: %q llm", id)
	var errs []string
	add := func(format string, args ...any) {
		errs = append(errs, where+": "+fmt.Sprintf(format, args...))
	}

	if action.Script != "" || len(action.Set) > 0 {
		add("an llm action has no script or set")
	}
	if action.Request != nil {
		add("request: applies only to a script action")
	}
	if len(action.Params) > 0 {
		add("params: applies only to a script action")
	}
	if action.Capabilities.Any() {
		add("capabilities: applies only to a script action; an llm action uses the AI provider by itself")
	}
	// The proposal is applied by the user through the ordinary write APIs,
	// so there is nothing for an elevation to apply to.
	if action.AllowACLBypass != "" || len(action.ElevatedUpdates) > 0 {
		add("allow_acl_bypass and elevated_updates apply only to a script action; " +
			"an llm action's changes are applied as the user")
	}
	// Every run sends data to an external provider and costs money, so the
	// operator has to name who may run it.
	if action.Permission == "" {
		add("permission is required; every run sends data to the AI provider")
	}
	if strings.TrimSpace(llm.Prompt) == "" {
		add("prompt is required")
	} else if len(llm.Prompt) > MaxLLMPromptChars {
		add("prompt is longer than %d characters", MaxLLMPromptChars)
	}
	if llm.MaxTokens < 0 || llm.MaxTokens > MaxLLMMaxTokens {
		add("max_tokens must be between 0 and %d", MaxLLMMaxTokens)
	}

	if action.AvailableOn == nil || len(action.AvailableOn.EntityTypes) != 1 {
		add("needs available_on with exactly one entity type, the type it runs on")
		return errs
	}
	subject := action.AvailableOn.EntityTypes[0]
	def, ok := meta.GetEntityDef(subject)
	if !ok {
		return errs // validateActionScope reports the unknown type
	}
	if len(def.Faces) > 0 {
		add("entity type %q has content states (faces), which llm actions do not support yet", subject)
	}

	for _, p := range llm.Inputs.Properties {
		if _, ok := def.Properties[p]; !ok {
			add("inputs.properties: unknown property %q on %q", p, subject)
		}
	}
	if c := llm.Inputs.Content; c != nil && (c.MaxChars < 0 || c.MaxChars > MaxLLMContentChars) {
		add("inputs.content.max_chars must be between 0 (the default) and %d", MaxLLMContentChars)
	}
	for i, r := range llm.Inputs.Related {
		errs = append(errs, validateLLMRelatedInput(fmt.Sprintf("%s: inputs.related[%d]", where, i), subject, r, meta)...)
	}

	if len(llm.Outputs.Set) == 0 && len(llm.Outputs.Relate) == 0 {
		add("outputs needs at least one set property or relate entry")
	}
	for _, msg := range validateLLMSet(llm.Outputs.Set, subject, def) {
		add("outputs.set: %s", msg)
	}
	if total := llmTotalItems(llm); total > MaxLLMTotalItems {
		add("the limits of inputs.related and outputs.relate add up to %d; at most %d are allowed", total, MaxLLMTotalItems)
	}
	seenRel := map[string]bool{}
	for i, r := range llm.Outputs.Relate {
		if seenRel[r.Relation] {
			add("outputs.relate: relation %q is listed twice", r.Relation)
		}
		seenRel[r.Relation] = true
		errs = append(errs, validateLLMRelateOutput(fmt.Sprintf("%s: outputs.relate[%d]", where, i), subject, r, meta)...)
	}
	return errs
}

// llmUnsettable says why a property cannot be an LLM output, or "".
func llmUnsettable(pd metamodel.PropertyDef) string {
	switch {
	case pd.Computed != "":
		return "it is computed"
	case pd.Rollup != nil:
		return "it is a rollup"
	case pd.Type == metamodel.PropertyTypeFile:
		return "it is a file"
	case pd.Type == metamodel.PropertyTypeExternalRef:
		return "it is an external_ref"
	}
	return ""
}

func validateLLMRelatedInput(where, subject string, r LLMRelatedInput, meta *metamodel.Metamodel) []string {
	var errs []string
	if r.Direction != "" && r.Direction != LLMDirectionOutgoing && r.Direction != LLMDirectionIncoming {
		errs = append(errs, fmt.Sprintf("%s: direction must be %q or %q", where, LLMDirectionOutgoing, LLMDirectionIncoming))
	}
	if r.Limit < 0 || r.Limit > MaxLLMRelatedLimit {
		errs = append(errs, fmt.Sprintf("%s: limit must be between 0 (the default) and %d", where, MaxLLMRelatedLimit))
	}
	rd, ok := meta.GetRelationDef(r.Relation)
	if !ok {
		return append(errs, fmt.Sprintf("%s: unknown relation %q", where, r.Relation))
	}
	near, far := rd.From, rd.To
	if !r.Outgoing() {
		near, far = rd.To, rd.From
	}
	if !slices.Contains(near, subject) {
		errs = append(errs, fmt.Sprintf("%s: relation %q does not connect %q in direction %s",
			where, r.Relation, subject, cmpOr(r.Direction, LLMDirectionOutgoing)))
	}
	return append(errs, llmFieldErrors(where, r.Fields, far, meta)...)
}

func validateLLMRelateOutput(where, subject string, r LLMRelateOutput, meta *metamodel.Metamodel) []string {
	var errs []string
	if r.Limit < 0 || r.Limit > MaxLLMCandidateLimit {
		errs = append(errs, fmt.Sprintf("%s: limit must be between 0 (the default) and %d", where, MaxLLMCandidateLimit))
	}
	if r.Max < 0 || r.Max > MaxLLMRelateMax {
		errs = append(errs, fmt.Sprintf("%s: max must be between 1 and %d", where, MaxLLMRelateMax))
	}
	rd, ok := meta.GetRelationDef(r.Relation)
	if !ok {
		return append(errs, fmt.Sprintf("%s: unknown relation %q", where, r.Relation))
	}
	if !slices.Contains(rd.From, subject) {
		errs = append(errs, fmt.Sprintf("%s: %q is not a from type of relation %q", where, subject, r.Relation))
	}
	if rd.Scope != metamodel.ScopeIdentity && rd.Scope != metamodel.ScopeIdentityExplicit {
		errs = append(errs, fmt.Sprintf("%s: relation %q is content-scoped, which llm actions do not support", where, r.Relation))
	}
	for name, pd := range rd.Properties {
		if pd.Required {
			errs = append(errs, fmt.Sprintf(
				"%s: relation %q has required property %q, which an llm proposal cannot fill", where, r.Relation, name))
		}
	}
	if rd.MaxOutgoing != nil && r.EffectiveMax() > *rd.MaxOutgoing {
		errs = append(errs, fmt.Sprintf("%s: max %d exceeds max_outgoing %d of relation %q",
			where, r.EffectiveMax(), *rd.MaxOutgoing, r.Relation))
	}
	for _, t := range rd.To {
		if td, ok := meta.GetEntityDef(t); ok && len(td.Faces) > 0 {
			errs = append(errs, fmt.Sprintf(
				"%s: target type %q has content states (faces), which llm actions do not support yet", where, t))
		}
	}
	return append(errs, llmFieldErrors(where, r.Fields, rd.To, meta)...)
}

// llmFieldErrors checks that every field is declared on at least one of the
// given types.
func llmFieldErrors(where string, fields, types []string, meta *metamodel.Metamodel) []string {
	var errs []string
	for _, f := range fields {
		found := false
		for _, t := range types {
			if def, ok := meta.GetEntityDef(t); ok {
				if _, ok := def.Properties[f]; ok {
					found = true
					break
				}
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf("%s: fields: property %q is not declared on %s",
				where, f, strings.Join(types, ", ")))
		}
	}
	return errs
}

func cmpOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// llmKeys and its siblings are the keys an `llm:` block accepts. Nested
// unknown keys are otherwise ignored by the decoder, so a typo such as
// `output:` would silently drop every proposal; the block checks its own.
var (
	llmKeys        = []string{"inputs", "max_tokens", "model", "outputs", "prompt"}
	llmInputKeys   = []string{"content", "properties", "related"}
	llmContentKeys = []string{"max_chars"}
	llmRelatedKeys = []string{"direction", "fields", "limit", "relation"}
	llmOutputKeys  = []string{"relate", "set"}
	llmRelateKeys  = []string{"candidates", "fields", "limit", "max", "relation"}
)

// llmUnknownKeys reports keys in any action's `llm:` block that the block
// does not define.
func llmUnknownKeys(data []byte) []string {
	var raw struct {
		Actions map[string]struct {
			LLM map[string]any `yaml:"llm"`
		} `yaml:"actions"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // the struct unmarshal reports malformed YAML
	}
	var errs []string
	for id, a := range raw.Actions {
		if a.LLM == nil {
			continue
		}
		p := fmt.Sprintf("actions.%s.llm", id)
		errs = append(errs, unknownKeysIn(p, a.LLM, llmKeys)...)
		if in, ok := a.LLM["inputs"].(map[string]any); ok {
			errs = append(errs, unknownKeysIn(p+".inputs", in, llmInputKeys)...)
			if c, ok := in["content"].(map[string]any); ok {
				errs = append(errs, unknownKeysIn(p+".inputs.content", c, llmContentKeys)...)
			}
			errs = append(errs, unknownKeysInList(p+".inputs.related", in["related"], llmRelatedKeys)...)
		}
		if out, ok := a.LLM["outputs"].(map[string]any); ok {
			errs = append(errs, unknownKeysIn(p+".outputs", out, llmOutputKeys)...)
			errs = append(errs, unknownKeysInList(p+".outputs.relate", out["relate"], llmRelateKeys)...)
		}
	}
	return errs
}

func unknownKeysInList(prefix string, v any, valid []string) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	var errs []string
	for i, item := range list {
		if m, ok := item.(map[string]any); ok {
			errs = append(errs, unknownKeysIn(fmt.Sprintf("%s[%d]", prefix, i), m, valid)...)
		}
	}
	return errs
}

// llmTotalItems is the most related entities and candidates one run of the
// action can send.
func llmTotalItems(llm *LLMAction) int {
	total := 0
	for _, r := range llm.Inputs.Related {
		total += r.EffectiveLimit()
	}
	for _, r := range llm.Outputs.Relate {
		total += r.EffectiveLimit()
	}
	return total
}

// validateLLMSet checks the properties of outputs.set against the subject
// type.
func validateLLMSet(set []string, subject string, def *metamodel.EntityDef) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range set {
		if seen[p] {
			out = append(out, fmt.Sprintf("property %q is listed twice", p))
			continue
		}
		seen[p] = true
		pd, ok := def.Properties[p]
		if !ok {
			out = append(out, fmt.Sprintf("unknown property %q on %q", p, subject))
			continue
		}
		if reason := llmUnsettable(pd); reason != "" {
			out = append(out, fmt.Sprintf("property %q cannot be proposed: %s", p, reason))
		}
	}
	return out
}
