-- sync-github-issues.lua
-- Imports open GitHub issues from sourcehaven-bv/rela into the rela ticket system.
--
-- Idempotency: each imported ticket contains a <!-- github-issue:NUMBER --> marker
-- in its content. Re-running the script skips issues that already have a matching ticket.
--
-- Usage:
--   rela lua scripts/sync-github-issues.lua
--
-- The GitHub API is public (no auth needed for public repos), but rate-limited
-- to 60 requests/hour for unauthenticated requests.

local REPO = "sourcehaven-bv/rela"
local API = "https://api.github.com/repos/" .. REPO .. "/issues"
local MARKER_PREFIX = "<!-- github-issue:"

-- Fetch open issues from GitHub (excludes PRs).
local function fetch_issues()
    local page = 1
    local all = {}
    while true do
        local url = API .. "?state=open&per_page=100&page=" .. page
        local resp, err = http.get(url, {
            headers = {
                Accept = "application/vnd.github+json",
                ["User-Agent"] = "rela-sync-script",
            },
            timeout = 15,
        })
        if err then
            rela.output({error = "GitHub API error: " .. err.kind .. ": " .. err.message})
            return nil
        end
        if resp.status_code ~= 200 then
            rela.output({error = "GitHub API returned " .. resp.status_code, body = resp.body})
            return nil
        end
        local issues, jerr = http.json_decode(resp.body)
        if jerr then
            rela.output({error = "Failed to parse response: " .. jerr.message})
            return nil
        end
        if #issues == 0 then break end
        for _, issue in ipairs(issues) do
            -- GitHub API returns PRs in the issues endpoint; skip them.
            if not issue.pull_request then
                table.insert(all, issue)
            end
        end
        page = page + 1
    end
    return all
end

-- Build a set of GitHub issue numbers that already have rela tickets.
local function find_existing_markers()
    local existing = {}
    local tickets = rela.list_entities("ticket")
    for _, t in ipairs(tickets) do
        local content = t.content or ""
        -- Pattern-escape: < and - are special in Lua patterns.
        local num = content:match("<!%-%- github%-issue:(%d+)")
        if num then
            existing[tonumber(num)] = t.id
        end
    end
    return existing
end

-- Map GitHub labels to rela ticket kind.
local LABEL_TO_KIND = {
    bug = "bug",
    enhancement = "enhancement",
    documentation = "docs",
    docs = "docs",
    refactor = "refactor",
}

local function classify_kind(labels)
    for _, label in ipairs(labels) do
        local kind = LABEL_TO_KIND[(label.name or ""):lower()]
        if kind then return kind end
    end
    return "enhancement"
end

-- Build markdown content from a GitHub issue.
local function build_content(issue)
    local parts = {}
    table.insert(parts, MARKER_PREFIX .. issue.number .. " -->")
    table.insert(parts, "")
    table.insert(parts, "Imported from [#" .. issue.number .. "](" .. issue.html_url .. ")")
    table.insert(parts, "")
    if issue.body and issue.body ~= "" then
        table.insert(parts, issue.body)
    end
    return table.concat(parts, "\n")
end

-- Main
local issues = fetch_issues()
if not issues then return end

local existing = find_existing_markers()
local created = 0
local skipped = 0

for _, issue in ipairs(issues) do
    if existing[issue.number] then
        skipped = skipped + 1
    else
        local kind = classify_kind(issue.labels or {})
        local content = build_content(issue)
        rela.create_entity("ticket", {
            title = issue.title,
            kind = kind,
            status = "backlog",
            priority = "medium",
        }, content)
        created = created + 1
    end
end

rela.output({
    source = REPO,
    fetched = #issues,
    created = created,
    skipped = skipped,
})
