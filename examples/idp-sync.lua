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
-- Secrets (.rela/secrets.yaml):
--   idp_operator_url  -- base URL of the operator API, e.g. https://id.example.com
--   idp_operator_key  -- the shared HMAC secret the operator API verifies

local base = rela.secrets["idp_operator_url"]
local key = rela.secrets["idp_operator_key"]
local uid = rela.params["user_id"]
local org = rela.params["org_id"]

if base == "" or key == "" then
  return { message_type = "error", message = "idp-sync: operator url/key not configured in secrets" }
end
if uid == "" or org == "" then
  return { message_type = "error", message = "idp-sync: missing user_id/org_id param" }
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
local path = "/api/v1/orgs/" .. org .. "/members/" .. uid
-- rela.now_unix is the current time as unix seconds (the sandbox has no
-- os.time()). string.format("%d", ...) renders it as an integer with no decimal
-- point / exponent, matching the server's integer date parse.
local date = string.format("%d", rela.now_unix)
local body = "" -- GET, no body
local canonical = "GET" .. "\n" .. path .. "\n" .. date .. "\n" .. crypto.sha256_hex(body)
local sig = crypto.hmac_sha256_base64(key, canonical)

local resp, err = http.get(base .. path, {
  headers = {
    ["X-Pratique-Date"] = date,
    ["X-Pratique-Signature"] = sig,
    ["Accept"] = "application/json",
  },
})
if err ~= nil then
  return { message_type = "error", message = "idp-sync: fetch failed: " .. (err.message or "unknown") }
end
if resp.status_code == 404 then
  -- The membership no longer exists (e.g. removed before we processed the event).
  -- Not an error the IdP should retry: acknowledge without provisioning.
  return { message = "idp-sync: user " .. uid .. " not a member of " .. org .. "; skipped" }
end
if resp.status_code ~= 200 then
  return { message_type = "error", message = "idp-sync: fetch returned " .. tostring(resp.status_code) }
end

local user, derr = rela.json.decode(resp.body)
if derr ~= nil or user == nil then
  return { message_type = "error", message = "idp-sync: bad JSON from operator API" }
end

-- Upsert the person, keyed on the stable subject. Idempotent: a redelivered
-- webhook (or a later membership change) updates the same entity rather than
-- duplicating it. The operator API returns { user_id, email, roles, status }.
local props = {
  sub = user.user_id,
  email = user.email or "",
}

local found = rela.list_entities("person", "sub=" .. uid)
if found ~= nil and #found > 0 then
  rela.update_entity(found[1].id, props)
  return { message = "idp-sync: updated person " .. uid }
end

rela.create_entity("person", props)
return { message = "idp-sync: created person " .. uid }
