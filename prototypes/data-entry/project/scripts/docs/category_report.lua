-- Category report: composes a markdown document for a category entity,
-- showing the category's own properties plus every ticket that belongs
-- to it (incoming `belongs-to`) with each ticket's blocks/labels.
--
-- This is the Lua equivalent of the `category_report` view, but rendered
-- as a single scrolling markdown doc with clickable form links built via
-- rela.url. Drive it from data-entry.yaml:
--
--   documents:
--     category_overview:
--       title: "Category Overview"
--       entity_type: category
--       script: docs/category_report.lua
--
-- Demonstrates:
--   - `rela.document.entry_id` — the entity being rendered.
--   - `rela.cache.memoize(...)` — caching a per-category rollup across
--     HTTP requests within the lifetime of the rela-server process.
--   - `rela.url(...)` — app-relative links verified against the frontend
--     route catalogue; form routes get a return_to appended automatically.

local entry_id = rela.document.entry_id
local category = rela.get_entity(entry_id)
if category == nil then
  print("_Category not found: " .. entry_id .. "_")
  return
end

-- Category header + properties.
print("# " .. (category.properties.title or category.properties.name or entry_id))
print()
if category.properties.description and category.properties.description ~= "" then
  print(category.properties.description)
  print()
end

-- Build the ticket rollup and cache it. The compute function walks the
-- graph via `trace_to` (incoming `belongs-to`) to find the category's
-- tickets, then returns a plain table that memoize stores. We include
-- the category's mod_time in the key so the cache invalidates when the
-- category changes; a fuller solution would also key on ticket changes
-- (tracked as TKT-E1FO1).
local cache_key = "category-tickets:" .. entry_id .. "@" .. (category.mod_time or "")
local tickets = rela.cache.memoize(cache_key, function()
  local result = {}
  local incoming = rela.trace_to(entry_id, 1) or {children = {}}
  for _, child in ipairs(incoming.children or {}) do
    local t = rela.get_entity(child.id)
    if t ~= nil and t.type == "ticket" then
      table.insert(result, {
        id = t.id,
        title = t.properties.title or t.id,
        status = t.properties.status or "open",
        priority = t.properties.priority or "medium",
        assignee = t.properties.assignee or "unassigned",
      })
    end
  end
  return result
end)

print("## Tickets (" .. #tickets .. ")")
print()
if #tickets == 0 then
  print("_No tickets belong to this category yet._")
  print()
else
  print("| ID | Title | Status | Priority | Assignee |")
  print("|----|-------|--------|----------|----------|")
  for _, t in ipairs(tickets) do
    -- The server appends return_to so submitting the form lands back here.
    local href = rela.url("/form/edit_ticket/" .. t.id)
    local link = "[" .. t.id .. "](" .. href .. ")"
    print("| " .. link ..
          " | " .. t.title ..
          " | " .. t.status ..
          " | " .. t.priority ..
          " | " .. t.assignee .. " |")
  end
  print()
end

-- Footer: link to the create form for a new ticket in this category.
print("---")
print()
print("[+ New ticket in this category](" ..
  rela.url("/form/create_ticket", {["rel.belongs-to"] = entry_id}) .. ")")
