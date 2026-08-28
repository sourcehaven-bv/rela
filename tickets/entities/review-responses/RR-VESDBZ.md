---
id: RR-VESDBZ
type: review-response
title: Kanban's v-if="column.icon" makes the fallback unreachable, so surfaces disagree on bad input
finding: |-
    KanbanView guards the icon element with `v-if="column.icon"`. A truthy-but-unknown name therefore renders the generic FileText fallback, while an EMPTY name renders nothing. Sidebar has no guard, so it always renders a fallback.

    So the two surfaces behave differently for the same bad input, and the reviewer argues the kanban fallback is arguably worse than nothing: an author who typo'd sees a document glyph beside 'Done' rather than the absence that would prompt a config check.
severity: minor
reason: |-
    The divergence is real but correct, because the two surfaces have genuinely different defaults.

    A nav entry ALWAYS has an icon (derived from its kind), so rendering a fallback there preserves the layout — dropping it would shift every label left. A kanban column has NO icon by default, so `v-if` is what makes an unset icon render nothing rather than a stray document glyph beside every column header in every unmigrated project. Removing the guard would put a FileText next to every column of every board that has never heard of icons.

    The reviewer's sharper point — that a typo showing a generic glyph is worse than showing nothing — is fair, but the server rejects unknown names at load, so this is only reachable via a hand-crafted API response or a config that predates the name. In that state a visible-but-wrong icon and no icon are about equally diagnostic, and neither is silent.

    Not worth adding an isKnownIcon() call to the template to unify behaviour for a state the validator already prevents. Recording the reasoning so it reads as a decision rather than an oversight.
status: wont-fix
---
