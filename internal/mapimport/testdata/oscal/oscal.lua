-- Skips withdrawn controls and gives a control without a label its key.
-- record is the source record as plain tables. JSON and XML differ in
-- shape, so both prop forms are read.
local function props(record)
  return record.props or record.prop or {}
end

function transform(draft, record)
  for _, p in ipairs(props(record)) do
    local name = p.name or p["@name"]
    local value = p.value or p["@value"]
    if name == "status" and value == "withdrawn" then
      return nil
    end
  end
  if draft.properties.label == nil then
    draft.properties.label = draft.key
  end
  return draft
end
