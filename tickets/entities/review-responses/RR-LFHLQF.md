---
id: RR-LFHLQF
type: review-response
title: Lease semantics over faces unspecified
finding: 'Security design review: store refs carry a face and translateVerb needs one; the plan did not say which face a claim is authorized on or where the badge shows.'
severity: significant
resolution: 'Plan: one lease per family keyed by id; claim authorizes on visibility.Resolver.WriteTarget (AmbiguousAddressError, never by rank); badge on every readable face; status documented as outside visible:.'
status: addressed
---
