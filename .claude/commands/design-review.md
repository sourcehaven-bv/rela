# Design Review

Review the planning checklist and technical approach BEFORE implementation begins.

## Purpose

Catch design issues early when they're cheap to fix. This review focuses on:
- Security vulnerabilities in the proposed approach
- Missing edge cases and error handling
- Architectural concerns
- Gaps in the plan that would cause problems during implementation

## Instructions

1. **Find the planning checklist** for the current ticket using `rela-issues-and-design-tickets`.

2. **Review the planning checklist** critically, asking:

   **Research:**
   - Was a library search done? Is there an existing solution?
   - Are there similar patterns in the codebase being reused?
   - Were reference implementations consulted?
   - Is this reinventing something that already exists?

   **Security:**
   - Where does input come from? Is it trusted?
   - Is validation allowlist (safe) or blocklist (dangerous)?
   - Could an attacker abuse this feature? (path traversal, injection, auth bypass)
   - Do error messages leak sensitive information?
   - Are there TOCTOU (time-of-check/time-of-use) races?

   **Edge Cases:**
   - What happens with empty/null/missing values?
   - What about boundary values (0, -1, MAX_INT)?
   - Special characters, unicode, null bytes?
   - Concurrent access?
   - Resource exhaustion?

   **Design:**
   - Is the approach the simplest solution?
   - Does it follow existing patterns in the codebase?
   - Are there hidden dependencies or assumptions?
   - Will this be testable?

   **Completeness:**
   - Are acceptance criteria specific and testable?
   - Is the scope clear (what's in/out)?
   - Are alternatives documented with reasoning?

3. **For each finding**, create a `review-response` entity with:
   - `title`: Brief description
   - `finding`: What's wrong or missing in the plan
   - `severity`: `critical` | `significant` | `minor` | `nit`
   - `status`: `open`

4. **Link findings to the ticket** via `has-review-response` relation.

5. **Update the planning checklist** to address findings before implementation.

## Severity Guide for Design Review

| Severity | Examples |
|----------|----------|
| critical | Security vulnerability in approach, fundamentally wrong design |
| significant | Missing important edge case, incomplete validation strategy |
| minor | Could be cleaner, minor gap in plan |
| nit | Documentation/clarity improvements |

## After Design Review

- Address critical and significant findings by updating the plan
- Update review-response status to `addressed`
- Only move to `in-progress` when design is solid
- Implementation should be "mechanical" - no design decisions left

## Example Findings

**Template parameter feature** - what design review would have caught:

1. "Template names come from `{{new.kind}}` which is user-controlled. Plan says 'reject path separators' but doesn't specify allowed characters. **Use allowlist (alphanumeric + hyphen + underscore) instead of blocklist.**" (significant)

2. "Plan doesn't address what happens if template file doesn't exist. Should this error or fall back to default?" (significant)

3. "No mention of null byte handling in template name validation." (minor)
