-- Send via Resend's emails API.
--
-- JSON with a bearer token and lowercase field names — the closest of the
-- three to the built-in `transport: http`, and still incompatible with it:
-- `from` is a string, not an object, and the body parts are `html`/`text`
-- rather than `html_content`/`text_content`.
--
-- .rela/mail.yaml:
--   transport: script
--   script: mail/resend.lua
--   from: notifications@example.com
--   capabilities:
--     http: true
--     secrets: [resend_key]

local key = rela.secrets.resend_key
if not key then
  error("resend: set resend_key in .rela/secrets.yaml and list it under " ..
        "capabilities.secrets in mail.yaml")
end

local base = rela.secrets.resend_base_url or "https://api.resend.com"

local function addr(a)
  if a.name and a.name ~= "" then
    return string.format("%s <%s>", a.name, a.email)
  end
  return a.email
end

local to = {}
for _, rcpt in ipairs(message.to) do
  table.insert(to, addr(rcpt))
end

local payload = {
  from = addr(message.from),
  to = to,
  subject = message.subject,
}
if message.html and message.html ~= "" then payload.html = message.html end
if message.text and message.text ~= "" then payload.text = message.text end

local resp, err = http.request({
  url = base .. "/emails",
  method = "POST",
  headers = {
    ["Authorization"] = "Bearer " .. key,
    ["Content-Type"] = "application/json",
  },
  body = rela.json.encode(payload),
})

if err then
  error("resend: " .. err.kind .. ": " .. err.message)
end

if resp.status_code < 200 or resp.status_code >= 300 then
  error(string.format("resend: HTTP %d: %s", resp.status_code, resp.body))
end
