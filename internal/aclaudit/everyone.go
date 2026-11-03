package aclaudit

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// checkEveryoneOpensBlocks is A14: the everyone role holds the verb of another
// role's field block, on some face of the type, and declares no block of its
// own. Field verdicts are the union of every role on the row (BUG-Y5UF3K) and
// every principal holds everyone, so on those faces the block restricts
// nobody. See [acl.EveryoneOpenedFieldBlocks].
//
// Without a metamodel every type is treated as faceless, so a grant naming
// only a face (`type@face`) is not reported.
//
// Nil: meta accepted (faces unknown, as above).
func checkEveryoneOpensBlocks(p *acl.Policy, meta MetamodelReader) []Finding {
	var facesOf func(string) []string
	if meta != nil {
		facesOf = meta.Faces
	}
	opened := acl.EveryoneOpenedFieldBlocks(p, facesOf)
	f := make([]Finding, 0, len(opened))
	for _, opened := range opened {
		f = append(f, Finding{
			Rule: "A14-everyone-opens-field-block", Severity: High,
			Subject: fmt.Sprintf("%s.%s.%s (%s)", opened.Role, opened.Dimension, opened.EntityType, opened.Verb),
			Detail:  opened.Explain(),
			Fix: fmt.Sprintf("give role %q its own %s block for %q naming what everyone may have, "+
				"or remove the %s grant on %q from it", acl.EveryoneRole, opened.Dimension, opened.EntityType,
				opened.Verb, opened.EntityType),
		})
	}
	return f
}
