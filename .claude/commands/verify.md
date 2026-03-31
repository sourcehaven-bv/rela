<!-- @managed: claude-workflow v1 -->
# Verify Implementation Quality

Run comprehensive quality checks before transitioning to the next workflow phase.
This command is invoked with a ticket/bug ID: $ARGUMENTS

## Step 1: Identify Current Phase

Using `show_entity` from rela-issues-and-design-tickets, determine:

- Current status of the ticket/bug
- What phase we're transitioning FROM

## Step 2: Run Phase-Appropriate Checks

### For transition to `in-progress` (from planning/analyzing):

**Planning Verification:**

1. Check planning checklist exists and is linked
2. Verify planning checklist content has substance:
   - Understanding section has acceptance criteria (not just checkboxes)
   - Approach section lists specific files to modify
   - Risk assessment identifies at least one risk or explicitly states "no risks"

**Syntax Check:**

```bash
# Verify interpolation syntax in planning docs
grep -n '{{.*\..*}}' tickets/entities/*/*/*.md 2>/dev/null | grep -v '{{new\.' | grep -v '{{entity\.' | grep -v '{{today}}'
```

If found, warn: "Non-standard interpolation syntax detected"

### For transition to `review` (from in-progress):

**Automated Checks:**

```bash
just test 2>&1
just lint 2>&1
just coverage-check 2>&1
```

Report pass/fail for each.

**Test Existence Check:**

- For each file modified in this ticket (use git diff), verify corresponding test file exists
- Warn if new code lacks tests

**Code Quality Check:**

- Use the cranky-code-reviewer agent to review the diff
- Create review-response entities for each finding
- Link to ticket via has-review-response

### For transition to `done` (from review):

**Review Response Check:**

1. List all review-response entities linked to this ticket
2. Verify no open critical or significant responses
3. For each addressed response, verify resolution is documented

**Final Verification:**

```bash
just ci
```

## Step 3: Report Results

Output a structured report:

```text
## Verification Report: {TICKET-ID}

### Phase: {from} → {to}

### Automated Checks
- [ ] Tests: PASS/FAIL
- [ ] Lint: PASS/FAIL
- [ ] Coverage: PASS/FAIL

### Planning Quality
- [ ] Acceptance criteria defined: YES/NO
- [ ] Files to modify listed: YES/NO
- [ ] Risks documented: YES/NO

### Code Review Status
- Critical findings: X open, Y addressed
- Significant findings: X open, Y addressed
- Minor/nit findings: X open, Y addressed

### Blockers
{List any issues that would prevent transition}

### Warnings
{List non-blocking concerns}
```

## Step 4: Gate Decision

Based on the report:

- If blockers exist: "Cannot transition. Fix blockers first."
- If warnings exist: "Can transition, but consider addressing warnings."
- If clean: "Ready to transition to {next-phase}."
