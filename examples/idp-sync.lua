-- idp-sync.lua — provision a person entity from an identity proxy on a webhook.
--
-- Invoked by rela's inbound-IdP webhook receiver (POST /webhooks/idp) after it
-- has VERIFIED the webhook's signed JWT. This action does the "fetch details"
-- half of a thin-notify + fetch flow: the webhook told us WHICH user changed;
-- here we fetch that user's authoritative data from the IdP's API and upsert a
-- `person` entity keyed on the stable subject (`sub`).
--
-- This file is the PROVIDER-SPECIFIC layer. Everything Pratique-shaped lives
-- here — the operator-API path, the HMAC canonical string, the X-Pratique-*
-- header names — assembled from rela's GENERIC crypto.* + http.* primitives.
-- Point rela at a different IdP by editing this script (and the secrets); no
-- rela rebuild. rela's Go core contains no Pratique-specific code.
--
-- Params (set by the webhook receiver from the verified JWT claims):
--   rela.params.user_id  -- the subject the event concerns (the `sub`)
--   rela.params.org_id   -- the tenant the event concerns
--   rela.params.event    -- e.g. "membership.created"
--
-- Results: a `message_type = "error"` result makes the receiver answer 502,
-- so the IdP retries the webhook; the stored roles stay as they were.
-- An event for a member who is gone or not active is acknowledged (no error)
-- with `asserted_roles = {}`, which clears the stored roles.
--
-- Secrets (.rela/secrets.yaml):
--   idp_operator_url  -- base URL of the operator API, e.g. https://id.example.com
--   idp_operator_key  -- the shared HMAC secret the operator API verifies
--   idp_org_id        -- the IdP org this deployment serves

local base = rela.secrets["idp_operator_url"]
local key = rela.secrets["idp_operator_key"]
local want_org = rela.secrets["idp_org_id"]
local uid = rela.params["user_id"]
local org = rela.params["org_id"]
local event = rela.params["event"]

-- rela.secrets and rela.params leave a missing key out, so treat nil as empty.
if (base or "") == "" or (key or "") == "" or (want_org or "") == "" then
  return { message_type = "error", message = "idp-sync: operator url/key/org not configured in secrets" }
end
if (uid or "") == "" or (org or "") == "" then
  return { message_type = "error", message = "idp-sync: missing user_id/org_id param" }
end

-- The IdP sends every org's membership events to every webhook endpoint, and
-- rela stores roles per subject, not per org. Without this check, a user
-- deactivated here but active in another org would get that org's roles back
-- from its next event. An event for another org says nothing about this one,
-- so it neither grants nor clears.
if org ~= want_org then
  return { message = "idp-sync: event for another org; ignored" }
end

-- Validate the identifiers before interpolating them into a request path and an
-- entity query below.
--
-- They arrive from a cryptographically verified webhook JWT, so this is
-- defence in depth rather than the primary control — but it is the cheap half
-- of it. A compromised or misconfigured IdP that emits a `sub` containing `/`,
-- `?`, `#` or a newline would otherwise reshape the outbound operator-API path
-- or the entity filter (rela#1083). An allowlist, not a blocklist: enumerate
-- what an identifier may contain rather than guessing what it may not.
--
-- The set covers the usual shapes: UUIDs, ULIDs, emails, and slugs. Some IdPs
-- issue subjects it rejects — Auth0's `auth0|abc123` is the common one — so
-- widen the pattern if yours does. Widen it deliberately, character by
-- character, and never to `.*`: the point is that a `/` or a newline in a
-- subject cannot reshape the request path built below.
local function valid_id(v)
  return v:match("^[%w._@%-]+$") ~= nil
end

if not valid_id(uid) or not valid_id(org) then
  return {
    message_type = "error",
    message = "idp-sync: user_id/org_id contains characters outside the allowed set",
  }
end

-- Build the Pratique operator-API request path and its HMAC signature. The
-- canonical string + header names are Pratique's wire format (see its
-- docs/04-architecture.md §operator API):
--   canonical = METHOD \n PATH \n DATE \n hex(sha256(body))
--   signature = base64( HMAC-SHA256(secret, canonical) )
--
-- Pratique's operator API has no GET for a single member, only the org's
-- member list (GET /api/v1/orgs/{id}/members) and DELETE on one member. So
-- fetch the list and pick the subject out of it.
local path = "/api/v1/orgs/" .. org .. "/members"
-- rela.now_unix is the current time as unix seconds (the sandbox has no
-- os.time()). string.format("%d", ...) renders it as an integer with no decimal
-- point / exponent, matching the server's integer date parse.
local date = string.format("%d", rela.now_unix)
local body = "" -- GET, no body
local canonical = "GET" .. "\n" .. path .. "\n" .. date .. "\n" .. crypto.sha256_hex(body)
local sig = crypto.hmac_sha256_base64(key, canonical)

-- clear acknowledges the event and clears any roles stored for the subject.
-- It does not create or change a person: an existing one stays as it was.
local function clear(reason)
  return {
    message = "idp-sync: user " .. reason .. "; roles cleared",
    asserted_roles = {},
  }
end

-- fail answers an error, so the receiver answers 502, the IdP retries, and the
-- stored roles stay. A membership.deleted event already proves the membership
-- is gone, so for that event a failed fetch clears instead: revocation must
-- not depend on the operator API being reachable.
local function fail(msg)
  if event == "membership.deleted" then
    return clear("membership deleted (" .. msg .. ")")
  end
  return { message_type = "error", message = "idp-sync: " .. msg }
end

local resp, err = http.get(base .. path, {
  headers = {
    ["X-Pratique-Date"] = date,
    ["X-Pratique-Signature"] = sig,
    ["Accept"] = "application/json",
  },
})
if err ~= nil then
  return fail("fetch failed: " .. (err.message or "unknown"))
end
-- Pratique answers 200 with an empty list for an unknown org, so any other
-- status, 404 included, means the request did not reach the member list (a
-- wrong base URL, a proxy, a changed route). That is an error, not "not a
-- member".
if resp.status_code ~= 200 then
  return fail("fetch returned " .. tostring(resp.status_code))
end

local list, derr = rela.json.decode(resp.body)
if derr ~= nil or type(list) ~= "table" or type(list.members) ~= "table" then
  return fail("bad JSON from operator API")
end

-- Each member is { user_id, email, roles, status }. A `members` value that is
-- an object rather than a list decodes to a table too; ipairs then finds no
-- member and the roles are cleared, which is the safe direction.
local user = nil
for _, m in ipairs(list.members) do
  if type(m) == "table" and m.user_id == uid then
    user = m
    break
  end
end
if user == nil then
  -- The membership was removed (membership.deleted), possibly before an
  -- earlier event for it was processed.
  return clear("is not a member")
end

-- Pratique offboards softly: a deactivated member stays in the list with
-- status "inactive" and keeps its roles. Keep roles only for an active member.
-- Any other status, including a missing one, clears them, so an unknown state
-- fails closed.
if user.status ~= "active" then
  local shown = "unknown"
  if user.status == "inactive" or user.status == "pending" then
    shown = user.status
  end
  return clear("status is " .. shown)
end

-- Upsert the person, keyed on the stable subject. Idempotent: a redelivered
-- webhook (or a later membership change) updates the same entity rather than
-- duplicating it.
local props = {
  sub = uid,
  email = user.email or "",
}

-- asserted_roles stores the IdP role names for the verified subject, so a
-- scheduled for_each task running as this person holds the rela roles
-- asserted_role_assignments maps them to. The receiver keys them by the
-- webhook's subject; the script cannot name another one. An empty list clears.
local roles = {}
for _, r in ipairs(user.roles or {}) do
  if type(r) == "string" then
    table.insert(roles, r)
  end
end

local found = rela.list_entities("person", "sub=" .. uid)
if found ~= nil and #found > 0 then
  rela.update_entity(found[1].id, props)
  return { message = "idp-sync: updated person " .. uid, asserted_roles = roles }
end

rela.create_entity("person", props)
return { message = "idp-sync: created person " .. uid, asserted_roles = roles }
