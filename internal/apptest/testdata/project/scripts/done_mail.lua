local to = entity.properties.note or "ok@example.com"
local _, err = mail.send{to = to, subject = "done " .. entity.id, text = "finished"}
if err then error("mail " .. err.kind) end
