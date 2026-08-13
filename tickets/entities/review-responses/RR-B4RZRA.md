---
id: RR-B4RZRA
type: review-response
title: Soft delete dropped the alias, duplicating the to-do on replay
finding: 'DeleteCalendarObject''s soft-delete path deleted the alias, leaving the href unowned. Two bugs followed. (1) An offline client replaying its cached PUT created a SECOND entity: with no alias, and a client-minted UUID that cannot satisfy splitFeedUID, the write fell through to createFromTodo - the exact duplication registerCalDAVRoutes refuses to start without an alias service to prevent. Reproduced live: one PUT + DELETE + replay produced two entities. (2) The resource could reappear under the derived <type>--<id>@rela.ics href, since objectFor falls back to it when no alias exists, which a client reads as delete-plus-create.'
severity: critical
resolution: 'Keep the alias on soft delete, matching the hard-delete path. The transition itself removes the resource from the collection via the where: filter, so the binding can safely survive. Verified live: PUT + DELETE + replay now yields exactly one entity, updated at its original href, and a DELETE with no replay leaves status=cancelled and the resource unlisted. The pre-existing TestCalDAV_SoftDeleteDropsTheAlias asserted the old behaviour on the reasoning that a later create would ''resurrect the old entity''; re-binding an href to the entity it already names is the correct answer to a PUT, not resurrection, so that test was replaced by TestCalDAV_SoftDeleteKeepsTheAlias plus a derived-href counterpart.'
status: addressed
---
