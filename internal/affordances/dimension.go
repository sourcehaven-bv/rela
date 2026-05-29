package affordances

// dimension answers one yes/no question for a single entity type
// across all of a principal's roles: "which fields may this user
// write?" (or, for the visibility dimension, "which fields may this
// user see?"). It exists because that answer is a union over roles
// with a closed-world twist, and that fold needs somewhere to
// accumulate.
//
// Worked example. Take a "ticket" with a `triager` role that grants
// write on {status, assignee} and a `reviewer` role that grants write
// on {status, resolution}. A user holding both should be able to write
// {status, assignee, resolution} — the UNION — and nothing else.
// "Nothing else" is the closed-world part: because at least one role
// declared a `fields:` block for ticket, every OTHER ticket field
// (title, description, ...) renders read-only. A dimension folds the
// roles one at a time (allow + observeDeny) and deny() reports the
// closed-world denials at the end.
//
// Semantics (DR-S3, DR-C4):
//   - A role "opts in" by declaring the block for this type. Once ANY
//     role opts in, the dimension is closed-world for the user.
//   - allowed accumulates the union of granted fields across roles.
//   - deny() denies every metamodel-declared field (plus any mentioned
//     candidate) that isn't in allowed.
//   - When no role opts in, the dimension stays permissive and deny()
//     emits nothing.
type dimension struct {
	// optedIn records whether any role declared this block for the
	// type. It's the closed-world switch: without it, deny() can't tell
	// "no role restricts these fields, leave them all writable" from
	// "a role opted in and granted zero, deny everything." A role
	// granting an empty list is a real, deny-all configuration
	// (DR-C4), so a non-empty allowed/candidates set isn't a reliable
	// proxy — the explicit flag is.
	optedIn    bool
	allowed    map[string]bool
	candidates map[string]bool
	// denyRole records, per candidate field, the FIRST role (in sorted
	// order) that observed a deny — used for attribution when the field
	// ends up denied. ruleKind labels the dimension for the attribution
	// string.
	denyRole map[string]string
	ruleKind string
}

func newDimension() *dimension {
	return &dimension{
		allowed:    map[string]bool{},
		candidates: map[string]bool{},
		denyRole:   map[string]string{},
	}
}

// optIn marks the dimension closed-world (a role declared the block).
func (d *dimension) optIn(ruleKind string) {
	d.optedIn = true
	d.ruleKind = ruleKind
}

// allow records a field as granted (union across roles).
func (d *dimension) allow(field string) {
	d.allowed[field] = true
	d.candidates[field] = true
	delete(d.denyRole, field)
}

// observeDeny records that role denied field (predicate failed). Only
// sticks if no role has allowed it. Keeps the FIRST denying role
// (roles iterate in sorted order) so attribution is deterministic.
func (d *dimension) observeDeny(field, role string) {
	d.candidates[field] = true
	if !d.allowed[field] {
		if _, seen := d.denyRole[field]; !seen {
			d.denyRole[field] = role
		}
	}
}

// deny emits the sparse {field: false} map for fields that are
// closed-world denied. When opted in, the deny universe is every
// metamodel-declared field (passed by the resolver) plus any
// candidate a grant mentioned — minus the union-allowed set. Returns
// the map (nil when nothing denied) and the updated attribution map.
func (d *dimension) deny(
	universe []string, attr map[string]string,
) (denied map[string]bool, attribution map[string]string) {
	if !d.optedIn {
		return nil, attr
	}
	denySet := map[string]bool{}
	for _, f := range universe {
		denySet[f] = true
	}
	for f := range d.candidates {
		denySet[f] = true
	}

	var out map[string]bool
	for field := range denySet {
		if d.allowed[field] {
			continue
		}
		if out == nil {
			out = map[string]bool{}
		}
		out[field] = false
		attr = recordAttribution(attr, field, d.ruleKind, d.denyRole[field])
	}
	return out, attr
}

// optionDimension accumulates per-(field, option) closed-world grants.
// Opt-in is per field: a field that appears under options: is
// closed-world for its options (only granted options allowed).
type optionDimension struct {
	optedInFields map[string]bool
	allowed       map[string]map[string]bool // field → option → true
	candidates    map[string]map[string]bool
	denyRole      map[string]string // "field=option" → role
}

func newOptionDimension() *optionDimension {
	return &optionDimension{
		optedInFields: map[string]bool{},
		allowed:       map[string]map[string]bool{},
		candidates:    map[string]map[string]bool{},
		denyRole:      map[string]string{},
	}
}

func (o *optionDimension) optIn(field string) {
	o.optedInFields[field] = true
}

func (o *optionDimension) allow(field, option string) {
	if o.allowed[field] == nil {
		o.allowed[field] = map[string]bool{}
	}
	o.allowed[field][option] = true
	o.candidate(field, option)
	delete(o.denyRole, field+"="+option)
}

func (o *optionDimension) observeDeny(field, option, role string) {
	o.candidate(field, option)
	if !o.allowed[field][option] {
		key := field + "=" + option
		if _, seen := o.denyRole[key]; !seen {
			o.denyRole[key] = role
		}
	}
}

func (o *optionDimension) candidate(field, option string) {
	if o.candidates[field] == nil {
		o.candidates[field] = map[string]bool{}
	}
	o.candidates[field][option] = true
}

// deny emits {field: {option: false}} for each declared option not
// granted on an opted-in field. The deny universe per field is the
// metamodel enum values (passed by the resolver) plus any mentioned
// candidate, minus the union-allowed set.
func (o *optionDimension) deny(
	enumValues map[string][]string, attr map[string]string,
) (denied map[string]map[string]bool, attribution map[string]string) {
	var out map[string]map[string]bool
	for field := range o.optedInFields {
		denySet := map[string]bool{}
		for _, v := range enumValues[field] {
			denySet[v] = true
		}
		for option := range o.candidates[field] {
			denySet[option] = true
		}
		for option := range denySet {
			if o.allowed[field][option] {
				continue
			}
			if out == nil {
				out = map[string]map[string]bool{}
			}
			if out[field] == nil {
				out[field] = map[string]bool{}
			}
			out[field][option] = false
			attr = recordAttribution(attr, field+"="+option, "enum-filtered", o.denyRole[field+"="+option])
		}
	}
	return out, attr
}

// recordAttribution adds a "<rule> via role <role>" attribution entry
// for a denied path. role may be empty (no specific role observed).
func recordAttribution(attr map[string]string, path, ruleKind, role string) map[string]string {
	if attr == nil {
		attr = map[string]string{}
	}
	if role == "" {
		attr[path] = "affordance:" + ruleKind
		return attr
	}
	attr[path] = "affordance:" + ruleKind + " role=" + role
	return attr
}
