---
id: RR-MNHN6U
type: review-response
title: Rename can overwrite a database locked by another process
finding: The planned temp file locks rela.db.import-*.lock, not rela.db.lock; a server started on the target meanwhile would have its database replaced by the rename.
severity: significant
resolution: 'Plan updated: Build the whole target in a sibling staging directory with the database at its final relative path; target must not exist; rename the staging directory, which fails if the target appeared.'
status: addressed
---
