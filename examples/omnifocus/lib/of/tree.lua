-- Helpers for the project > action group > action tree.
--
-- Every action has at most one child_of edge, to its project or to the
-- action group it sits in. Blocking (sequential order, holds, deferral) is
-- not stored: the roll-ups on action compute it on read.
local M = {}

-- The id of the project or group an action sits in, or nil.
function M.parent_of(action_id)
  local r = rela.get_relations({ from = action_id, type = "child_of" })
  return r[1] and r[1].to or nil
end

-- The project or group an action sits in, or nil.
function M.container_of(action)
  local parent = M.parent_of(action.id)
  if parent then return rela.get_entity(parent) end
  return nil
end

-- The number of open direct children of a project or group.
function M.open_children(container)
  local n = 0
  for _, rel in ipairs(rela.get_relations({ to = container.id, type = "child_of" })) do
    local child = rela.get_entity(rel.from)
    if child and child.properties.status == "open" then n = n + 1 end
  end
  return n
end

-- Days since 1970-01-01 for a YYYY-MM-DD string (Howard Hinnant's
-- days_from_civil). Scripts have no os.time.
function M.day_number(iso)
  local y, m, d = iso:match("(%d+)-(%d+)-(%d+)")
  y, m, d = tonumber(y), tonumber(m), tonumber(d)
  if m <= 2 then y = y - 1 end
  local era = math.floor(y / 400)
  local yoe = y - era * 400
  local mp = (m + 9) % 12
  local doy = math.floor((153 * mp + 2) / 5) + d - 1
  local doe = yoe * 365 + math.floor(yoe / 4) - math.floor(yoe / 100) + doy
  return era * 146097 + doe - 719468
end

return M
