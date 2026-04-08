---
id: RR-LQ1R
type: review-response
title: Threat model dismissively framed; script-level exfiltration via legitimate provider is a real new risk
finding: 'Plan says ''anyone running an untrusted Lua script in their rela project is already compromised'' to dismiss the network-egress addition. This is half right and dangerously framed. Before AI: malicious script can damage your local project (contained). After AI: malicious script can silently exfiltrate every entity to *your own* legitimate provider via ai.chat({messages = {{role=''user'', content=entire_project_dump}}}). The data lands in your provider''s logs (potentially in training data, billing logs, or readable by junior staff). The script needs no malicious config, no filesystem write, no separate compromise. This is a genuine new risk class. Fix: rewrite the security section to acknowledge script-level data exfiltration via the user''s own provider as a distinct threat. Document it honestly in CLAUDE.md and the ai-integration concept. Consider whether `rela script` should print a one-time warning the first time AI is invoked from a script (deferred to a follow-up if the warning needs UX design).'
severity: significant
resolution: 'Security section completely rewritten. New ''Script-level data exfiltration (NEW threat class)'' subsection explicitly distinguishes the before-state (contained-to-local damage) from the after-state (silent data egress to user''s own legitimate provider, lands in provider logs / training data / billing logs). CLAUDE.md and the ai-integration concept will document this honestly as part of the implementation. Mitigations listed as deferred: one-time warning, allowlist, per-script budgets, audit log.'
status: addressed
---
