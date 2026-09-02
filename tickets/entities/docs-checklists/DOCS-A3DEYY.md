---
id: DOCS-A3DEYY
type: docs-checklist
title: 'Docs: Operator recipient allowlist for mail.send'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

Three comments carry weight beyond describing what the code does:

- **`RecipientPolicyCarrier`** explains why it is an optional interface rather
than a widened `MailSender`: the wiring site injects a value and this package
asks it a question it may not be able to answer. It also states that a sender
NOT implementing it denies everything — the unimplemented case and the
unconfigured case are the same fact, so they get the same answer.
- **`LuaSender.RecipientPolicy`** records why the policy travels with the
SENDER rather than being passed separately into the runtime: the sender is
already built from `mail.yaml`, so a second wiring step would be a second thing
to forget, and forgetting it fails OPEN unless the default is deny.
- **The domain comparison** in `permits` names the bypass it prevents. A suffix
test would let `evil-example.com` match `*@example.com`, and that is not obvious
from reading a `HasSuffix` call.

`isDomainPattern` documents why a partial wildcard is refused rather than
implemented: every extra wildcard position is another way to write a pattern
admitting more than the operator pictured.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: the optional-interface
pattern follows `NotFoundError` in the same package)
- [x] docs/ updated for changed behaviour — see below
- [x] ~~Architecture docs updated~~ (N/A: the design was chosen to RESPECT the
existing arch-lint boundary rather than change it — `internal/mail` still may
not reach `filter` or `store`)

`GUIDE-mail.md` gains a "Who may receive mail" section under the
sending-from-scripts material, where a reader already is when they care.

It corrects a claim while it is there. The guide previously said a script
"cannot reach a destination you did not set up" — true of the TRANSPORT, but
read naturally it implies a bound on recipients that did not exist. It now says
"and only to a recipient you allowed", which is the same sentence made true.

## External Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLI reference updated~~ (N/A: no command or flag)
- [x] ~~API docs updated~~ (N/A: no HTTP surface change)

## Rationale for N/A

The user-visible change is a config key, documented where the config is.

The part that HAD to be documented, and the reason this section is not simply
"new key, new docs": **an absent `recipients:` block denies everything, which
inverts the rest of the file.** An absent `mail.yaml` means mail is off; an
absent `port` means 587. An operator who has internalised that will read the
first denial as a bug in rela rather than as their own missing block.

So the guide states the inversion and its reasoning outright — permitting on
absence fails silently and irreversibly, because mail leaves the ACL perimeter
and nobody finds out until the recipient replies; refusing fails loudly and
harmlessly. A control whose unconfigured state is "allow" is not a control.

Also documented deliberately: that `allow_any: true` must be a written line,
never inferred. The point is that it stays greppable in a config review, which
is only true if operators know to write it rather than discovering that an empty
block happens to do the same thing.

Deliberately NOT documented: that the query form is coming. A "not yet
implemented" note in an operator guide invites waiting for it, and the domain
form is the right answer for most deployments regardless.
