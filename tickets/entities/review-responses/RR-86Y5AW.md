---
id: RR-86Y5AW
type: review-response
title: archplan was installed from a mutable tag
finding: pip install from a git tag follows a tag that can be moved, in jobs that hold write scopes. Raised by both reviewers.
severity: minor
resolution: The justfile pins archplan_commit next to archplan_version. CI installs the commit and checks that archplan --version matches the version. Actions stay on tags, as elsewhere in this repo.
status: addressed
---
