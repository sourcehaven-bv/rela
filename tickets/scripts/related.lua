-- scripts/related.lua
-- Usage: rela script scripts/related.lua <query or entity-id> [threshold] [limit]
--
-- Find entities related to a free-text query or an existing entity.
-- Useful when starting new work to discover prior decisions, existing
-- concepts, relevant bugs, and potential pitfalls.
--
-- Examples:
--   rela script scripts/related.lua "content-hash caching for AI"
--   rela script scripts/related.lua TKT-5FYM
--   rela script scripts/related.lua "embeddings" 0.6 20

local query = rela.args[1]
if not query then
  error("usage: rela script scripts/related.lua <query or entity-id> [threshold] [limit]")
end

local threshold = tonumber(rela.args[2]) or 0.65
local limit = tonumber(rela.args[3]) or 25

-- Check if the query is an existing entity ID
local query_text = query
local source_entity = rela.get_entity(query)
if source_entity then
  local parts = {}
  if source_entity.properties.title then parts[#parts+1] = source_entity.properties.title end
  if source_entity.content then parts[#parts+1] = source_entity.content end
  query_text = table.concat(parts, "\n")
  print("Related to: " .. (source_entity.properties.title or query) .. " (" .. query .. ")")
else
  print("Related to: \"" .. query .. "\"")
end
print("Threshold: " .. threshold .. " | Limit: " .. limit)

-- Build text representation (truncated to ~2000 chars to stay within
-- embedding model context limits — nomic-embed-text has 8192 tokens)
local max_chars = 2000
local function entity_text(e)
  local parts = {}
  if e.properties.title then parts[#parts+1] = e.properties.title end
  if e.content then parts[#parts+1] = e.content end
  local text = table.concat(parts, "\n")
  if #text > max_chars then
    text = text:sub(1, max_chars)
  end
  return text
end

-- Cosine similarity
local function cosine(a, b)
  local dot, na, nb = 0, 0, 0
  for i = 1, #a do
    dot = dot + a[i] * b[i]
    na = na + a[i] * a[i]
    nb = nb + b[i] * b[i]
  end
  return dot / (math.sqrt(na) * math.sqrt(nb))
end

-- Embed the query
local query_vec = ai.embed(query_text)[1]

-- Gather all entities across all types
local texts, entities = {}, {}
local types = rela.get_entity_types()
for type_name, _ in pairs(types) do
  local list = rela.list_entities(type_name)
  for _, e in ipairs(list) do
    if not source_entity or e.id ~= source_entity.id then
      local t = entity_text(e)
      if t ~= "" then
        texts[#texts+1] = t
        entities[#entities+1] = e
      end
    end
  end
end

-- Batch embed in chunks of 100
local all_vecs = {}
for i = 1, #texts, 100 do
  local batch = {}
  for j = i, math.min(i + 99, #texts) do
    batch[#batch+1] = texts[j]
  end
  local vecs, err = ai.embed(batch)
  if err then
    error("embed failed: " .. err.message)
  end
  for _, v in ipairs(vecs) do
    all_vecs[#all_vecs+1] = v
  end
end

-- Score and rank
local results = {}
for i, e in ipairs(entities) do
  local sim = cosine(query_vec, all_vecs[i])
  if sim >= threshold then
    results[#results+1] = {
      id = e.id,
      title = e.properties.title or "(no title)",
      type = e.type,
      score = sim,
      status = e.properties.status or "",
    }
  end
end
table.sort(results, function(a, b) return a.score > b.score end)

-- Trim to limit
if #results > limit then
  local trimmed = {}
  for i = 1, limit do trimmed[i] = results[i] end
  results = trimmed
end

-- Group by type for readable output
local by_type = {}
local type_order = {}
for _, r in ipairs(results) do
  if not by_type[r.type] then
    by_type[r.type] = {}
    type_order[#type_order+1] = r.type
  end
  by_type[r.type][#by_type[r.type]+1] = r
end

print(string.rep("-", 72))
print(string.format("Found %d related entities across %d types", #results, #type_order))
print("")

for _, typ in ipairs(type_order) do
  local items = by_type[typ]
  print(string.format("  %s (%d)", typ, #items))
  for _, r in ipairs(items) do
    local status_str = ""
    if r.status ~= "" then status_str = " [" .. r.status .. "]" end
    print(string.format("    %.3f  %-16s %s%s", r.score, r.id, r.title, status_str))
  end
  print("")
end

if #results == 0 then
  print("  (no entities above threshold " .. threshold .. ")")
end
