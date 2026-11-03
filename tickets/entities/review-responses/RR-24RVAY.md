---
id: RR-24RVAY
type: review-response
title: The pre-check UpdateGrant is reused after the Append and the striped-lock wait (authorization TOCTOU)
finding: 'internal/realtime/service.go:366 and :405, internal/realtime/outputs.go:571-573 and internal/appbuild/realtime_outputs.go:162-163. The grant that AuthorizeUpdate returns before the Append is spent by the inline run, after the channel write and after Local.lockOutputs. That lock is striped over 64 keys (outputs.go:416-421), and another channel''s run can hold a stripe for up to 4 x promoteTimeout. UpdateGrant.PatchEntity skips the row ACL whenever the type, ref and principal match (entitymanager/manager.go:1349-1360). So an update grant that depends on the row (a when: on entity.owner or entity.status, or a membership edge) is judged on the row as it stood before the wait. A concurrent reassignment or status change in that window does not stop the write. The read gate still runs. Exploit: unverified; it needs a concurrent legitimate change. Fix: do not carry the grant past the Append. Use Check only as a yes/no gate and write through o.gated.PatchEntity, which authorizes again on the current row. The extra cost is one ACL evaluation per set.'
severity: minor
resolution: The pre-check grant is no longer reused; the write asks the row ACL again on the current row via the gated PatchEntity.
status: addressed
---
