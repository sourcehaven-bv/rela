-- icinga-alert.lua — receive a monitoring alert and fold it into an incident.
--
-- The motivating use case for request-scoped actions (TKT-EFMRQM): Icinga POSTs
-- a JSON alert, this script finds or creates the matching `incident`, appends
-- the notification to its markdown body, and answers with a status Icinga can
-- act on.
--
-- Wire it up in data-entry.yaml:
--
--   actions:
--     icinga-alert:
--       script: icinga-alert.lua
--       request:
--         body: true
--         headers: [X-Icinga-Event]
--
-- Then POST to /api/v1/_action/icinga-alert.
--
-- Everything Icinga-shaped lives HERE — the payload field names, the state
-- vocabulary, the incident title format. rela's Go core contains no
-- Icinga-specific code; point it at a different monitoring system by editing
-- this file.
--
-- This is deliberately the ESCAPE HATCH. If your mapping is a plain
-- find-or-create-and-append, a declarative `webhooks:` route (docs/webhooks.md)
-- expresses it in config with no Lua at all — reach for that first.

if rela.request == nil then
  -- The action is missing its `request:` block, so nothing arrived. Fail loudly
  -- rather than inventing an incident from empty fields.
  return {
    status = 500,
    body = "icinga-alert: action is not request-scoped (add a request: block)",
    content_type = "text/plain",
  }
end

local alert = rela.request.body
if alert == nil then
  -- Not JSON. rela.request.raw still holds the bytes if you need to look.
  return { status = 400, body = "expected a JSON body", content_type = "text/plain" }
end

local host = alert.host or alert.host_name
local service = alert.service or alert.service_name or ""
local state = alert.state or "UNKNOWN"
local output = alert.output or ""

if host == nil or host == "" then
  -- 422, not 400: the request was well-formed, it just cannot produce a valid
  -- entity. A sender that distinguishes the two will not retry this forever.
  return { status = 422, body = "alert has no host", content_type = "text/plain" }
end

-- The match key. Keep this stable across deliveries for the same underlying
-- problem, or every alert mints a duplicate incident.
local key = host
if service ~= "" then key = host .. "!" .. service end

-- IDEMPOTENCY IS THIS SCRIPT'S JOB. rela offers no dedup key on this endpoint,
-- and a producer that retries will invoke this twice. Matching on the key
-- before appending is what stops a retry writing the notification twice — the
-- quiet failure mode, since a doubled incident body looks plausible.
local incident
for _, e in ipairs(rela.list_entities("incident", { filter = "status=open" })) do
  if e.properties.alert_key == key then
    incident = e
    break
  end
end

local line = string.format("- %s %s: %s", rela.today, state, output)

if incident == nil then
  local created = rela.create_entity("incident", {
    title = "Monitoring: " .. key,
    alert_key = key,
    status = "open",
  }, "## Notifications\n\n" .. line .. "\n")

  return {
    status = 201,
    body = rela.json.encode({ status = "created", incident = created.id }),
    content_type = "application/json",
  }
end

-- Append rather than replace: the body is the running notification log.
rela.update_entity(incident.id, {}, (incident.content or "") .. line .. "\n")

return {
  status = 202,
  body = rela.json.encode({ status = "appended", incident = incident.id }),
  content_type = "application/json",
}
