-- Standalone document: a cross-cutting status review.
--
-- Unlike docs/category_report.lua this document is NOT about one entity — it
-- aggregates across the whole graph, so it is configured without an
-- entity_type: and reached from a navigation entry rather than an entity page.
-- rela.document.entry_id is therefore nil here.

assert(rela.mode == "document", "expected document mode")
assert(rela.document.entry_id == nil, "a standalone document has no entry entity")

print("# Status Review")
print("")

local tickets = rela.list_entities("ticket")
local categories = rela.list_entities("category")

print(string.format("%d tickets across %d categories.", #tickets, #categories))
print("")

-- Tickets by status.
local by_status = {}
for _, t in ipairs(tickets) do
  local s = t.properties.status or "unknown"
  by_status[s] = (by_status[s] or 0) + 1
end

local statuses = {}
for s in pairs(by_status) do
  statuses[#statuses + 1] = s
end
table.sort(statuses)

print("## By status")
print("")
print("| Status | Count |")
print("| ------ | ----- |")
for _, s in ipairs(statuses) do
  print(string.format("| %s | %d |", s, by_status[s]))
end
print("")

-- High-priority open work, linking back into the app.
print("## High priority")
print("")
local found = false
for _, t in ipairs(tickets) do
  if t.properties.priority == "high" and t.properties.status ~= "done" then
    found = true
    print(string.format("- [%s](/entity/ticket/%s) — %s",
      t.properties.title or t.id, t.id, t.properties.status or "unknown"))
  end
end
if not found then
  print("_Nothing high priority and open._")
end
