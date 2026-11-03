---
id: RR-9PCUHV
type: review-response
title: Affordance offers a promote that the create ACL will refuse
finding: 'internal/realtime/actions.go:229-244 offeredActions checks channel gates only, so a member without create on promote_to.type sees the action and gets 403 promote_denied, against AC15. Fix: add a narrow CanCreate(type, face) capability evaluated once per action per page and fold it into actionCheck; extend TestAction_AffordanceAndGateAgree.'
severity: minor
resolution: Promoter gains CheckCreate which runs entitymanager.CheckCreate (face check plus create ACL as a probe) and the affordance and RunAction ask it once per type and face per call; tested by AffordanceAndGateAgree GatesOncePerPage and TestPromote_AffordanceFollowsCreateACL (453779953 6302a4b22)
status: addressed
---
