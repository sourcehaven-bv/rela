-- Relation-cardinality validator for workflow gates.
--
-- Counts OUTGOING relations of a given type whose TARGET entity matches all of
-- the supplied property filters, then asserts that count against a threshold.
-- This is the Lua stopgap for the metamodel `relations:` block, which the
-- validation engine does not evaluate (ValidationRule has no such field, so the
-- block is silently dropped at parse time). See TKT-IFHO2L.
--
-- The rule's own `when:` selects which entities the check applies to; this
-- script only enforces the relation count.
--
-- Arguments (via rela.args):
--   [1] = relation type            (e.g. "has-review")
--   [2] = mode                     ("min" or "max")
--   [3] = threshold                (integer)
--   [4..] = target filters         ("key=value"), ANDed; target must match all
--
-- Returns: nil (pass) or {message=...} (violation).

local rel_type = rela.args[1]
local mode = rela.args[2]
local threshold = tonumber(rela.args[3])

if rel_type == nil or rel_type == "" then
    return { message = "require-relation-count: missing relation type (arg 1)" }
end
if mode ~= "min" and mode ~= "max" then
    return { message = "require-relation-count: mode (arg 2) must be 'min' or 'max', got '" .. tostring(mode) .. "'" }
end
if threshold == nil then
    return { message = "require-relation-count: threshold (arg 3) must be a number" }
end

-- Parse "key=value" target filters from args[4..].
local filters = {}
for i = 4, #rela.args do
    local pair = rela.args[i]
    local eq = string.find(pair, "=", 1, true)
    if eq == nil then
        return { message = "require-relation-count: filter '" .. pair .. "' must be key=value" }
    end
    filters[#filters + 1] = {
        key = string.sub(pair, 1, eq - 1),
        value = string.sub(pair, eq + 1),
    }
end

local function target_matches(target)
    if target == nil then
        return false
    end
    for _, f in ipairs(filters) do
        if (target.properties[f.key] or "") ~= f.value then
            return false
        end
    end
    return true
end

-- Count matching outgoing relations of this type.
local count = 0
local rels = rela.get_relations{ from = entity.id, type = rel_type }
for _, rel in ipairs(rels) do
    if target_matches(rela.get_entity(rel.to)) then
        count = count + 1
    end
end

-- Describe the target constraint for the violation message.
local constraint = "'" .. rel_type .. "'"
if #filters > 0 then
    local parts = {}
    for _, f in ipairs(filters) do
        parts[#parts + 1] = f.key .. "=" .. f.value
    end
    constraint = constraint .. " (" .. table.concat(parts, ", ") .. ")"
end

if mode == "min" and count < threshold then
    return {
        message = "requires at least " .. threshold .. " " .. constraint ..
            " relation(s), has " .. count,
    }
end
if mode == "max" and count > threshold then
    return {
        message = "must have at most " .. threshold .. " " .. constraint ..
            " relation(s), has " .. count,
    }
end

return nil
