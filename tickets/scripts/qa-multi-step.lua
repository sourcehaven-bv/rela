#!/usr/bin/env -S rela flow
-- QA Test: Multi-Step Flow
-- Tests navigation between multiple forms

local data = {}

-- Step 1
local e1 = rela.flow.emit({
    type = "form",
    title = "Step 1 of 3: Basic Info",
    fields = {
        {type = "markdown", content = "Enter basic information to continue."},
        {name = "name", type = "text", label = "Name", required = true},
        {name = "email", type = "text", label = "Email", placeholder = "user@example.com"},
    },
    actions = {
        {"next", "Next →"},
        {"cancel", "Cancel"},
    },
})

if e1.action == "cancel" then
    rela.output({cancelled = true, step = 1})
    return
end
data.name = e1.data.name
data.email = e1.data.email

-- Step 2
local e2 = rela.flow.emit({
    type = "form",
    title = "Step 2 of 3: Preferences",
    fields = {
        {type = "markdown", content = "Hello **" .. data.name .. "**! Choose your preferences:"},
        {name = "theme", type = "select", label = "Theme",
         options = {{"light", "Light"}, {"dark", "Dark"}, {"auto", "Auto"}}},
        {name = "notifications", type = "boolean", label = "Enable Notifications", default = true},
    },
    actions = {
        {"back", "← Back"},
        {"next", "Next →"},
        {"cancel", "Cancel"},
    },
})

if e2.action == "cancel" then
    rela.output({cancelled = true, step = 2})
    return
end
if e2.action == "back" then
    rela.output({back = true, step = 2, note = "Back navigation would restart flow in real implementation"})
    return
end
data.theme = e2.data.theme
data.notifications = e2.data.notifications

-- Step 3: Confirmation
local summary = string.format([[
## Summary

- **Name:** %s
- **Email:** %s
- **Theme:** %s
- **Notifications:** %s

Please confirm to complete.
]], data.name, data.email or "(not provided)", data.theme, tostring(data.notifications))

local e3 = rela.flow.emit({
    type = "form",
    title = "Step 3 of 3: Confirm",
    fields = {
        {type = "markdown", content = summary},
    },
    actions = {
        {"back", "← Back"},
        {"confirm", "Confirm", "primary"},
        {"cancel", "Cancel", "danger"},
    },
})

if e3.action == "cancel" then
    rela.output({cancelled = true, step = 3})
    return
end
if e3.action == "back" then
    rela.output({back = true, step = 3})
    return
end

rela.output({
    completed = true,
    data = data,
})
