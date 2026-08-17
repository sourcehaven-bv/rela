-- Open review-response gate, aware of whether the parent work item has started.
--
-- `ci-no-open-review-responses` used to fail any `status=open` review-response
-- outright. That is right for a finding on work in flight, but wrong for one
-- raised by `/design-review` on a ticket still sitting in the backlog: the
-- finding is *supposed* to stay open until someone picks the ticket up, and
-- failing CI for it pressures people into flipping it to a resolved status that
-- misrepresents the state.
--
-- So the check is conditional on the parent:
--
--   parent backlog / ready        -> allowed to stay open (not started yet)
--   parent anything else, or none -> must not be left open
--
-- "Parent" is the entity on the FROM side of a `has-review-response` relation
-- pointing at this review-response. There is normally exactly one; if several
-- exist, the finding is treated as not-yet-started only when EVERY parent is
-- unstarted — one active parent means the finding is live.
--
-- The rule's own `when:` selects `status=open` review-responses; this script
-- only decides whether that is acceptable.
--
-- Arguments: none.
--
-- Returns: nil (pass) or {message=...} (violation).

-- Statuses meaning "this work has not been picked up yet".
local UNSTARTED = {
    backlog = true,
    ready = true,
}

-- Find the work items that own this finding.
local parents = {}
local rels = rela.get_relations{ to = entity.id, type = "has-review-response" }
for _, rel in ipairs(rels) do
    local parent = rela.get_entity(rel.from)
    if parent ~= nil then
        parents[#parents + 1] = parent
    end
end

-- An orphaned finding has nothing to defer to, so hold it to the strict rule.
if #parents == 0 then
    return {
        message = "is 'open' and has no parent work item - address or close it, " ..
            "or link it to a ticket/bug with has-review-response",
    }
end

-- Unstarted only when every parent is unstarted.
local blocking = nil
for _, parent in ipairs(parents) do
    local status = parent.properties["status"] or ""
    if not UNSTARTED[status] then
        blocking = parent
        break
    end
end

if blocking == nil then
    return nil
end

return {
    message = "is 'open' while its parent " .. blocking.id .. " is '" ..
        (blocking.properties["status"] or "") .. "' - address or close it " ..
        "(findings may stay open only while the parent is backlog/ready)",
}
