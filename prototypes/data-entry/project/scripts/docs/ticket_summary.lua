-- Ticket summary: a compact markdown card for one ticket.
--
-- This replaces an earlier inline `command:` that shelled out to
-- `rela show {id} | jq`. That form is gone (TKT-QGHNVA): splicing the entity
-- id into a shell string made it the one piece of request-derived data
-- reaching `sh -c`, so an id leading with "-" arrived as an option flag
-- rather than an operand. `command:` is now an argv array with no shell, and
-- a renderer that needs the entity is better expressed in Lua anyway — it
-- reads the graph directly instead of shelling out to the CLI to read it back
-- as JSON.
--
--   documents:
--     ticket_summary:
--       title: "Ticket Summary"
--       entity_type: ticket
--       script: docs/ticket_summary.lua

-- `rela.document.entry_id`, NOT `rela.entity_id` — the latter is not bound by
-- the Lua runtime, so it was silently nil and every render of this document
-- failed with "bad argument #1 to get_entity (string expected, got nil)".
-- The sibling category_report.lua already used the documented name.
local id = rela.document.entry_id
local e = rela.get_entity(id)

if not e then
  print("# Unknown ticket")
  print()
  print("No entity with id `" .. tostring(id) .. "`.")
  return
end

local p = e.properties or {}

local function value(key, fallback)
  local v = p[key]
  if v == nil or v == "" then
    return fallback
  end
  return tostring(v)
end

print("# " .. value("title", "Untitled"))
print()
print("**Status:** " .. value("status", "unknown") ..
  " | **Priority:** " .. value("priority", "medium") ..
  " | **Assignee:** " .. value("assignee", "unassigned"))

local desc = p["description"]
if desc ~= nil and desc ~= "" then
  print()
  print("## Description")
  print()
  print(tostring(desc))
end
