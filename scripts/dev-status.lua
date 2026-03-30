-- dev-status.lua
-- Generate a development status report for rela itself
-- Usage: rela lua scripts/dev-status.lua
--    or: lua_run(path: "dev-status.lua")

local function count_by_status(entities)
    local counts = {}
    for _, e in ipairs(entities) do
        local status = e.properties.status or "unknown"
        counts[status] = (counts[status] or 0) + 1
    end
    return counts
end

local function get_recent(entities, status, limit)
    local filtered = {}
    for _, e in ipairs(entities) do
        if e.properties.status == status then
            table.insert(filtered, {
                id = e.id,
                title = e.properties.title,
                priority = e.properties.priority
            })
        end
    end
    -- Return up to limit items
    local result = {}
    for i = 1, math.min(limit, #filtered) do
        table.insert(result, filtered[i])
    end
    return result
end

-- Collect data
local tickets = rela.list_entities("ticket")
local bugs = rela.list_entities("bug")
local features = rela.list_entities("feature")
local decisions = rela.list_entities("decision")
local ideas = rela.list_entities("idea")

-- Calculate statistics
local ticket_stats = count_by_status(tickets)
local bug_stats = count_by_status(bugs)
local feature_stats = count_by_status(features)

-- Find active work
local in_progress_tickets = get_recent(tickets, "in-progress", 10)
local in_progress_bugs = get_recent(bugs, "in-progress", 5)
local review_items = get_recent(tickets, "review", 10)

-- Find blockers
local blocked_tickets = get_recent(tickets, "blocked", 10)
local blocked_bugs = get_recent(bugs, "blocked", 5)

-- Find ready work (backlog items ready to start)
local ready_tickets = get_recent(tickets, "ready", 10)
local ready_bugs = get_recent(bugs, "ready", 5)

-- Count open review responses
local review_responses = rela.list_entities("review-response")
local open_reviews = {}
local critical_reviews = {}
for _, rr in ipairs(review_responses) do
    if rr.properties.status == "open" then
        table.insert(open_reviews, {
            id = rr.id,
            title = rr.properties.title,
            severity = rr.properties.severity
        })
        if rr.properties.severity == "critical" then
            table.insert(critical_reviews, rr)
        end
    end
end

-- Build report
local report = {
    summary = {
        tickets = {
            total = #tickets,
            by_status = ticket_stats
        },
        bugs = {
            total = #bugs,
            by_status = bug_stats
        },
        features = {
            total = #features,
            by_status = feature_stats
        },
        decisions = #decisions,
        ideas = #ideas
    },
    active_work = {
        in_progress = {
            tickets = in_progress_tickets,
            bugs = in_progress_bugs
        },
        in_review = review_items
    },
    blockers = {
        tickets = blocked_tickets,
        bugs = blocked_bugs
    },
    ready_to_start = {
        tickets = ready_tickets,
        bugs = ready_bugs
    },
    code_review = {
        open_count = #open_reviews,
        critical_count = #critical_reviews,
        open_items = open_reviews
    }
}

-- Add warnings
report.warnings = {}

if #critical_reviews > 0 then
    table.insert(report.warnings, {
        level = "critical",
        message = string.format("%d critical review response(s) need attention", #critical_reviews)
    })
end

if #blocked_tickets + #blocked_bugs > 0 then
    table.insert(report.warnings, {
        level = "warning",
        message = string.format("%d item(s) are blocked", #blocked_tickets + #blocked_bugs)
    })
end

local in_progress_count = #in_progress_tickets + #in_progress_bugs
if in_progress_count > 5 then
    table.insert(report.warnings, {
        level = "info",
        message = string.format("%d items in progress - consider focusing", in_progress_count)
    })
end

rela.output(report)
