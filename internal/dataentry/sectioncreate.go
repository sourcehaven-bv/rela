package dataentry

import (
	"context"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// SectionCreateTarget is one entity type a section's create affordance offers.
type SectionCreateTarget struct {
	EntityType string
	FormID     string
	Label      string
	// Template is the operator-preselected entity template variant for this
	// type, or "" for the form's own default. Resolved server-side from
	// `create.types.<type>.template` so the client never maps config to types.
	Template string
}

// SectionCreateInfo describes the create affordance on one view section.
type SectionCreateInfo struct {
	Relation string // relation the new entity is linked by
	LinkAs   string // "from" or "to" — role of the NEW entity in that relation
	PeerID   string // the entry entity, which the new entity links to
	Flow     string // "modal" or "page"
	Heading  string // section heading, for the header menu's label
	InHeader bool   // whether this section also contributes to the header menu
	Targets  []SectionCreateTarget
}

// creatableTargets derives the entity types a principal may actually create,
// from the types a relation can reach.
//
// Two conditions, both required, matching the rule TKT-OMUD56 settled: a create
// form must resolve for the type, AND the principal must hold create permission
// for it. The permission half goes through affordanceService.computeCollectionActions
// — the same call the list handler uses — so a button can never offer something
// the write path would refuse. It is a UI hint either way: the write
// re-authorizes independently.
//
// templates may be nil; when non-nil it supplies the per-type preselected
// template variant.
//
// The `seen` memo matters: a view section is resolved per section, and a type
// reachable by several relations would otherwise be re-authorized once per
// section per relation.
func (h *viewsHandler) creatableTargets(
	ctx context.Context, candidateTypes []string, create *dataentryconfig.SectionCreate,
) []SectionCreateTarget {
	if len(candidateTypes) == 0 {
		return nil
	}
	s := h.schema()
	var targets []SectionCreateTarget
	for _, et := range candidateTypes {
		formID := h.createFormForType(et)
		if formID == "" {
			continue
		}
		if !h.affordances.computeCollectionActions(ctx, et)["create"] {
			continue
		}
		label := et
		if ed, ok := s.Meta.GetEntityDef(et); ok && ed.Label != "" {
			label = ed.Label
		}
		targets = append(targets, SectionCreateTarget{
			EntityType: et,
			FormID:     formID,
			Label:      label,
			Template:   create.TemplateFor(et),
		})
	}
	return targets
}

// resolveSectionCreate populates CreateInfo on the sections of an entity-detail
// view (TKT-R4BMJM).
//
// This is the narrow, opt-in relaxation of TKT-651W's read-only-view invariant.
// A section is considered ONLY when its config carries an explicit `create:`
// block, so a view that says nothing keeps behaving exactly as it did — which
// is what lets TestV1Views_NoAddOrLinkInfoOnSections keep asserting absence for
// every section that did not opt in.
//
// Deliberately separate from [viewsHandler.resolveSectionButtonsWithTraverse]
// rather than a generalization of it: that function also builds LinkInfo
// unconditionally, with no form or permission check, and link-existing is out
// of scope here. Reusing it would have shipped an ungated link affordance onto
// the view path as a side effect.
//
// entry must be the view's entry entity; sections must be the built sections in
// the same order as viewConfig.Sections.
func (h *viewsHandler) resolveSectionCreate(
	ctx context.Context, viewConfig ViewConfig, sections []SectionData, entry *entity.Entity,
) {
	if entry == nil {
		return
	}
	s := h.schema()
	for i, cfgSec := range viewConfig.Sections {
		if i >= len(sections) || cfgSec.Create == nil {
			continue
		}
		// The relation is what the new entity gets linked by. A section with no
		// single originating relation has no peer to link to, so it gets no
		// affordance — config load already refuses this combination, and this
		// is the runtime half of the same rule.
		relName, linkAs, ok := dataentryconfig.SectionOriginRelation(viewConfig, cfgSec)
		if !ok {
			continue
		}
		relDef, found := s.Meta.GetRelationDef(relName)
		if !found {
			continue
		}
		candidateTypes := relDef.To
		if linkAs == "from" {
			candidateTypes = relDef.From
		}
		targets := h.creatableTargets(ctx, candidateTypes, cfgSec.Create)
		if len(targets) == 0 {
			continue
		}
		sections[i].CreateInfo = &SectionCreateInfo{
			Relation: relName,
			LinkAs:   linkAs,
			PeerID:   entry.ID,
			Flow:     cfgSec.Create.EffectiveFlow(),
			Heading:  cfgSec.Heading,
			InHeader: cfgSec.Create.PlacedIn(dataentryconfig.SectionCreateInHeader),
			Targets:  targets,
		}
	}
}

// sectionCreateToV1 lifts a section's create affordance onto the wire.
//
// Nil in, nil out — which is the common case and the one that matters: a
// section that did not opt in must leave `create` absent from its JSON.
func sectionCreateToV1(ci *SectionCreateInfo) *v1.ViewSectionCreate {
	if ci == nil {
		return nil
	}
	out := &v1.ViewSectionCreate{
		Relation: ci.Relation,
		LinkAs:   ci.LinkAs,
		PeerID:   ci.PeerID,
		Flow:     ci.Flow,
		Targets:  make([]v1.ViewSectionCreateTarget, 0, len(ci.Targets)),
	}
	for _, t := range ci.Targets {
		out.Targets = append(out.Targets, v1.ViewSectionCreateTarget{
			EntityType: t.EntityType,
			FormID:     t.FormID,
			Label:      t.Label,
			Template:   t.Template,
		})
	}
	return out
}

// sectionCreateMenuToV1 lifts the header menu onto the wire, carrying each
// entry's section heading so the menu can label itself.
func sectionCreateMenuToV1(menu []SectionCreateInfo) []v1.ViewSectionCreate {
	if len(menu) == 0 {
		return nil
	}
	out := make([]v1.ViewSectionCreate, 0, len(menu))
	for i := range menu {
		entry := sectionCreateToV1(&menu[i])
		entry.Heading = menu[i].Heading
		out = append(out, *entry)
	}
	return out
}

// headerCreateMenu collects the sections that opted into the page-header menu.
//
// Assembled server-side, deduped by relation and kept in section order, so the
// SPA renders what it is given rather than reimplementing the aggregation — and
// so a handler test can assert one fixed payload.
//
// Dedup key is the relation: two sections over one relation are one menu entry,
// taken from the first that opted in. `in`/`flow` may legitimately differ
// between such sections (they are per-section choices), but the menu can only
// show one, and section order is the only ordering an operator can see.
func headerCreateMenu(sections []SectionData) []SectionCreateInfo {
	var out []SectionCreateInfo
	seen := map[string]bool{}
	for _, sec := range sections {
		ci := sec.CreateInfo
		if ci == nil || !ci.InHeader || seen[ci.Relation] {
			continue
		}
		seen[ci.Relation] = true
		out = append(out, *ci)
	}
	return out
}
