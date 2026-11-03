---
id: RR-QDXE61
type: review-response
title: Missing acceptance criteria and tests
finding: 'Not covered: timeout during module load; pcall-caught module error then retry; require in a detached (mail) runtime; rela.cache from a module required by inline code; deep nested require chains.'
severity: minor
resolution: 'Added tests: timeout during module load, pcall-caught module error then retry, detached runtime, cache namespace from a module, deep require chain at the call-stack limit. rela.cache from a module required by inline code is documented to raise like inline code.'
status: addressed
---
