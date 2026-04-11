-- view-deps.lua
-- Collect all entity IDs reachable from an entry entity by following
-- specific relation types. This is the Lua equivalent of `rela view deps`.
--
-- Usage:
--   rela script examples/view-deps.lua REQ-001 addresses implements
--
-- Arguments:
--   1: entry entity ID
--   2+: relation types to follow (both outgoing and incoming)
--
-- Output: JSON array of unique entity IDs (sorted)

local entry_id = rela.args[1]
if not entry_id then
    error("usage: rela script view-deps.lua <entry-id> [relation-types...]")
end

-- Collect relation types to follow from remaining args
local follow = {}
for i = 2, #rela.args do
    follow[rela.args[i]] = true
end
local follow_all = next(follow) == nil -- follow all relations if none specified

-- BFS traversal collecting all reachable entity IDs
local seen = {}
local queue = {entry_id}
seen[entry_id] = true

while #queue > 0 do
    local id = table.remove(queue, 1)

    -- Follow outgoing relations
    local outgoing = rela.get_relations({from = id})
    for _, rel in ipairs(outgoing) do
        if (follow_all or follow[rel.type]) and not seen[rel.to] then
            seen[rel.to] = true
            table.insert(queue, rel.to)
        end
    end

    -- Follow incoming relations
    local incoming = rela.get_relations({to = id})
    for _, rel in ipairs(incoming) do
        if (follow_all or follow[rel.type]) and not seen[rel.from] then
            seen[rel.from] = true
            table.insert(queue, rel.from)
        end
    end
end

-- Collect and sort IDs
local ids = {}
for id in pairs(seen) do
    table.insert(ids, id)
end
table.sort(ids)

rela.output(ids)
