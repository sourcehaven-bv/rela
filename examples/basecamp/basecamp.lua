-- Basecamp to-do sync for rela. See README.md for the setup.
--
-- One file holds both directions, because a rela script cannot load
-- another file:
--
--   * Run as a scheduled task (schedules.yaml), it PULLS: it mirrors every
--     Basecamp project and to-do list the account can see, reads every
--     to-do, and merges each into its rela todo.
--   * Run as a background automation (schema.yaml), the global `entity` is
--     set, and it PUSHES that one todo's local edits to Basecamp.
--
-- Both run as integration:basecamp and remember the last agreed state of
-- each todo as the version tag sync/basecamp. The merge rules are the ones
-- in the "Sync connectors" section of the Lua scripting guide.

local SYSTEM = "basecamp"
local LIST_SYSTEM = "basecamp-list"
local PROJECT_SYSTEM = "basecamp-project"
local TAG = "sync/basecamp"
local TYPE = "todo"
local IN_LIST, IN_PROJECT = "in-list", "in-project"
local FIELDS = {"title", "due", "done", "content"}

-- Basecamp refuses a request whose User-Agent names no contact. Put your
-- application's name and a contact address here.
local USER_AGENT = "rela-basecamp-example (ops@example.com)"

local RATE_LIMITED = "basecamp: rate limited"
local VANISHED = "vanished: Basecamp no longer lists this to-do (deleted or archived)"
local NO_LIST = "no to-do list: link this todo to a to-do list with in-list, then save it again"
local RICH = "body: the Basecamp description holds attachments, mentions or images that rela " ..
  "cannot keep; edit the body in Basecamp"
local LOCAL_RICH = "body: holds images, raw HTML or links that Basecamp cannot take; remove them to sync"

-- Settings. They are secrets only so the test can point the API at a stub;
-- none of them is sensitive.
local S = rela.secrets
local ACCOUNT = S.basecamp_account_id or ""
-- Optional: the list a todo made in rela goes to when it has no in-list
-- relation.
local DEFAULT_LIST = S.basecamp_todolist_id or ""
if not ACCOUNT:match("^%d+$") then
  error("basecamp: set the secret basecamp_account_id to a Basecamp account id", 0)
end
if DEFAULT_LIST ~= "" and not DEFAULT_LIST:match("^%d+$") then
  error("basecamp: basecamp_todolist_id, when set, must be a Basecamp id", 0)
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
  check(resp, "listing " .. url)
  local items = decode(resp, "listing " .. url)
  local nxt = next_link(resp)
  if resp.headers["etag"] then
    rela.cache.set(key, {etag = resp.headers["etag"], body = resp.body, next = nxt}, {ttl = 86400})
  end
  return items, nxt
end

-- list_all reads every page of a listing.
local function list_all(url)
  local out = {}
  while url do
    local items, nxt = get_page(url)
    for _, r in ipairs(items) do table.insert(out, r) end
    url = nxt
  end
  return out
end

-- recordings lists every active recording of a type across all projects the
-- account can see. Oldest first, so a new recording lands on the last page
-- and the earlier pages stay cached.
local function recordings(kind)
  return list_all(ROOT .. "/projects/recordings.json?type=" .. kind .. "&sort=created_at&direction=asc")
end

local function todo_url(id) return ROOT .. "/todos/" .. id .. ".json" end

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

-- Basecamp stores rich text as HTML and rela stores markdown. The
-- conversions settle after one round trip, so a merge does too.
--
-- convert runs one of them. It returns the text and whether something was
-- dropped, or nil and the reason when the body cannot be converted at all;
-- one bad body must not stop the run.
local function convert(fn, text)
  local ok, out, lossy = pcall(fn, text or "")
  if not ok then return nil, "body: " .. tostring(out) end
  return out, lossy
end

local function body_of(r) return convert(rela.md.from_html, r.description) end

-- html_of renders a rela body for Basecamp, or returns nil and the reason it
-- cannot go: pushing a body Basecamp cannot hold would delete the dropped
-- part in rela on the next pull.
local function html_of(md)
  local html, lossy = convert(rela.md.to_html, md)
  if not html then return nil, lossy end
  if lossy then return nil, LOCAL_RICH end
  return html
end

local function theirs_of(r)
  return {
    properties = {
      title = r.content,
      due = r.due_on or EMPTY,
      done = r.completed == true,
    },
    -- A body that cannot be converted is not reported, so the merge
    -- leaves it alone.
    content = (body_of(r)),
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
    local description = fresh.description
    if push.content ~= nil then description = assert(html_of(value(push.content))) end
    local body = {
      content = pick("title", fresh.content),
      description = description,
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
    if m.push.content ~= nil then
      local _, remote_lossy = body_of(remote)
      local _, why = html_of(value(m.push.content))
      if remote_lossy then why = RICH end
      if why then
        record_conflict(id, ours, why)
        return "done"
      end
    end
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

-- links maps each todo or to-do list to the target of its one outgoing
-- relation of rtype, read in one query.
local function links(rtype)
  local out = {}
  for _, r in ipairs(rela.get_relations({type = rtype})) do out[r.from] = r.to end
  return out
end

-- link_one makes `to` the only target of from's rtype relation.
local function link_one(current, from, rtype, to)
  if current[from] == to then return end
  for _, r in ipairs(rela.get_relations({from = from, type = rtype})) do
    rela.delete_relation(from, rtype, r.to)
  end
  rela.create_relation(from, rtype, to)
  current[from] = to
end

-- mirror creates or renames the rela entity for a Basecamp project or to-do
-- list and returns its id. Basecamp owns these, so a title edited in rela
-- is overwritten.
local function mirror(typ, system, rid, title, url)
  local e = rela.find_by_external_ref(system, rid)
  if not e then
    return rela.create_entity(typ, {title = title, [SYSTEM] = {id = rid, url = url}}).id
  end
  if e.properties.title ~= title then rela.update_entity(e.id, {title = title}) end
  return e.id
end

local function pull_one(remote, list_ids, in_list)
  local rid = sid(remote.id)
  local e = rela.find_by_external_ref(SYSTEM, rid)
  local settled = false
  if not e then
    local ok, created, _, tok = pcall(rela.create_entity, TYPE, {
      title = remote.content,
      due = remote.due_on,
      done = remote.completed == true,
      [SYSTEM] = {id = rid, url = remote.app_url},
    }, body_of(remote) or "", nil, {token = true})
    if not ok then
      -- Most often "must be unique": an entity this connector cannot see
      -- holds the id. Retrying would fail the same way.
      rela.output("basecamp " .. rid .. ": " .. tostring(created))
      return
    end
    stats.created = stats.created + 1
    e = created
    -- When the tag fails, an automation rewrote the todo: merge it with no
    -- base below.
    settled = tok and rela.tag_version(created.id, TAG, {expect = tok})
  end
  -- Basecamp owns the list a to-do is in, so a move in rela is undone here.
  local list = remote.parent and list_ids[sid(remote.parent.id)]
  if list then link_one(in_list, e.id, IN_LIST, list) end
  if not settled then sync_item(e.id, remote) end
end

local function pull()
  local lists, remotes
  local ok, err = pcall(function()
    lists = recordings("Todolist")
    remotes = recordings("Todo")
  end)
  if not ok then
    if tostring(err):find(RATE_LIMITED, 1, true) then
      -- Nothing was merged yet, so no tag moved. The next run starts over.
      rela.output(tostring(err) .. "; this run merged nothing")
      return
    end
    error(err, 0)
  end

  -- Projects and to-do lists first, so each todo can be linked to its list.
  local project_ids, list_ids = {}, {}
  local in_project, in_list = links(IN_PROJECT), links(IN_LIST)
  for _, l in ipairs(lists) do
    local b = l.bucket or {}
    local pid = sid(b.id or "")
    if pid ~= "" and not project_ids[pid] then
      project_ids[pid] = mirror("project", PROJECT_SYSTEM, pid, b.name or pid, nil)
    end
    local lid = mirror("todolist", LIST_SYSTEM, sid(l.id), l.title or l.name or sid(l.id), l.app_url)
    list_ids[sid(l.id)] = lid
    if project_ids[pid] then link_one(in_project, lid, IN_PROJECT, project_ids[pid]) end
  end

  local seen = {}
  for _, r in ipairs(remotes) do
    seen[sid(r.id)] = true
    pull_one(r, list_ids, in_list)
  end
  for _, e in ipairs(rela.list_entities(TYPE)) do
    local ref = e.properties[SYSTEM]
    if ref and not seen[ref.id] then
      local ours = rela.get_entity(e.id)
      if ours then record_conflict(e.id, ours, VANISHED) end
    end
  end
  rela.output(string.format("basecamp: %d lists, %d to-dos; created %d, updated %d, pushed %d, conflicts %d",
    #lists, #remotes, stats.created, stats.written, stats.pushed, stats.conflicts))
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
-- target_list returns the Basecamp id of the list a new todo goes to: the
-- list its in-list relation names, else basecamp_todolist_id.
local function target_list(e)
  for _, r in ipairs(rela.get_relations({from = e.id, type = IN_LIST})) do
    local l = rela.get_entity(r.to)
    local ref = l and l.properties[SYSTEM]
    if ref then return ref.id end
  end
  if DEFAULT_LIST ~= "" then return DEFAULT_LIST end
  return nil
end

local function create_remote(e)
  local list = target_list(e)
  if not list then return record_conflict(e.id, e, NO_LIST) end
  local description, why = html_of(e.content)
  if not description then return record_conflict(e.id, e, why) end
  local read_tok = rela.version_token(e.id)
  local body = {content = e.properties.title or e.id, description = description,
    due_on = e.properties.due, notify = false}
  local resp = request("POST", ROOT .. "/todolists/" .. list .. "/todos.json", body)
  check(resp, "creating a to-do")
  local remote = decode(resp, "creating a to-do")
  local rid = sid(remote.id)
  local ref = {[SYSTEM] = {id = rid, url = remote.app_url}}
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
