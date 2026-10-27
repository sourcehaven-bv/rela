---
id: RR-5JWQNM
type: review-response
title: Unrelated Prettier reformatting and strict ratchet will conflict with open PRs
finding: About 15 existing files were reformatted beyond the needed lines. The two-way ratchet fails any open PR that changes a bare-string count.
severity: significant
resolution: Reverted all reformatting; the diff on existing files now holds only the semantic change. The two-way ratchet is kept on purpose, and the PR body says that concurrent PRs adding bare strings must use t() or update the count.
status: addressed
---
