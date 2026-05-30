---
id: RR-RPC4X
type: review-response
title: :deep() in .form-field styles future widget descendants
finding: Old FieldRenderer scoped CSS used flat selectors (input[type='text']{...}) that matched only direct descendants -- did NOT cross component boundaries. New FieldShell uses .form-field :deep(input[type='text']) which crosses every boundary inside the slot. Today invisible (widgets render raw inputs); future widgets that nest their input one component deeper (date picker control, popover) will silently inherit form-field styling.
severity: significant
resolution: 'Moved all input/textarea/select typography (padding, border, focus glow, disabled, error visuals) from FieldShell.vue into each widget''s own scoped <style>. Each widget reads its own error prop and toggles an is-error class on its element. FieldShell now owns ONLY label/help/error chrome + .form-field layout + checkbox-wrapper layout. No :deep() crosses the slot boundary into widget descendants any more. Result: a future view-mode or composite widget that nests inputs cannot accidentally inherit form-field styling.'
status: addressed
---
