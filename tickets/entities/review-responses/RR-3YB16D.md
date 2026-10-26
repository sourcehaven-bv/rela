---
id: RR-3YB16D
type: review-response
title: Agreement test misses the cases most likely to drift; two rows are vacuous
finding: 'internal/appbuild/realtime_runas_agree_internal_test.go:57-141. (1) No policy combines a role with a fields:/options: block and a block-less role holding the verb (the BUG-Y5UF3K union). That fold is JobGrants'' most intricate logic, and only acl unit tests cover it, not the comparison with FieldVerdicts/CreateFieldVerdicts. (2) No `type@face` fields:/visible:/options: block, so blockKeys is never compared. (3) Options are compared only for custom-type enums (line 187, svc.meta.Types[pd.Type].Values); inline `values:` enums, which grantChecker.enumValues also handles, are never compared. (4) ''local role only'' has no owns edge from system:bot, and ''asserted only'' binds a principal with no claims, so both rows deny trivially and prove nothing. (5) No mixed faced/unfaced attach_to row (see the grants.go finding). Fix: add those policies and schema shapes. For (4), seed the edge and stamp a claim, then assert that the request ALSO denies, or drop the rows.'
severity: minor
resolution: 'The agreement test covers role unions with and without fields blocks, type@face blocks, inline values: enums, and a local-role edge and asserted claim that grant a person but not the bot.'
status: addressed
---
