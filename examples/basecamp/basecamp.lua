-- Basecamp to-do sync for rela. See README.md for the setup.
--
-- One file holds both directions, because a rela script cannot load
-- another file:
--
--   * Run as a scheduled task (schedules.yaml), it PULLS: it reads every
--     to-do of one Basecamp to-do list and merges each into its rela todo.
--   * Run as a background automation (schema.yaml), the global `entity` is
--     set, and it PUSHES that one todo's local edits to Basecamp.
--
-- Both run as integration:basecamp and remember the last agreed state of
-- each todo as the version tag sync/basecamp. The merge rules are the ones
-- in the "Sync connectors" section of the Lua scripting guide.

local SYSTEM = "basecamp"
local TAG = "sync/basecamp"
local TYPE = "todo"
local FIELDS = {"title", "due", "done", "content"}

-- Basecamp refuses a request whose User-Agent names no contact. Put your
-- application's name and a contact address here.
local USER_AGENT = "rela-basecamp-example (ops@example.com)"

local RATE_LIMITED = "basecamp: rate limited"
local VANISHED = "vanished: Basecamp no longer lists this to-do (deleted, archived or moved)"

-- Settings. They are secrets only so the test can point the API at a stub;
-- none of them is sensitive.
local S = rela.secrets
local ACCOUNT = S.basecamp_account_id or ""
local LIST = S.basecamp_todolist_id or ""
if not ACCOUNT:match("^%d+$") or not LIST:match("^%d+$") then
  error("basecamp: set the secrets basecamp_account_id and basecamp_todolist_id to Basecamp ids", 0)
end
local API = string.gsub(S.basecamp_api_base or "https://3.basecampapi.com", "/+$", "")
-- The access token goes to API, so it must be https. Plain http is allowed
-- only to this machine, for a local stub.
local function api_allowed(base)
  if base:match("^https://[^/]") then return true end
  local host = base:match("^http://([^/:]+)")
  return host == "127.0.0.1" or host == "localhost"
end
if not api_allowed(API) then
  error("basecamp: basecamp_api_base must be an https URL (http only to 127.0.0.1 or localhost)", 0)
end
local ROOT = API .. "/" .. ACCOUNT

-- ---------------------------------------------------------------------------
-- HTTP

local token -- the access token for this run

local function access_token()
  if not token then
    local t, err = rela.oauth.access_token(SYSTEM)
    if not t then
      error("basecamp: no access token (" .. err.kind .. "): " .. err.message, 0)
    end
    token = t
  end
  return token
end

-- request sends one API call. A 401 means Basecamp rejected the access
-- token: rela.oauth.invalidate clears it only if it is still the stored
-- one, and the call is tried once more with a fresh token.
local function request(method, url, body, extra)
  for attempt = 1, 2 do
    local headers = {
      ["User-Agent"] = USER_AGENT,
      ["Authorization"] = "Bearer " .. access_token(),
      ["Accept"] = "application/json",
    }
    for k, v in pairs(extra or {}) do headers[k] = v end
    local opts = {method = method, url = url, headers = headers, timeout = 30}
    if body then
      headers["Content-Type"] = "application/json"
      opts.body = rela.json.encode(body)
    end
    local resp, err = http.request(opts)
    if not resp then
      error("basecamp: " .. method .. " failed (" .. err.kind .. "): " .. err.message, 0)
    end
    if resp.status_code ~= 401 or attempt == 2 then
      return resp
    end
    rela.oauth.invalidate(SYSTEM, token)
    token = nil
  end
end

-- check raises on anything but a success. 429 and 503 carry the wait
-- Basecamp asked for.
local function check(resp, what)
  local code = resp.status_code
  if code == 429 or code == 503 then
    error(RATE_LIMITED .. "; retry after " .. tostring(resp.retry_after or 0) .. " s", 0)
  end
  if code < 200 or code > 299 then
    error("basecamp: " .. what .. " answered " .. resp.status, 0)
  end
end

local function decode(resp, what)
  local v, err = rela.json.decode(resp.body)
  if err then error("basecamp: " .. what .. " sent a body that is not JSON", 0) end
  return v
end

-- next_link returns the next page from the Link header. It refuses a link
-- that leaves the account, so a bad header cannot send the token elsewhere.
local function next_link(resp)
  local link = resp.headers["link"]
  if not link then return nil end
  local url = link:match('<([^>]+)>%s*;%s*rel="?next"?')
  if not url then return nil end
  if url:sub(1, #ROOT + 1) ~= ROOT .. "/" then
    error("basecamp: refusing a next-page link outside " .. ROOT, 0)
  end
  return url
end

-- get_page reads one page of a list. An ETag from an earlier run makes an
-- unchanged page a cheap 304. The next-page link is read from the 304 when
-- it carries one: a page can stay the same while a new page is added after
-- it, and the cached link would then stop the listing early.
local function get_page(url)
  local key = "page:" .. url
  local cached = rela.cache.get(key)
  local extra = {}
  if cached then extra["If-None-Match"] = cached.etag end
  local resp = request("GET", url, nil, extra)
  if resp.status_code == 304 and cached then
    local nxt = cached.next
    if resp.headers["link"] then nxt = next_link(resp) end
    return rela.json.decode(cached.body), nxt
  end
  check(resp, "listing to-dos")
  local items = decode(resp, "listing to-dos")
  local nxt = next_link(resp)
  if resp.headers["etag"] then
    rela.cache.set(key, {etag = resp.headers["etag"], body = resp.body, next = nxt}, {ttl = 86400})
  end
  return items, nxt
end

local function todo_url(id) return ROOT .. "/todos/" .. id .. ".json" end

-- The ref url of a to-do ends in a fragment naming the to-do list it was
-- synced from. The vanished check reads it, so pointing
-- basecamp_todolist_id at another list does not flag the todos of the old
-- one. A browser ignores the fragment.
local LIST_MARK = "#todolist-" .. LIST

local function list_ref_url(remote)
  if not remote.app_url then return nil end
  return remote.app_url .. LIST_MARK
end

local function in_this_list(ref)
  local url = ref.url or ""
  return url:sub(-#LIST_MARK) == LIST_MARK
end

-- get_todo returns the to-do, or nil when Basecamp no longer has it.
local function get_todo(id)
  local resp = request("GET", todo_url(id))
  if resp.status_code == 404 then return nil end
  check(resp, "reading to-do " .. id)
  return decode(resp, "reading to-do " .. id)
end

local function sid(n)
  if type(n) == "number" then return string.format("%d", n) end
  return tostring(n)
end

local function ids(people)
  local out = {}
  for _, p in ipairs(people or {}) do table.insert(out, p.id) end
  return out
end

-- ---------------------------------------------------------------------------
-- Mapping

local EMPTY = rela.sync.EMPTY

local function theirs_of(r)
  return {
    properties = {
      title = r.content,
      due = r.due_on or EMPTY,
      done = r.completed == true,
    },
    content = r.description or "",
  }
end

-- as_theirs presents an entity in the shape of a Basecamp report, so a merge
-- against it compares two rela states.
local function as_theirs(e)
  local p = {}
  for _, f in ipairs(FIELDS) do
    if f ~= "content" then
      local v = e.properties[f]
      if v == nil then v = EMPTY end
      p[f] = v
    end
  end
  return {properties = p, content = e.content or ""}
end

local function value(v)
  if v == EMPTY then return nil end
  return v
end

-- put_remote writes our side of a merge to Basecamp. A PUT replaces the
-- whole to-do, and Basecamp clears any field the PUT leaves out, so it
-- re-reads the to-do first, sends back every field rela does not own, and
-- stops if the to-do changed since the merge read it. Returns false when it
-- changed: the next pull merges again.
local function put_remote(remote, push)
  local fresh = get_todo(sid(remote.id))
  if not fresh or fresh.updated_at ~= remote.updated_at then return false end
  if push.title ~= nil or push.due ~= nil or push.content ~= nil then
    local function pick(f, theirs)
      if push[f] ~= nil then return value(push[f]) end
      return theirs
    end
    local body = {
      content = pick("title", fresh.content),
      description = pick("content", fresh.description),
      due_on = pick("due", fresh.due_on),
      starts_on = fresh.starts_on,
      notify = false,
    }
    -- An empty Lua table encodes as a JSON object, so an empty list is left
    -- out; leaving it out clears it, which is the same.
    local assignees = ids(fresh.assignees)
    if #assignees > 0 then body.assignee_ids = assignees end
    local subscribers = ids(fresh.completion_subscribers)
    if #subscribers > 0 then body.completion_subscriber_ids = subscribers end
    check(request("PUT", todo_url(sid(remote.id)), body), "updating to-do " .. sid(remote.id))
  end
  if push.done ~= nil then
    local method = "DELETE"
    if push.done == true then method = "POST" end
    check(request(method, ROOT .. "/todos/" .. sid(remote.id) .. "/completion.json"),
      "changing completion of to-do " .. sid(remote.id))
  end
  return true
end

-- ---------------------------------------------------------------------------
-- Merge

local stats = {created = 0, written = 0, pushed = 0, conflicts = 0}

-- record_conflict notes on the todo why it is not syncing. A person
-- resolves it by editing one side; the next sync clears the note.
local function record_conflict(id, ours, text)
  stats.conflicts = stats.conflicts + 1
  if ours.properties.sync_conflict ~= text then
    rela.update_entity(id, {sync_conflict = text})
  end
end

local function describe(conflicts)
  local parts = {}
  for _, c in ipairs(conflicts) do
    table.insert(parts, c.field .. " changed on both sides")
  end
  return "conflict: " .. table.concat(parts, "; ")
end

-- sync_once makes one attempt for one to-do. Returns "done" or "retry".
local function sync_once(id, remote)
  local base = rela.version_by_tag(id, TAG)
  local read_tok = rela.version_token(id)
  local ours = rela.get_entity(id)
  local m = rela.sync.merge(base, ours, theirs_of(remote), {fields = FIELDS})
  if #m.conflicts > 0 then
    record_conflict(id, ours, describe(m.conflicts))
    return "done" -- leave the base where it is
  end

  local write = {}
  for k, v in pairs(m.write) do write[k] = v end
  if ours.properties.sync_conflict then write.sync_conflict = EMPTY end
  local tok = read_tok
  if next(write) or m.content then
    local w, _, t = rela.update_entity(id, write, m.content, {expect = read_tok})
    if not w then return "retry" end -- edited since we read it
    tok = t
    stats.written = stats.written + 1
  end
  if next(m.push) then
    if not put_remote(remote, m.push) then return "done" end
    stats.pushed = stats.pushed + 1
  end
  -- Move the base only after a complete report, and only with the token of
  -- what was written or read.
  if m.complete and (next(write) or m.content or next(m.push) or m.retag) then
    if not tok then return "done" end
    if not rela.tag_version(id, TAG, {expect = tok}) then return "retry" end
  end
  return "done"
end

local function sync_item(id, remote)
  for _ = 1, 3 do
    if sync_once(id, remote) == "done" then return end
  end
  rela.output(id .. ": still changing after 3 attempts; the next run merges it")
end

-- ---------------------------------------------------------------------------
-- Pull

local function pull_one(remote)
  local rid = sid(remote.id)
  local e = rela.find_by_external_ref(SYSTEM, rid)
  if not e then
    local ok, created, _, tok = pcall(rela.create_entity, TYPE, {
      title = remote.content,
      due = remote.due_on,
      done = remote.completed == true,
      [SYSTEM] = {id = rid, url = list_ref_url(remote)},
    }, remote.description or "", nil, {token = true})
    if not ok then
      -- Most often "must be unique": an entity this connector cannot see
      -- holds the id. Retrying would fail the same way.
      rela.output("basecamp " .. rid .. ": " .. tostring(created))
      return
    end
    stats.created = stats.created + 1
    if tok and rela.tag_version(created.id, TAG, {expect = tok}) then return end
    e = created -- an automation rewrote it; merge it with no base
  end
  sync_item(e.id, remote)
end

local function pull()
  local remotes, seen = {}, {}
  local ok, err = pcall(function()
    for _, query in ipairs({"", "?completed=true"}) do
      local url = ROOT .. "/todolists/" .. LIST .. "/todos.json" .. query
      while url do
        local items, nxt = get_page(url)
        for _, r in ipairs(items) do table.insert(remotes, r) end
        url = nxt
      end
    end
  end)
  if not ok then
    if tostring(err):find(RATE_LIMITED, 1, true) then
      -- Nothing was merged yet, so no tag moved. The next run starts over.
      rela.output(tostring(err) .. "; this run merged nothing")
      return
    end
    error(err, 0)
  end

  for _, r in ipairs(remotes) do
    seen[sid(r.id)] = true
    pull_one(r)
  end
  for _, e in ipairs(rela.list_entities(TYPE)) do
    local ref = e.properties[SYSTEM]
    if ref and not seen[ref.id] and in_this_list(ref) then
      local ours = rela.get_entity(e.id)
      if ours then record_conflict(e.id, ours, VANISHED) end
    end
  end
  rela.output(string.format("basecamp: %d to-dos; created %d, updated %d, pushed %d, conflicts %d",
    #remotes, stats.created, stats.written, stats.pushed, stats.conflicts))
end

-- ---------------------------------------------------------------------------
-- Push

-- create_remote creates the Basecamp to-do for a todo made in rela.
--
-- The todo is linked right after the POST, before any call that can raise.
-- A failed run is retried, and the retry must find the link; otherwise it
-- would create a second to-do. A retry that finds the link but no base
-- stops (see push), and the next pull merges the todo. With no base, that
-- pull records any field that still differs, such as a completion that did
-- not reach Basecamp, as a conflict.
local function create_remote(e)
  local read_tok = rela.version_token(e.id)
  local body = {content = e.properties.title or e.id, description = e.content or "",
    due_on = e.properties.due, notify = false}
  local resp = request("POST", ROOT .. "/todolists/" .. LIST .. "/todos.json", body)
  check(resp, "creating a to-do")
  local remote = decode(resp, "creating a to-do")
  local rid = sid(remote.id)
  local ref = {[SYSTEM] = {id = rid, url = list_ref_url(remote)}}
  local w, _, tok = rela.update_entity(e.id, ref, nil, {expect = read_tok})
  if not w then
    -- Edited while we created it. Link it anyway and leave the merge to
    -- the next pull.
    rela.update_entity(e.id, ref)
  end
  if e.properties.done then
    check(request("POST", ROOT .. "/todos/" .. rid .. "/completion.json"), "completing to-do " .. rid)
  end
  if w and tok then rela.tag_version(e.id, TAG, {expect = tok}) end
end

local function push(e)
  if not e.properties[SYSTEM] then return create_remote(e) end
  local base = rela.version_by_tag(e.id, TAG)
  -- A pulled to-do gets its first base from the pull.
  if not base then return end
  local ours = rela.get_entity(e.id)
  if not ours then return end
  -- Compare with the base first: a save that changed none of the synced
  -- fields, such as a pull's own write, makes no HTTP call.
  local local_change = rela.sync.merge(base, ours, as_theirs(base), {fields = FIELDS})
  if not next(local_change.push) then return end
  local remote = get_todo(ours.properties[SYSTEM].id)
  if not remote then return record_conflict(e.id, ours, VANISHED) end
  sync_item(e.id, remote)
end

if entity then
  push(entity)
else
  pull()
end
