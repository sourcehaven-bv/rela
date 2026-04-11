-- view-affected.lua
-- Find which root entities are affected by changes to a set of entity IDs.
-- This is the Lua equivalent of `rela view affected`.
--
-- Usage:
--   rela script examples/view-affected.lua <root-type> <relation-types...> -- <changed-ids...>
--
-- Arguments:
--   1: entity type of root entities (e.g., "document")
--   2..N: relation types to follow during traversal
--   --: separator
--   N+1..M: changed entity IDs
--
-- Example:
--   rela script examples/view-affected.lua document addresses implements -- DEC-001 REQ-002
--
-- Output: JSON array of affected root entity IDs

-- Parse arguments: split on "--"
local root_type = rela.args[1]
if not root_type then
    error("usage: rela script view-affected.lua <root-type> <relation-types...> -- <changed-ids...>")
end

local follow = {}
local changed_ids = {}
local past_separator = false

for i = 2, #rela.args do
    if rela.args[i] == "--" then
        past_separator = true
    elseif past_separator then
        table.insert(changed_ids, rela.args[i])
    else
        follow[rela.args[i]] = true
    end
end

if #changed_ids == 0 then
    error("no changed IDs provided (use -- separator before changed IDs)")
end

local follow_all = next(follow) == nil

-- Build set of changed IDs for fast lookup
local changed_set = {}
for _, id in ipairs(changed_ids) do
    changed_set[id] = true
end

-- Collect deps for a single root (BFS, same as view-deps.lua)
local function collect_deps(entry_id)
    local seen = {}
    local queue = {entry_id}
    seen[entry_id] = true

    while #queue > 0 do
        local id = table.remove(queue, 1)

        local outgoing = rela.get_relations({from = id})
        for _, rel in ipairs(outgoing) do
            if (follow_all or follow[rel.type]) and not seen[rel.to] then
                seen[rel.to] = true
                table.insert(queue, rel.to)
            end
        end

        local incoming = rela.get_relations({to = id})
        for _, rel in ipairs(incoming) do
            if (follow_all or follow[rel.type]) and not seen[rel.from] then
                seen[rel.from] = true
                table.insert(queue, rel.from)
            end
        end
    end

    return seen
end

-- Check each root entity of the given type
local roots = rela.list_entities(root_type)
local affected = {}

for _, root in ipairs(roots) do
    local deps = collect_deps(root.id)
    for changed_id in pairs(changed_set) do
        if deps[changed_id] then
            table.insert(affected, root.id)
            break
        end
    end
end

table.sort(affected)
rela.output(affected)
