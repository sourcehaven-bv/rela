---
id: RR-IO6DHEP
type: review-response
title: Validate test does not pin which migration fired
finding: 'validate_test.go asserted only IsMigrationError, so an unrelated detection would keep it green; no negative case for a view section with link: detail.'
severity: minor
resolution: 'The test asserts the error mentions link: detail and adds a view-section link: detail case that must not be a migration error.'
status: addressed
---
