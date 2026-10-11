---
id: AM-bleve-matcher-index-oracle
type: automated-measure
title: 'Oracle test: bleve MatchedFields never reports a field the index did not match'
description: Indexes one entity per table case and checks that MatchedFields reports the property exactly when the bleve index returns the entity. Catches any drift between the field-visibility provenance and the index query rules (the class that produced BUG-IPO4UH).
kind: test
location: internal/search/bleveindex/fieldmatch_test.go (TestMatchedFields_AgreesWithTheIndex)
status: active
---
