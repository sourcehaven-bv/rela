---
id: RR-20PB
type: review-response
title: MarkdownEditor toolbar button typed via dual cast — hides legitimate EasyMDE option-type mismatches
finding: |-
    `MarkdownEditor.vue:65`:
    ```ts
    ] as EasyMDE.Options['toolbar'],
    ```

    This cast asserts the array satisfies EasyMDE's toolbar type. The object form for the custom 'entity-ref' button is missing fields some EasyMDE versions accept (e.g. `noDisable`, `noMobile`, `default`). The cast also tells TypeScript to STOP type-checking the rest of the array elements (the strings 'bold', 'italic', etc. are now under the cast umbrella too). If EasyMDE's type definitions tighten in a future version, the entire toolbar config will pass type-check while breaking at runtime.

    The right shape is either:
      - Type the array WITHOUT the cast and let TypeScript verify each element. If EasyMDE's exported toolbar union type rejects the custom-button object, file an issue upstream or use a tighter local interface that matches what EasyMDE actually expects.
      - If a cast is unavoidable, narrow it to JUST the custom-button object: `{ name: 'entity-ref', action: () => { ... }, className: 'fa fa-at', title: 'Insert entity reference' } as EasyMDE.ToolbarButton` (or whatever the exported name is) and leave the surrounding array typed by inference.

    Minor severity — the runtime works today — but the cast undermines the value of having typecheck in the first place.
severity: nit
resolution: 'Replaced the `as EasyMDE.Options[''toolbar'']` array-wide cast with a typed `const entityRefButton: EasyMDE.ToolbarIcon = {...}` declaration referenced from inside the toolbar array. The surrounding string entries now stay under EasyMDE''s typed union and the whole array type-checks per element.'
status: addressed
---
