---
id: RR-764AR
type: review-response
title: validateCreateIDOpts accepted bare-prefix IDs (e.g. id="TAG-")
finding: 'For manual-ID types with a declared prefix, strings.HasPrefix(id, p) returns true even when id == p, so a caller could POST {id: "TAG-"} to a tag type (prefix TAG-) and create a useless entity whose ID has no distinguishing suffix. Not a security hole, but exactly the kind of inconsistency the prefix rule is meant to prevent.'
severity: significant
resolution: Added len(id) > len(p) check to validateCreateIDOpts in api_v1.go. Updated error message to 'must start with one of X and include a suffix'. Added test row 'manual prefixed, bare prefix as id' to TestValidateCreateIDOpts.
status: addressed
---
