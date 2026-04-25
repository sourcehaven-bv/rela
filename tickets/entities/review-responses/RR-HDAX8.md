---
id: RR-HDAX8
type: review-response
title: 'Whitespace handling: explicit check, not a side effect'
finding: 'Plan rejects display_property: '' titel '' but only because the lookup against the property map (which has whitespace stripped on declaration) coincidentally fails. Fragile: if validateEntityStructure is ever loosened, display_property handling silently shifts. Also, the diagnostic ''not a defined property'' misleads the user into thinking they typo''d.'
severity: significant
resolution: 'Add explicit whitespace check: if def.DisplayProperty != strings.TrimSpace(def.DisplayProperty), emit a dedicated error ''display_property %q has leading/trailing whitespace''. Two extra lines. Decoupled from validateEntityStructure''s trim invariant; better diagnostic.'
status: addressed
---
