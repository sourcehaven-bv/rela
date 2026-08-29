-- Send via Postmark's email API.
--
-- JSON, but with its own vocabulary: a custom auth header rather than a
-- bearer token, capitalized field names, and `From`/`To` as comma-joined
-- strings rather than arrays.
--
-- .rela/mail.yaml:
--   transport: script
--   script: mail/postmark.lua
--   from: notifications@example.com
--   capabilities:
--     http: true
--     secrets: [postmark_token]

local token = rela.secrets.postmark_token
if not token then
  error("postmark: set postmark_token in .rela/secrets.yaml and list it " ..
        "under capabilities.secrets in mail.yaml")
end

local base = rela.secrets.postmark_base_url or "https://api.postmarkapp.com"

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
  From = addr(message.from),
  To = table.concat(to, ","),
  Subject = message.subject,
}
if message.html and message.html ~= "" then payload.HtmlBody = message.html end
if message.text and message.text ~= "" then payload.TextBody = message.text end

local resp, err = http.request({
  url = base .. "/email",
  method = "POST",
  headers = {
    ["X-Postmark-Server-Token"] = token,
    ["Content-Type"] = "application/json",
    ["Accept"] = "application/json",
  },
  body = rela.json.encode(payload),
})

if err then
  error("postmark: " .. err.kind .. ": " .. err.message)
end

if resp.status_code < 200 or resp.status_code >= 300 then
  error(string.format("postmark: HTTP %d: %s", resp.status_code, resp.body))
end
