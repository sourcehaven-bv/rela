#!/usr/bin/env -S rela flow
-- Stale Ticket Review Flow
-- Reviews tickets and bugs that haven't been updated in N days
-- Usage: rela flow scripts/stale-review.lua [days]

local days_threshold = tonumber(rela.args[1]) or 14

-- Collect stale items (tickets and bugs not in terminal states)
local function collect_stale_items()
    local stale = {}

    -- Get tickets not in terminal states
    local tickets = rela.list_entities("ticket")
    for _, t in ipairs(tickets) do
        local status = t:prop("status", "")
        if status ~= "done" and status ~= "wont-fix" then
            local days = rela.days_since(t.mod_time)
            if days >= days_threshold then
                table.insert(stale, {
                    entity = t,
                    days = days,
                    kind = "ticket"
                })
            end
        end
    end

    -- Get bugs not in terminal states
    local bugs = rela.list_entities("bug")
    for _, b in ipairs(bugs) do
        local status = b:prop("status", "")
        if status ~= "done" and status ~= "wont-fix" then
            local days = rela.days_since(b.mod_time)
            if days >= days_threshold then
                table.insert(stale, {
                    entity = b,
                    days = days,
                    kind = "bug"
                })
            end
        end
    end

    -- Sort by staleness (most stale first)
    table.sort(stale, function(a, b) return a.days > b.days end)

    return stale
end

local stale_items = collect_stale_items()

if #stale_items == 0 then
    rela.output({
        message = "No stale items found",
        threshold_days = days_threshold
    })
    return
end

-- Summary screen
local summary_lines = {
    "Found **" .. #stale_items .. "** items not updated in " .. days_threshold .. "+ days.\n",
    "| ID | Type | Status | Days | Title |",
    "|:---|:-----|:-------|-----:|:------|",
}
for _, item in ipairs(stale_items) do
    local e = item.entity
    table.insert(summary_lines, string.format(
        "| %s | %s | %s | %d | %s |",
        e.id,
        item.kind,
        e:prop("status", "?"),
        item.days,
        e:prop("title", "(no title)")
    ))
end

local summary_event = rela.flow.emit({
    type = "form",
    title = "Stale Item Review",
    fields = {
        {type = "markdown", content = table.concat(summary_lines, "\n")},
    },
    actions = {
        {"review", "Review One by One"},
        {"skip", "Skip Review"},
    },
})

if summary_event.action == "skip" then
    rela.output({skipped = true, count = #stale_items})
    return
end

-- Review each item
local reviewed = 0
local updated = 0
local closed = 0

for i, item in ipairs(stale_items) do
    local e = item.entity
    local progress = string.format("**%d of %d** - %d days stale", i, #stale_items, item.days)

    -- Build status options based on type
    local status_options
    if item.kind == "ticket" then
        status_options = {
            {"backlog", "Backlog"},
            {"ready", "Ready"},
            {"planning", "Planning"},
            {"in-progress", "In Progress"},
            {"review", "Review"},
            {"done", "Done"},
            {"wont-fix", "Won't Fix"},
            {"blocked", "Blocked"},
        }
    else -- bug
        status_options = {
            {"backlog", "Backlog"},
            {"ready", "Ready"},
            {"analyzing", "Analyzing"},
            {"in-progress", "In Progress"},
            {"review", "Review"},
            {"done", "Done"},
            {"wont-fix", "Won't Fix"},
            {"blocked", "Blocked"},
        }
    end

    -- Build content preview (first 200 chars)
    local content_preview = e.content or ""
    if #content_preview > 200 then
        content_preview = content_preview:sub(1, 200) .. "..."
    end

    local detail_md = string.format([[
%s

---

### %s: %s

**Status:** %s | **Priority:** %s | **Effort:** %s

%s
]],
        progress,
        e.id,
        e:prop("title", "(no title)"),
        e:prop("status", "?"),
        e:prop("priority", "-"),
        e:prop("effort", "-"),
        content_preview
    )

    local event = rela.flow.emit({
        type = "form",
        title = "Review: " .. e.id,
        fields = {
            {type = "markdown", content = detail_md},
            {name = "action", type = "select", label = "Action", options = {
                {"keep", "Keep as-is (touch to update mod time)"},
                {"update", "Update status"},
                {"close", "Close (done/won't-fix)"},
                {"skip", "Skip this item"},
            }},
            {name = "new_status", type = "select", label = "New Status", options = status_options, default = e:prop("status", "backlog")},
            {name = "note", type = "text", label = "Note (optional)", placeholder = "Add a comment to the description...", lines = 2},
        },
        actions = {
            {"apply", "Apply"},
            {"finish", "Finish Review"},
        },
    })

    if event.action == "finish" then
        break
    end

    reviewed = reviewed + 1
    local action = event.data.action

    if action == "skip" then
        -- Do nothing
    elseif action == "keep" then
        -- Touch the entity to update mod time (update with same values)
        rela.update_entity(e.id, {})
        updated = updated + 1
    elseif action == "update" then
        local props = {status = event.data.new_status}
        local content = e.content
        if event.data.note and event.data.note ~= "" then
            content = content .. "\n\n**Note (" .. rela.today .. "):** " .. event.data.note
        end
        rela.update_entity(e.id, props, content)
        updated = updated + 1
    elseif action == "close" then
        local new_status = event.data.new_status
        if new_status ~= "done" and new_status ~= "wont-fix" then
            new_status = "done" -- Default to done for close action
        end
        local props = {status = new_status}
        local content = e.content
        if event.data.note and event.data.note ~= "" then
            content = content .. "\n\n**Note (" .. rela.today .. "):** " .. event.data.note
        end
        rela.update_entity(e.id, props, content)
        updated = updated + 1
        closed = closed + 1
    end
end

rela.output({
    total_stale = #stale_items,
    reviewed = reviewed,
    updated = updated,
    closed = closed,
    threshold_days = days_threshold,
})
