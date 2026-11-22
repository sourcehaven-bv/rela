---
id: BUG-RO4WSP1
type: bug
title: Read-only workspace wording and create form
description: A read-only tenant says "The tenant is read-only", the setting-up screen says "organization", and the new-record form on a read-only tenant shows no fields.
priority: medium
effort: s
why1: The server message and the provisioning text named the tenant and the organisation. The create form's catch for a failed dry-run marked the field filter ready with an empty visible set, so every field was hidden.
why2: A read-only tenant refuses every non-read request, and the create form's dry-run is a POST, so on a read-only tenant the dry-run always fails.
why3: The fail-open path was written for a rare failure and set the same flag as a success, which made it fail closed on fields. No test made the dry-run fail.
why4: The read-only mode was built server-side, and no SPA screen was checked against it.
why5: Customer-facing text was written in the operator's terms, and a new server mode had no check of the screens it affects.
prevention: Vitest tests serve the dry-run refusal through the real API client and pin that the fields stay, the notice shows and nothing is posted. A unit test pins when the read-only flag is set and cleared. The Go read-only test pins the workspace wording.
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description

Testing the tenant API showed three problems with a read-only tenant and a new
tenant.

- The toast for a refused write says "The tenant is read-only". Customers know
  a tenant as a workspace.
- The setting-up screen says "Setting up your organization", in US spelling.
- While a tenant is read-only, the new-record form shows no Title field and no
  other fields.

## Acceptance Criteria

- [x] A refused write says "This workspace is read-only", in English and Dutch.
- [x] The setting-up screen says "workspace" or "werkruimte", with no US spelling.
- [x] On a read-only workspace the create form keeps its fields, says the workspace is read-only, and offers no Create button.
- [x] The behaviour is documented.
