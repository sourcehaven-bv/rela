---
id: TKT-OFQG3J
type: ticket
title: 'One type switch for the property value domain: storeutil.ValidateProperties / canonical.normalize / entity.CloneValue disagree on which container types exist'
kind: refactor
priority: low
status: backlog
description: 'Three walkers cover the same property value domain and each lists a different set of container types: canonical.normalize knows map[any]any and entity.CloneValue lacks map[any]any and map[string]string while storeutil.ValidateProperties (BUG-X7ICNM) was written against the first two. A container type missing from one of them is a silent pass (validate) or an aliased clone (CloneValue) which is the bug class BUG-X7ICNM is about one level up. Derive all three from one shared enumeration or at least pin them against each other with a test. Raised in code review of BUG-X7ICNM.'
---
