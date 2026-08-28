-- Send via Mailgun's messages API.
--
-- Mailgun is the reason `transport: script` exists rather than a JSON
-- field-mapping DSL: it accepts multipart/form-data with HTTP Basic auth
-- (user "api", password the API key) and no JSON at all, so any mapping
-- layer built around JSON field names would have excluded it by construction.
--
-- .rela/mail.yaml:
--   transport: script
--   script: mail/mailgun.lua
--   from: notifications@example.com
--   capabilities:
--     http: true
--     secrets: [mailgun_key, mailgun_domain]
--
-- .rela/secrets.yaml:
--   mailgun_key: key-...
--   mailgun_domain: mg.example.com
--
-- MAILGUN_BASE_URL is honored so rela's own tests can point this at a local
-- stub. In production leave it unset and the EU/US endpoint below applies.

local key = rela.secrets.mailgun_key
local domain = rela.secrets.mailgun_domain
if not key or not domain then
  error("mailgun: set mailgun_key and mailgun_domain in .rela/secrets.yaml, " ..
        "and list them under capabilities.secrets in mail.yaml")
end

local base = rela.secrets.mailgun_base_url or "https://api.mailgun.net/v3"

-- from as a single "Name <addr>" string: Mailgun has no separate name field.
local from = message.from.email
if message.from.name and message.from.name ~= "" then
  from = string.format("%s <%s>", message.from.name, message.from.email)
end

-- Repeated `to` fields, one per recipient. This is why http's `form` accepts
-- the positional {name, value} shape as well as a map: a Lua table keyed by
-- name cannot express "to" three times.
local form = {
  { "from", from },
  { "subject", message.subject },
}
for _, rcpt in ipairs(message.to) do
  local addr = rcpt.email
  if rcpt.name and rcpt.name ~= "" then
    addr = string.format("%s <%s>", rcpt.name, rcpt.email)
  end
  table.insert(form, { "to", addr })
end
if message.html and message.html ~= "" then
  table.insert(form, { "html", message.html })
end
if message.text and message.text ~= "" then
  table.insert(form, { "text", message.text })
end

local resp, err = http.request({
  url = base .. "/" .. domain .. "/messages",
  method = "POST",
  form = form,
  basic_auth = { user = "api", pass = key },
})

if err then
  -- Raise: the outbox treats a failed script as a failed send and retries it
  -- through the existing backoff ladder.
  error("mailgun: " .. err.kind .. ": " .. err.message)
end

if resp.status_code < 200 or resp.status_code >= 300 then
  error(string.format("mailgun: HTTP %d: %s", resp.status_code, resp.body))
end
