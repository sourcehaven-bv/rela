-- Runs when an action's status changes: complete with last action, and
-- repeats.
local tree = require("of.tree")

local p = entity.properties

-- Complete with last action: a group or project whose last open child just
-- closed completes too.
if p.status ~= "open" then
  local container = tree.container_of(entity)
  if container and container.properties.complete_with_last_action == true
      and tree.open_children(container) == 0 then
    local st = container.properties.status
    if st == "open" or st == "active" then
      rela.update_entity(container.id, { status = "completed" })
    end
  end
end

-- Repeat: completing (or dropping) a repeating action creates the next one.
-- fixed       next dates follow the old dates
-- defer_again next defer date follows today
-- due_again   next due date follows today
if p.status ~= "open" and p.repeat_rule then
  local today = rela.now:sub(1, 10)
  local function day(v) return v and string.sub(v, 1, 10) or nil end
  local function time(v) return v and string.sub(v, 11) or "" end

  local anchor_prop = (p.repeat_method == "defer_again" and "defer_date")
    or (p.repeat_method == "due_again" and "due_date")
    or (p.due_date and "due_date") or "defer_date"
  local base = (p.repeat_method == "fixed") and (day(p[anchor_prop]) or today) or today
  local rule = p.repeat_rule
  if not string.find(rule, "DTSTART") then
    rule = rule .. ";DTSTART=" .. string.gsub(base, "-", "") .. "T000000Z"
  end
  local next_day = rela.rrule_next(rule, base)
  if next_day then
    local props = {
      title = p.title, status = "open", flagged = p.flagged,
      estimate_minutes = p.estimate_minutes,
      repeat_rule = p.repeat_rule, repeat_method = p.repeat_method,
      ordering = p.ordering,
    }
    -- Move every date by the same number of days as the anchor date.
    local shift_from = day(p[anchor_prop]) or base
    for _, k in ipairs({ "defer_date", "planned_date", "due_date" }) do
      if p[k] then
        local d = day(p[k])
        local offset = tree.day_number(next_day) - tree.day_number(shift_from)
        props[k] = rela.date_add(d, offset .. "d") .. time(p[k])
      end
    end
    local next_action = rela.create_entity("action", props, entity.content)
    local parent = tree.parent_of(entity.id)
    if parent then rela.create_relation(next_action.id, "child_of", parent) end
    for _, rel in ipairs(rela.get_relations({ from = entity.id, type = "tagged" })) do
      rela.create_relation(next_action.id, "tagged", rel.to)
    end
  end
end

