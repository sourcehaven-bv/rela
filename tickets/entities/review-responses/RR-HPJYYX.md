---
id: RR-HPJYYX
type: review-response
title: write⊆read validation ignores affordance-based grants — invariant overstated
finding: 'Policy.Validate''s new write⊆read check iterates only role.Write; a role with fields:/options:/relations: grants for a type but no read grant passes validation, while the godoc and docs claimed downstream logic ''may assume writable ⇒ readable'' without qualification — over-trusting code could treat affordance-grant types as readable.'
severity: significant
resolution: 'Verified in code that affordance grants never confer write authorization: both authorizeEntityWrite and authorizeRelationWrite resolve through decideFromAttrs against role.Write only; fields/options/relations grants restrict surfaces WITHIN an already-authorized write, so an affordance-only role is inert, not incoherent. Narrowed the documented invariant accordingly (Validate godoc ''Scope:'' paragraph, GUIDE-acl-security, docs/security.md) and added the ''affordance-only role without read ok'' case to TestLoadPolicy_WriteWithoutRead_Rejected pinning that the pass is intentional. Commit 622b6cf7.'
status: addressed
---
