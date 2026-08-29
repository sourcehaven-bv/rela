---
id: RR-DOCA04
type: review-response
title: identical_to covered only the body channel, not the header oracle
finding: APIResponse had no headers, so identical_to could not see ETag suppression. The existing Go test TestACLGet_ETagSuppressedOnDeny pins that a denied GET must not emit an ETag and must not honor If-None-Match — a replayed ETag returning 304 confirms existence. The ticket sold the verb as expressing a property 'currently pinned only in Go' while expressing about half of it.
severity: significant
resolution: APIResponse carries Header; identical_to compares an allowlist (ETag, Last-Modified) rather than all headers, since Date and Content-Length vary without disclosing anything and comparing them would make the check fail always. Documented, and pinned by a mutant plus a five-case table.
status: addressed
---
