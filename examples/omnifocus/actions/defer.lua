-- Defers the selected action by rela.params.days days, from today, to the
-- start of that day (UTC).
if entity == nil then
  return { message = "Select an action first", message_type = "warning" }
end
local day = rela.date_add(rela.now:sub(1, 10), rela.params.days .. "d")
rela.update_entity(entity.id, { defer_date = day .. "T00:00:00Z" })
