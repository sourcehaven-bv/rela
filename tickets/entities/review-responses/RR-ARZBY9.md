---
id: RR-ARZBY9
type: review-response
title: Two role=listbox elements misrepresented one selection, with no aria-activedescendant
finding: The highlight is one sequence across both sections, but ARIA saw two independent listboxes, with no `aria-activedescendant` anywhere and no `id` on any option. Focus deliberately stays in the editor, so the rows are never focused and a screen reader user got no announcement at all as the highlight moved. Pre-existing structure that this diff extended rather than introduced, but the keyboard is this menu's primary interaction path.
severity: minor
resolution: 'Restructured to ONE `role=listbox` on the panel with `aria-activedescendant` naming the highlighted option, and the two sections as `role=group` labelled by their section headings. Options carry per-instance ids (`Math.random` prefix) so two editors on a page cannot mint colliding ids, since activedescendant resolves document-wide. The Entities group falls back to `aria-label` when its heading is not rendered, because a dangling idref names nothing. Verified in the browser, not just in markup: activedescendant resolves to a real element, that element is the highlighted row, and the id changes on ArrowDown. Pinned by the e2e test ''the listbox names the active row for assistive tech''. e2e locators moved to `data-section` since the ARIA label now varies.'
status: addressed
---
