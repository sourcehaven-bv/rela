---
id: RR-6K3G8Q
type: review-response
title: The simpler alternative — grant the report's audience a read role in acl.yaml — is never considered, and DEC-O59WM4 says it is the sanctioned route
finding: |-
    The ticket jumps from 'the sales manager sees a partial report' to 'the render must elevate', skipping the option DEC-O59WM4 designates as the normal mechanism: grant the report's audience read access via a role in acl.yaml.

    If the intent is 'the whole sales team may see company-wide sales figures', that IS a read-privilege statement about an identity, and DEC-O59WM4 is explicit that script read privilege 'resolves through identity + acl.yaml, never inline config'. A role granting the sales team read on organisatie/opportunity/abonnement/product gives every one of them a complete, correct report with NO new mechanism, no elevation, and no new config surface — and it composes with delegate-X tamper resistance, group expansion, `rela acl who-can`, and the effective-access map, all of which an inline document flag bypasses.

    docs/acl-security.md:663-669 prefers bypass_acl over 'widening the policy' specifically because it leaves an audit row. But that reasoning is about a SCRIPT needing to read beyond its caller for a bounded, incidental purpose — not about an operator deciding a standing audience should see standing data. Here the widened policy is arguably the honest expression of intent: the sales team really may read this data, and saying so in acl.yaml makes it visible to who-can and the access map, where a document-level flag hides it inside data-entry.yaml.

    The elevation route is only clearly superior when the audience must see AGGREGATES without the underlying rows (e.g. see the company total, but not each client's figures). That is a real and probably common requirement — but the ticket never states it, and it is the only thing that makes the whole mechanism necessary rather than merely convenient. If that IS the requirement, it should be the ticket's stated problem, and it also changes the design: the document must be trusted not to leak the rows it aggregates, which is a property nothing currently enforces.
severity: significant
resolution: |-
    Resolved in favour of elevation: the acl.yaml alternative cannot express the requirement.

    The real requirement (clarified by the operator; the sales report was only an example) is BENCHMARKING ACROSS PRINCIPALS WHO ARE MUTUALLY INVISIBLE. Sales managers each own a client set and are disallowed entirely on the others — not merely denied the figures, but denied the EXISTENCE of those clients ('don't even know they are in the system'). The report benchmarks a manager company-wide, or against the best-performing manager.

    A role in acl.yaml granting read on the other clients would defeat the requirement it is meant to serve: row-level ACL is all-or-nothing per row, so granting enough to COMPUTE the benchmark also grants enough to ENUMERATE the competitors and their revenue. The row-level rule ('a hidden entity is nonexistent') is exactly what must be preserved. There is no role assignment that yields 'may aggregate over, may not see'.

    The output is safe to share BECAUSE the Lua aggregates: what reaches the manager is a derived statistic (company total, or a peer's rank/figure) rather than the rows behind it. The aggregation step IS the confidentiality boundary — which is a stronger claim than the ticket originally made, and relocates the design problem (see RR-LWD8N3: nothing currently enforces that an elevated script only emits aggregates).
status: addressed
---
