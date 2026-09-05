---
id: RR-KBD2T2
type: review-response
title: 'Under NopACL every history read emitted a reveal record'
finding: |-
    Under NopACL and ReadOnlyACL no middleware attaches a read gate, so
    readGateFromContext hands back nopReadGate, whose HoldsPermission returns true
    for EVERY permission (readgate.go:135 -- the RR-CWWJGW fail-open shape). The
    first implementation therefore took the reveal arm on EVERY history read in an
    unconfigured deployment and emitted a history-reveal row for each one.

    Those reads reveal nothing: with no policy configured nothing is redacted, so
    the "reveal" is an artifact of the permit-all gate rather than a privileged
    disclosure. Reproduced with a failing test before fixing: a NopACL app serving
    one history read produced 1 record where it must produce 0.

    This is worse than a cosmetic false positive. It would fill the audit log of
    every unconfigured deployment with meaningless history-reveal rows, and train an
    operator who LATER configures a policy to ignore exactly the row this ticket
    exists to make visible -- defeating the control while appearing to satisfy it.
severity: critical
resolution: |-
    Gated the emit on `revealIsPrivileged(a.acl)`: a closed switch on the ACL
    IMPLEMENTATION, matching the existing `permitsGatedUIElement`
    (views_handler.go:318). NopACL / ReadOnlyACL / nil => no policy => not a
    privileged disclosure => no record. Anything else audits.

    Deliberately NOT a read-gate query: the read gate is the thing that cannot
    answer here, so consulting it is precisely the fail-open mistake being avoided.
    Value and pointer forms are both matched because these types' methods have value
    receivers -- matching only the value form would drop `&acl.NopACL{}` into the
    default arm.

    The default arm AUDITS. For a log that is the conservative direction: a spurious
    row can be filtered later, a missing one is gone. This inverts
    permitsGatedUIElement's default, which hides -- correctly, since that one is
    deciding disclosure and this one is deciding whether to write evidence.

    Also fixed a latent inaccuracy in the test harness that this exposed:
    `buildPolicyApp` built a Declarative resolver but left `app.acl` as NopACL, so a
    test handing it a policy modelled a configured deployment for field redaction
    and an unconfigured one for anything switching on the ACL implementation. It now
    installs the policy as `app.acl` too.

    Pinned by `TestHistoryReveal_NoACL_NotAudited`, mutation-verified in both
    directions: flipping the Nop/ReadOnly arm to `true` reddens that test alone;
    flipping the default arm to `false` reddens the two positive tests alone.
status: addressed
---

## Resolution

Gated the emit on `revealIsPrivileged(a.acl)`: a closed switch on the ACL
IMPLEMENTATION, matching the existing `permitsGatedUIElement`
(views_handler.go:318). NopACL / ReadOnlyACL / nil => no policy => not a
privileged disclosure => no record. Anything else audits.

Deliberately NOT a read-gate query: the read gate is the thing that cannot
answer here, so consulting it is precisely the fail-open mistake being avoided.
Value and pointer forms are both matched because these types' methods have value
receivers -- matching only the value form would drop `&acl.NopACL{}` into the
default arm.

The default arm AUDITS. For a log that is the conservative direction: a spurious
row can be filtered later, a missing one is gone. This inverts
permitsGatedUIElement's default, which hides -- correctly, since that one is
deciding disclosure and this one is deciding whether to write evidence.

Also fixed a latent inaccuracy in the test harness that this exposed:
`buildPolicyApp` built a Declarative resolver but left `app.acl` as NopACL, so a
test handing it a policy modelled a configured deployment for field redaction
and an unconfigured one for anything switching on the ACL implementation. It now
installs the policy as `app.acl` too.

Pinned by `TestHistoryReveal_NoACL_NotAudited`, mutation-verified in both
directions: flipping the Nop/ReadOnly arm to `true` reddens that test alone;
flipping the default arm to `false` reddens the two positive tests alone.
