-- The ticket and bug checklist workflow of tickets/flows.yaml, end to end.
--
-- Each case runs on a fresh in-memory copy of this project's config. A case
-- named "<trigger>: ..." covers that trigger; internal/apptest's
-- TestTicketsTriggersCovered fails when a trigger has no such case.
-- Run with: rela --project=tickets test --require-tests
local t = require("rela.test")

local function ticket(a, status)
  return (a:create("ticket", {title = "Fix the thing", kind = "chore", status = status}))
end

local function bug(a, status)
  return (a:create("bug", {title = "It breaks", status = status}))
end

local function move(a, e, status)
  return (a:update(e, {status = status}))
end

-- only returns the one target of e's rel relation.
local function only(a, e, rel)
  local got = a:related(e, rel)
  t.expect(#got).to_be(1)
  return got[1]
end

-- expect_checklist checks a checklist a trigger created.
local function expect_checklist(c, typ, title)
  t.expect(c.type).to_be(typ)
  t.expect(c.properties.title).to_be(title)
  t.expect(c.properties.status).to_be("in-progress")
  t.expect(c.properties.started).to_be(t.today())
end

-- Ticket checklists

t.case("ticket-planning-checklist: planning creates the planning checklist", function()
  local a = t.as("agent")
  local e = ticket(a, "ready")
  move(a, e, "planning")
  expect_checklist(only(a, e, "has-planning"), "planning-checklist", "Planning: Fix the thing")
end)

t.case("ticket-implementation-checklist: in-progress creates the implementation checklist", function()
  local a = t.as("agent")
  local e = ticket(a, "ready")
  move(a, e, "planning")
  move(a, e, "in-progress")
  expect_checklist(only(a, e, "has-implementation"), "implementation-checklist",
    "Implementation: Fix the thing")
end)

t.case("ticket-review-checklist: review creates the review checklist", function()
  local a = t.as("agent")
  local e = ticket(a, "in-progress")
  move(a, e, "review")
  expect_checklist(only(a, e, "has-review"), "review-checklist", "Review: Fix the thing")
end)

-- Bug checklists

t.case("bug-analysis-checklist: analyzing creates the analysis checklist", function()
  local a = t.as("agent")
  local e = bug(a, "ready")
  move(a, e, "analyzing")
  expect_checklist(only(a, e, "has-bug-analysis"), "bug-analysis-checklist", "Analysis: It breaks")
end)

t.case("bug-implementation-checklist: in-progress creates the implementation checklist", function()
  local a = t.as("agent")
  local e = bug(a, "analyzing")
  move(a, e, "in-progress")
  expect_checklist(only(a, e, "has-implementation"), "implementation-checklist",
    "Implementation: It breaks")
end)

t.case("bug-review-checklist: review creates the review checklist", function()
  local a = t.as("agent")
  local e = bug(a, "in-progress")
  move(a, e, "review")
  expect_checklist(only(a, e, "has-review"), "review-checklist", "Review: It breaks")
end)

-- Lifecycle dates

t.case("ticket-started-planning: planning sets started once", function()
  local a = t.as("agent")
  local e = ticket(a, "ready")
  local first = t.today()
  t.expect(move(a, e, "planning").properties.started).to_be(first)
  t.advance(t.days(3))
  move(a, e, "in-progress")
  move(a, e, "review")
  move(a, e, "planning")
  t.expect(a:get(e).properties.started).to_be(first)
end)

t.case("ticket-started-in-progress: in-progress sets started when planning was skipped", function()
  local a = t.as("agent")
  local e = ticket(a, "ready")
  t.expect(move(a, e, "in-progress").properties.started).to_be(t.today())
end)

t.case("bug-started-analyzing: analyzing sets started once", function()
  local a = t.as("agent")
  local e = bug(a, "ready")
  local first = t.today()
  t.expect(move(a, e, "analyzing").properties.started).to_be(first)
  t.advance(t.days(2))
  move(a, e, "in-progress")
  t.expect(a:get(e).properties.started).to_be(first)
end)

t.case("bug-started-in-progress: in-progress sets started when analysis was skipped", function()
  local a = t.as("agent")
  local e = bug(a, "ready")
  t.expect(move(a, e, "in-progress").properties.started).to_be(t.today())
end)

t.case("work-completed: done sets completed on tickets and bugs", function()
  local a = t.as("agent")
  local tk = ticket(a, "review")
  local bg = bug(a, "review")
  t.advance(t.days(1))
  t.expect(move(a, tk, "done").properties.completed).to_be(t.today())
  t.expect(move(a, bg, "done").properties.completed).to_be(t.today())
end)

t.case("docs-checklist-started: in-progress sets started on a docs checklist", function()
  local a = t.as("agent")
  local d = a:create("docs-checklist", {title = "Docs: Fix the thing", status = "pending"})
  t.expect(move(a, d, "in-progress").properties.started).to_be(t.today())
end)

t.case("checklist-completed: done sets completed on every checklist type", function()
  local a = t.as("agent")
  for _, typ in ipairs({"planning-checklist", "bug-analysis-checklist", "implementation-checklist",
      "review-checklist", "docs-checklist"}) do
    local c = a:create(typ, {title = "A checklist", status = "in-progress"})
    t.expect(move(a, c, "done").properties.completed).to_be(t.today())
  end
end)

-- Edge cases

t.case("edge: re-entering a status creates no second checklist", function()
  local a = t.as("agent")
  local e = ticket(a, "ready")
  move(a, e, "planning")
  move(a, e, "in-progress")
  move(a, e, "review")
  move(a, e, "in-progress")
  move(a, e, "review")
  move(a, e, "planning")
  t.expect(#a:related(e, "has-planning")).to_be(1)
  t.expect(#a:related(e, "has-implementation")).to_be(1)
  t.expect(#a:related(e, "has-review")).to_be(1)
  t.expect(#a:list("planning-checklist")).to_be(1)
  t.expect(#a:list("implementation-checklist")).to_be(1)
  t.expect(#a:list("review-checklist")).to_be(1)
end)

t.case("edge: ready, blocked and wont-fix create nothing", function()
  local a = t.as("agent")
  local tk = ticket(a, "backlog")
  local bg = bug(a, "backlog")
  for _, s in ipairs({"ready", "blocked", "wont-fix"}) do
    move(a, tk, s)
    move(a, bg, s)
  end
  t.expect(a:get(tk).properties.started).to_be_nil()
  t.expect(a:get(tk).properties.completed).to_be_nil()
  t.expect(a:get(bg).properties.started).to_be_nil()
  t.expect(a:get(bg).properties.completed).to_be_nil()
  for _, typ in ipairs({"planning-checklist", "bug-analysis-checklist", "implementation-checklist",
      "review-checklist", "docs-checklist"}) do
    t.expect(#a:list(typ)).to_be(0)
  end
end)

-- A status trigger fires on a change, not on create. CLAUDE.md tells agents
-- to create in backlog or ready and then move the status.
t.case("edge: creating in a working status fires no trigger", function()
  local a = t.as("agent")
  local tk = ticket(a, "planning")
  local bg = bug(a, "in-progress")
  local d = a:create("docs-checklist", {title = "Docs", status = "in-progress"})
  t.expect(#a:related(tk, "has-planning")).to_be(0)
  t.expect(#a:related(bg, "has-implementation")).to_be(0)
  t.expect(a:get(tk).properties.started).to_be_nil()
  t.expect(a:get(bg).properties.started).to_be_nil()
  t.expect(a:get(d).properties.started).to_be_nil()
end)

-- The workflow gates are validation rules without enforce: write, so the
-- write that skips one is accepted. `rela validate` in CI and the analyze
-- gates enforce them, not the write.
t.case("edge: a move past an unfinished checklist is accepted", function()
  local a = t.as("agent")
  local e = ticket(a, "ready")
  move(a, e, "planning")
  t.expect(move(a, e, "done").properties.status).to_be("done")
end)

t.case("edge: no docs checklist is created for an enhancement", function()
  local a = t.as("agent")
  local e = a:create("ticket", {title = "Add a thing", kind = "enhancement", status = "ready"})
  for _, s in ipairs({"planning", "in-progress", "review", "done"}) do
    move(a, e, s)
  end
  t.expect(#a:related(e, "has-docs")).to_be(0)
  t.expect(#a:list("docs-checklist")).to_be(0)
end)

t.case("edge: re-entering a bug status creates no second checklist", function()
  local a = t.as("agent")
  local e = bug(a, "ready")
  move(a, e, "analyzing")
  move(a, e, "in-progress")
  move(a, e, "review")
  move(a, e, "in-progress")
  move(a, e, "analyzing")
  move(a, e, "review")
  t.expect(#a:list("bug-analysis-checklist")).to_be(1)
  t.expect(#a:list("implementation-checklist")).to_be(1)
  t.expect(#a:list("review-checklist")).to_be(1)
end)
