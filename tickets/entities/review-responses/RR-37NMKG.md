---
id: RR-37NMKG
type: review-response
title: The appbuildtest Deps hand-copy stays a comment where it should be structure
finding: The fixture's Deps literal remains a hand-copy of buildEntityManager's rather than a call to it, and the note added by this change ('the authorization deps are the ones where drift is dangerous — each is either required by entitymanager.New or taken from TransitionWiring') is accurate today only because it was checked by hand. That is precisely the class of claim that was previously false for CopyReadGate and became this security bug. Deps will grow another optional authz field and the note will be wrong again, with nothing failing. The structural remedy is for the fixture to CALL the production Deps construction rather than mirror it — TKT-WRLDAPI item 5 already named this hand-copy as the root cause.
severity: minor
reason: 'Agreed, and deliberately not done here. The remedy is a refactor of the composition root''s Deps construction affecting both wiring sites and every test using the fixture, which is a different change from the one-line-guard bug fix under review — mixing them would make the security fix harder to review and to revert. The immediate risk is already removed by other means: the three authorization deps this bug was about are no longer hand-copied (they come from the shared TransitionWiring), and any FUTURE authz dep that is forgotten under a policy is caught by requireCopyGates rather than by the comment. Filed as follow-up work on the ticket; the reviewer explicitly scoped it as ''worth a ticket, not a scope expansion''.'
status: deferred
---
