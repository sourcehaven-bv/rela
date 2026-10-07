---
id: RR-Z4JQ1K
type: review-response
title: Target may commit .rela/ to git
finding: A project whose .rela/ is ignored by a parent repo has no own .gitignore; copying it to a new directory and running git add would commit secrets and rela.db.
severity: significant
resolution: 'Plan updated: The import ensures the target .gitignore covers .rela/ and says so in the report.'
status: addressed
---
