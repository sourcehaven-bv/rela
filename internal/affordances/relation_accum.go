package affordances

// relationAccumulator unions relation grants across roles. A relation
// type's verdict is sparse: it appears in the output only when it
// deviates from the permissive default (creatable + removable + all
// meta writable).
//
// Closed-world for relations is per (role, type): a role that declares
// relations: for the entity type makes the listed relation types
// closed-world for that role's contribution. Cross-role union: a
// relation operation is allowed if ANY role grants it.
type relationAccumulator struct {
	// optedInTypes are relation types some role opted into (declared a
	// grant for). Only these can be denied.
	optedInTypes map[string]bool
	// creatable / removable accumulate the union of allowed ops.
	creatable map[string]bool
	removable map[string]bool
	// metaAllowed[type][field] = true when some role grants writing
	// that meta field under a passing predicate.
	metaAllowed map[string]map[string]bool
	// metaCandidates records every meta field any role mentioned for a
	// type, so unlisted-but-mentioned fields can be denied.
	metaCandidates map[string]map[string]bool
	// metaOptedIn[type] is true when a role declared meta-field grants
	// for that relation type (closed-world for meta on that type).
	metaOptedIn map[string]bool
	// metaAll[type] is true when a role on the row declares no meta-field
	// grants for that type and may update the row: it writes every meta
	// key, so the type's meta stays open.
	metaAll map[string]bool

	denyRole map[string]string // "<type>:<dim>" → role
}

func newRelationAccumulator() *relationAccumulator {
	return &relationAccumulator{
		optedInTypes:   map[string]bool{},
		creatable:      map[string]bool{},
		removable:      map[string]bool{},
		metaAllowed:    map[string]map[string]bool{},
		metaCandidates: map[string]map[string]bool{},
		metaOptedIn:    map[string]bool{},
		metaAll:        map[string]bool{},
		denyRole:       map[string]string{},
	}
}

// observe folds one role's compiled relation grant into the union.
// grantPassed is whether the grant's whole-grant predicate evaluated
// true; a failed predicate denies create/remove for this role's
// contribution (but other roles may still grant). metaPassed carries
// per-meta-field pass results (field → allowed), already AND-ed with
// grantPassed by the resolver.
func (a *relationAccumulator) observe(
	role string, rg compiledRelationGrant, grantPassed bool, metaPassed map[string]bool,
) {
	rt := rg.relation
	a.optedInTypes[rt] = true

	// create/remove default to true when the grant exists and its
	// predicate passes; an explicit false denies for this role.
	if grantPassed {
		if create := rg.create == nil || *rg.create; create {
			a.creatable[rt] = true
		} else {
			a.recordDeny(rt, "create", role)
		}
		if remove := rg.remove == nil || *rg.remove; remove {
			a.removable[rt] = true
		} else {
			a.recordDeny(rt, "remove", role)
		}
	} else {
		a.recordDeny(rt, "create", role)
		a.recordDeny(rt, "remove", role)
	}

	// Meta-field grants are closed-world for the relation type when any
	// role declares them.
	if len(rg.fields) > 0 {
		a.metaOptedIn[rt] = true
	}
	for _, fg := range rg.fields {
		a.candidateMeta(rt, fg.field)
		if metaPassed[fg.field] {
			a.allowMeta(rt, fg.field)
		} else {
			a.denyMeta(rt, fg.field, role)
		}
	}
}

func (a *relationAccumulator) recordDeny(rt, dim, role string) {
	key := rt + ":" + dim
	if _, ok := a.denyRole[key]; !ok {
		a.denyRole[key] = role
	}
}

func (a *relationAccumulator) candidateMeta(rt, field string) {
	if a.metaCandidates[rt] == nil {
		a.metaCandidates[rt] = map[string]bool{}
	}
	a.metaCandidates[rt][field] = true
}

func (a *relationAccumulator) allowMeta(rt, field string) {
	if a.metaAllowed[rt] == nil {
		a.metaAllowed[rt] = map[string]bool{}
	}
	a.metaAllowed[rt][field] = true
	delete(a.denyRole, rt+":fields."+field)
}

func (a *relationAccumulator) denyMeta(rt, field, role string) {
	if !a.metaAllowed[rt][field] {
		a.recordDeny(rt, "fields."+field, role)
	}
}

// allowAllMeta records a role that writes every meta key of rt.
func (a *relationAccumulator) allowAllMeta(rt string) {
	a.metaAll[rt] = true
}

// verdicts emits the sparse per-relation-type verdict map. A type
// appears only when it deviates from the permissive default.
//
// Meta grants are an allowlist, like entity `fields:` grants: once a role
// declares meta fields for a type, every key no role grants is read-only.
// declared returns the type's declared meta properties, which are listed
// with every key a grant names so the wire can mark them, and its order
// properties, which are listed writable unless a grant names them. A key
// outside all of these is read-only through RelationVerdict.MetaClosed.
func (a *relationAccumulator) verdicts(
	declared func(rt string) (props, order []string),
) map[string]RelationVerdict {
	var out map[string]RelationVerdict
	for rt := range a.optedInTypes {
		creatable := a.creatable[rt]
		removable := a.removable[rt]

		var fields map[string]bool
		var attribution map[string]string
		closed := a.metaOptedIn[rt] && !a.metaAll[rt]
		if closed {
			fields = map[string]bool{}
			props, order := declared(rt)
			for _, field := range props {
				fields[field] = a.metaAllowed[rt][field]
			}
			for _, field := range order {
				fields[field] = true
			}
			for field := range a.metaCandidates[rt] {
				fields[field] = a.metaAllowed[rt][field]
			}
			for field, writable := range fields {
				if !writable {
					attribution = recordAttribution(attribution, "fields."+field,
						"relation-meta-read-only", a.denyRole[rt+":fields."+field])
				}
			}
		}

		// Skip types that are fully permissive (nothing deviates).
		if creatable && removable && !closed {
			continue
		}

		if !creatable {
			attribution = recordAttribution(attribution, "create",
				"relation-not-creatable", a.denyRole[rt+":create"])
		}
		if !removable {
			attribution = recordAttribution(attribution, "remove",
				"relation-not-removable", a.denyRole[rt+":remove"])
		}

		if out == nil {
			out = map[string]RelationVerdict{}
		}
		out[rt] = RelationVerdict{
			Creatable:   creatable,
			Removable:   removable,
			Fields:      fields,
			MetaClosed:  closed,
			Attribution: attribution,
		}
	}
	return out
}
