# rela-components

Vue 3 component library for the Atlas Projects interface, catalogued in Storybook.

## Commands

```text
npm run storybook       # component catalogue on :6006
npm run dev             # example app
npm run test            # the stories' interaction tests, in a real browser
npm run build           # typecheck + build the example app
npm run build:lib       # types + bundle for consuming apps, into dist/lib
npm run check           # theme, contrast, safe-area, forwarding, purity and motion guards
npm run build-storybook # static catalogue
```

## Structure

```text
src/styles/tokens.css   design tokens (colour, spacing, type, radius)
src/styles/focus-ring.css  the shared focus ring
src/styles/control.css     the shared input box
src/styles/panel.css       the floating panels (teleported, so global)
src/composables/        overlay stack, focus restore, Tab trap,
                        anchored panel positioning, the message seam
src/types/              shared domain types
src/fixtures/           sample data used by the stories
  bulk.ts               the same shapes at workspace scale, for the
                        at-scale example screens
src/components/
  common/               heading, text, field label, icon, icon button,
                        add button, disclosure, tag, tag list, status dot,
                        status pill, button, button group, confirm
                        dialog, spinner, count, meta item, inline edit
  layout/               app shell, sidebar (+ workspace switcher, badge,
                        group, group header, footer link, nav item),
                        section heading, page header, view tabs,
                        slide panel (+ stack, list row), nav status
  board/                board, column, collapsed column, task card,
                        swimlane board, swimlane, swimlane column
  table/                table, section, row
  task/                 task detail, detail panel, fields, comments
  doc/                  doc editor, doc toolbar
  form/                 field shell, text field, textarea, number field,
                        select, option select, multi select, checkbox,
                        radio group, switch, date field (+ range),
                        file field
  overlay/              modal, drawer, menu (+ item, separator,
                        section), tooltip, command palette
  feedback/             banner, callout, toast (+ host, useToasts),
                        empty state, skeleton, progress bar,
                        activity bar, auto-save indicator, timeline
  data/                 search box, filter chip, pagination, avatar
                        (+ group), accordion, segmented control,
                        keyboard key, bulk action bar, code block
src/pages/Pages.stories.ts     full-page compositions of the five designs
src/pages/Examples.stories.ts  worked screens: a form, a list, settings,
                               inline editing, and the board, table,
                               list-plus-detail and sidebar screens at
                               scale
```

## Consuming this library

`npm run build:lib` writes an ES bundle, one stylesheet and the declarations to
`dist/lib`. Vue, `@milkdown/kit`, the drag-and-drop adapters and Floating UI stay
external, so a consuming app's own copies are the ones used. Two copies of Vue
mean two reactivity systems, and a second `@milkdown/kit` would hand the app a
different ProseMirror `Schema` class whose nodes the editor would reject.

```text
import { RlTable, RlTimeline } from 'rela-components'
import 'rela-components/styles'
```

Types come from `vue-tsc` rather than a dts plugin, so the declarations are
emitted by the same compiler that typechecks the source and the two cannot
disagree. Stories, the example app and the fixtures are excluded from the
published surface.

rela consumes the library from source as an npm workspace
(`frontend/packages/rela-components`). The package exports map every path
under `src/` without the `src/` segment, so rela imports
`rela-components/components/common/RlButton.vue`,
`rela-components/components/feedback/useToasts` and `rela-components/types`.

## Styling

Components use scoped CSS and read from the custom properties in
`src/styles/tokens.css`. Consuming apps import `src/styles/base.css` once;
no build-time CSS tooling is required. Override any token on `:root` to
re-theme the library.

### Dark mode

The library ships both themes. `src/styles/dark.css`, imported by
`base.css`, redeclares the colour tokens; nothing else changes, because every
component reads its colours from those tokens rather than naming them.

It turns on in two ways, and an app can use either or both:

- **The operating system**, through `prefers-color-scheme`. An app that does
  nothing gets dark mode on a dark system.
- **A `.dark` class** on `<html>` or any ancestor, which wins over the OS
  setting. A `.light` class does the reverse, with both absent for `system`.

`RlThemeToggle` is the control for those three states, and `RlSidebar` puts
it in its footer by default, so an app that imports the sidebar gets a
working theme picker without wiring one. Set `theme-toggle: false` to leave
it out, or replace the whole band through the `footer` slot.

It writes the classes itself, because they are the library's contract with
its own stylesheet and an app maintaining a copy of that logic is a place for
the two to drift. Storing the choice stays the app's business: bind
`v-model:theme` to wherever the preference lives. Left unbound the control
keeps the choice for the page, so it works out of the box and survives
nothing.

Three states rather than a switch, because `system` is where a user starts. A
binary toggle has to pick a side on first paint for someone who has never
expressed a preference, and then offers no way back to following the OS.

**An app with its own dark palette has to answer `prefers-color-scheme` too.**
`system` is the absence of both classes, so there is nothing for app CSS to
key off in that state: a stylesheet whose dark rules hang off `:root.dark`
alone stays light on a dark system while every `Rl*` component goes dark.
Half a page theming correctly is harder to spot than none of it. Either
mirror the media query, as `dark.css` does, or have the app resolve the OS in
JavaScript and write `.dark` itself, in which case it owns both classes and
the picker should only report the choice through `v-model:theme`.

Because `.dark` works on any element, a subtree can be themed on its own; it
paints its own background so the page behind does not show through.

Two tokens exist for the dark theme's sake and are worth knowing when adding
a component that floats above the page:

- `--rl-color-bg-raised` is the surface for a modal, drawer, menu panel or
  toast. It equals `--rl-color-bg` in the light theme. In the dark theme it is
  lighter than the page, because a scrim cannot separate two near-black
  surfaces: a modal painted `--rl-color-bg` sits on its own scrimmed page at
  1.05:1.
- `--rl-color-border-raised` and `--rl-color-border-scrimmed` are the edges on
  those surfaces, which do the separating that a black-on-black shadow cannot.
  The scrimmed one is transparent in the light theme, where the design has no
  outline on a modal.

Both themes are checked on every change:

```text
npm run check     # every check below
npm run check:theme      # the two palette blocks agree; no token is missing a dark value
npm run check:contrast   # every foreground/background pairing meets its target
npm run check:safe-area  # no edge carries its inset twice
npm run check:forwarding # a wrapper reaches all of its child's props
npm run check:purity     # components take props and reach for nothing else
npm run check:motion     # durations, easings and stacking come from the tokens
```

`check:forwarding` guards a prop that exists but cannot be reached. Where a
wrapper renders a child and re-exposes its props, adding a prop to the child
alone still compiles: Vue drops the unknown attribute, the child keeps its
default, and the feature is silently absent. Passing it typechecks, and a
test that drives the child directly passes too. A wrapper that deliberately
owns one of its child's props opts out by name in the script, with a reason.

`check:purity` guards the line between a component library and an
application. A component takes props and emits events; it does not know where
its data came from or what else is on screen. The check enforces three things:
a component imports only Vue, this library and the declared rendering
dependencies; it does not read ambient browser state (`fetch`, storage,
cookies, the URL); and reactive state lives in a component instance rather
than in a module, where every reader would share one value.

The failure it catches is silent. A component that reaches for a store still
renders in its story and still renders in the app that has one, so nothing
reports the coupling until a second app wants the component and has a
different store. Two files keep shared state on purpose and are listed in the
script with a reason: the toast queue, because the code that raises a toast is
never the corner that shows it, and the overlay stack, because the screen has
only one.

`check:contrast` carries four known light-theme near-misses that predate the
dark theme, listed in the script; it fails on anything new. They are the same
kind of near-miss as the calendar chip noted under Accessibility: muted text
or the accent on a surface the light palette was not tuned against.

`check:motion` guards the two scales whose drift is invisible: motion and
stacking. A transition written `120ms` behaves correctly and matches its
neighbours today, and is wrong only later, once an app retunes
`--rl-duration-fast` and one component keeps the old speed. A raw `z-index: 30`
is worse, because it reads as deliberate: the `--rl-z-*` tokens carry the ORDER
as their contract and the numbers are incidental, so an app rebasing the ladder
moves every token and leaves the literal behind. The layer that was level with
the flyout is then behind it, drawn but unclickable.

Two exemptions, both narrow. `animation` shorthands are not checked, because a
looping indicator's period is tuned against what it indicates and putting a
spinner and a drawer on one scale makes changing either break the other. And
`z-index: 0/1/2` inside a component is a local decision, such as a card above
its own column's backdrop, which says nothing about the overlay ladder. Anything
higher is claiming a place in it and has to name a token.

### Motion

`--rl-duration-fast`, `-base` and `-slow` are named for what moves rather than
by size, because the choice is about how much of the screen changes: `fast` is a
state change in place (a hover fill, a caret turning), `base` is something small
travelling a short way (a switch thumb, a panel in a stack), and `slow` is a
large surface crossing the screen (the shell's drawer, which reads as jerky at
`base`). One `--rl-ease` serves all of them, because every transition here is
short enough that a separate in and out curve is not visible.

An app retuning them should move all three together: they are a ratio as much as
three values, and `slow` below `base` makes the big surfaces look broken.

The looping indicators (spinner, skeleton shimmer, activity bar) keep their own
literal periods, tuned per indicator against what they are indicating.

### Composed strings and translation

Most text is already the app's: a label, a placeholder or a caption arrives as a
prop. What is left are the strings a component composes around a value it only
knows at render (`3 tasks selected`) and the screen-reader announcements that
have no visible element to hang a prop on. Those go through
`src/composables/useMessages.ts`.

```text
import { provideMessages } from 'rela-components'

provideMessages({
  selectedCount: ({ count, noun }) => `${count} ${noun} geselecteerd`,
})
```

Provided once at the root and merged over the English defaults, so an app
translates what it cares about, and anything it misses (including a string a
later version adds) keeps working rather than rendering blank.

Each entry is a function rather than a template with placeholders, so a
translation can reorder and inflect: `1 item` against `2 items` is the easy case
and Polish has three plural forms. An app already holding an i18n library can
delegate with `selectedCount: (args) => t('table.selected', args)`.

Provide/inject rather than a module-level record, so two languages can coexist
on one page, such as a translator's side-by-side view or an admin screen pinned
to English inside a localised app, and so components stay pure in the sense
`check:purity` means.

The announcements are the reason this exists. An English label in a Dutch app is
reported on the first screenshot; an English announcement inside an `aria-live`
region is heard only by the users least likely to be asked, and looks perfect to
everyone reviewing it.

### Typography

`RlHeading`, `RlText` and `RlFieldLabel` carry the shared type styles for
headings, body copy and field labels. Prefer them over a bare `<h2>` or
`<span>` when adding a component, so sizes and tones stay consistent.
Controls that style their own label, such as buttons and tabs, still set
their type directly from the tokens.

`RlHeading` keeps the semantic level and the visual size apart: `level`
places the heading in the document outline and `size` sets how large it
looks. A section heading can therefore be an `h2` while rendering at the
same size as body text. Set `line-height="normal"` for headings that sit in
running text; the default `tight` suits display titles.

### Icons

`RlIcon` draws one glyph from a set of 237, catalogued under Common/Icon in
Storybook with a filter over the names and descriptions. The set is Lucide
(ISC), and the names are rela's: `src/components/common/icons.ts` is generated
from `scripts/icon-defs.json`, whose entries were taken verbatim from rela's
`internal/dataentryconfig/icondefs`. A name is a public contract with every
project that authored it in `data-entry.yaml`, so the two vocabularies must not
drift.

```text
npm run generate-icons
```

The artwork is **vendored**, not imported at runtime: the generator reads
Lucide's node arrays and writes the SVG children out as markup, so
`lucide-vue-next` stays a devDependency and a consuming app still needs no icon
package. Refreshing the set means bumping that devDependency and re-running the
generator; the diff on `icons.ts` is the review.

The bodies are markup rather than a single path string because most Lucide
glyphs are not one path — a search icon is a path and a circle, a calendar is a
rect and ten segments. The hand-drawn set this replaced could hold one `<path>`
per icon, which is what limited it to 37 approximations.

Two naming rules, both taken from rela's table and both learned there
expensively:

- **Name the glyph, not the use site.** `wrench`, not `settings-page`. A name
  tied to one use site is wrong the moment a second use site appears.
- **Prefer Lucide's canonical component name over a legacy alias.** `House`,
  not `Home`; `TriangleAlert`, not `AlertTriangle`. Aliases are absent from
  Lucide's type declarations and disappear at a major bump.

Adopting rela's vocabulary renamed some of the old set. Most were spelling
(`chevronDown` to `chevron-down`), but one was a genuine inversion worth
knowing: the old `check` was the *circled* check and `checkMark` was the bare
one. Lucide and rela call the circled one `done` and keep `check` for the bare
mark, which is now what those names mean here.

#### Icon names from project config

A sidebar entry is the one place an icon name comes from project config rather
than from this library's own code. Everywhere else the name is written by a
developer, and the `IconName` union already makes a wrong one a type error.

So `resolveIcon` in `src/components/common/resolveIcon.ts` is the boundary, and
it is deliberately a lookup in a closed map rather than anything resembling
dynamic resolution: a config value must never be able to name something the
library did not intend to draw. `RlSidebarNavItem` resolves through it.

An unknown name falls back rather than throwing, so a stale or hand-edited
config still renders a usable sidebar; the author is told by whatever validates
the config on load, which is where a config error belongs. The reserved name
`none` (`NO_ICON`) means "draw nothing" and still holds the icon column, which
is different from an absent name — that one means "use whatever this entry's
kind implies", and only the caller knows what that is.

### Boards and swimlanes

`RlBoard` groups cards into columns: one column per state, a card in the
column it is in. `RlSwimlaneBoard` crosses those columns with named
horizontal lanes, so the board answers "what state is this in" and "whose is
it" at once. That is what a board grouped by assignee, priority or epic
needs, and what sorting a plain board cannot show.

A lane holds sections rather than items, because splitting items across
columns is a decision only the caller can make. The lanes come from
`Swimlane<T>`, whose `sections` reuse the same `Section<T>` the plain board
takes, so a swimlane board is a regrouping of data a board already has.

The two differ in where the scroll lives, and this is the reason the swimlane
board does not reuse `RlBoardColumn`:

- On `RlBoard` each column scrolls on its own, so reaching the bottom of a
  long column leaves the other columns and every heading where they were.
- On `RlSwimlaneBoard` the board scrolls as one sheet in both directions. A
  lane is a row across every column, so a column that scrolled by itself
  would slide out of line with the lane header beside it. The column headings
  are named once in a row that sticks to the top, and each lane's title is a
  rail that sticks to the left, so both stay on screen however far the board
  is scrolled.

Alignment across lanes is held by shared widths rather than by measurement:
every cell and every heading is `--rl-swimlane-column-width` wide, with the
lane rail at `--rl-swimlane-header-width`. A lane that has no section for one
of the board's columns still gets an empty cell there, because a lane that
closed the gap would put every column to its right under the wrong heading.

### Column and lane icons

`Section.icon` and `Swimlane.icon` draw a glyph before the title, and both
take **the component itself** rather than a name to look up:

```text
{ id: 'todo', title: 'To do', icon: resolveIcon('inbox'), items: [] }
```

A name would have to resolve through this library's icon registry, which
makes a column unable to use a glyph the registry lacks and forces any app
with its own registry to map between two vocabularies. Taking the component
means an app passes whatever its own resolver returns, and `RlIcon` is
simply one of the things that can be passed. `NavItem.as` makes the same
choice for the same reason.

The icon replaces the status dot rather than joining it: both occupy the slot
before the title, and a heading carrying a dot and a glyph reads as two
separate claims about the column.

### Moving a card

Both boards can move a card, by dragging it or from the keyboard. Neither
moves it themselves: a drop emits `move` naming the item and where it landed,
and the caller's data decides whether the move happens. A board that moved
its own card would disagree with the caller the moment a save failed.

```text
<RlBoard :can-move="(task) => task.canEdit" @move="({ item, to }) => …" />

<RlSwimlaneBoard :can-move="…" @move="({ item, to, lane }) => …" />
```

`can-move` is answered per item, because permission usually is. Leaving it
out turns dragging off entirely, which is the default: a board that cannot
save a move should not invite one.

Dragging is built on `@atlaskit/pragmatic-drag-and-drop`, which drives the
browser's own drag events rather than tracking the pointer itself. That is
what lets a card stay whatever the `card` slot rendered: `RlBoardCard` wraps
the slot's output instead of replacing it, so the card keeps its own clicks.
The board scrolls itself when a held card nears an edge, since the hand
holding the card is the one that would otherwise scroll.

**A card that is a link works, and needs nothing from the caller.** A link or
an image is natively draggable, so a press that lands on one would start the
browser's own drag of that element: the card's drag never begins and no drop
ever fires, while the card still looks draggable. `RlBoardCard` marks links
and images in the slot `draggable="false"` so the gesture reaches the card.
An element that sets the attribute itself is left alone, for the rare card
that wants a link dragged out of it.

**Touch cannot drag.** The HTML drag events this is built on do not fire for
a finger. The keyboard path below is the accessible route, and a board that
needs touch should offer a "move to" menu on the card.

The keyboard path is a grab-and-place rather than a simulated drag:

```text
Enter/Space   pick the focused card up, or drop it where it is aimed
Left/Right    choose a column
Up/Down       choose a lane, on a swimlane board
Escape        put it back
```

A drag driven by arrow keys would have to invent a cursor position and fire
moves against it, so the card would follow something the user cannot see.
Choosing a named column is the same decision the drop makes, and it can be
announced: a live region names what is held and where it is aimed.

While a card is held the target column is drawn as a tinted, dashed well, the
same feedback a drag gets. It is a well rather than a line between two cards
because a drop sets which column the card is in and nothing about where in
it; an insertion line would promise a position the board does not keep. For
the same reason `move` carries no index. A board that stores an explicit
order needs to derive it from the drop itself.

Collapsing works on both axes. A collapsed lane keeps its header and drops
its cards, and a collapsed column narrows to a rail down every lane at once,
which is what lets a wide board be read without losing the columns that are
not being worked on.

### Actions

`RlButton` splits weight from meaning. `variant` sets how loud the button is
(`primary`, `secondary`, `subtle`, `ghost`) and `tone` sets what it means, so
a destructive action can be a solid red Delete or a quiet red text link
without either needing its own variant. `RlIconButton` takes the same `tone`.

An action that goes somewhere is a link, not a button. `as` renders an `a`,
or a router link component passed directly, while keeping the variant, size
and tone. Use it whenever the action navigates: only a real link gives the
user the middle click, the modifier click that opens a new tab, and the
status-bar preview of the destination. The pending props (`loading`,
`pendingLabel`, `type`) do not apply to a link and are ignored, and a
disabled link has no native equivalent, so omit the link instead.

`as` takes the component as a prop value rather than resolving it by name, so
a test's `global.stubs` never intercepts it. A consumer testing a router-link
button needs a real router rather than a `RouterLink` stub.

Pair actions with `RlButtonGroup` and write the confirming action last: the
DOM order is the order a keyboard or screen-reader user meets them, whatever
the alignment.

Destructive actions should not fire on a single click. `RlConfirmDialog`
holds the real Delete: it opens focused on Cancel, traps Tab, closes on
Escape or a scrim click, and returns focus to the control that opened it.

Set `loading` on a button for a pending action. It disables the button so it
cannot be submitted twice, swaps the leading icon for `RlSpinner`, and sets
`aria-busy`.

### Forms

Every input renders through `RlFieldShell`, which owns the label, the hint,
the error and the ids that tie them together. It hands the control its `id`,
`aria-describedby` and invalid state through a scoped slot, so a field cannot
be left unlabelled by accident.

Rendering your own control into that default slot is a supported entry point,
not a workaround. Use it for a control the library does not cover, and use it
when an app owns its own field layer: an app whose fields carry a column span
or a display/edit mode cannot adopt `RlTextField` wholesale, but it can still
take the shell's labelling and announce behaviour.

The slot props are the contract, and a rename of any of them is a breaking
change, because a host reads them by name:

```vue
<RlFieldShell label="Name" :error="error" v-slot="{ id, describedBy, invalid, required }">
  <input
    :id="id"
    class="rl-control"
    :aria-describedby="describedBy"
    :aria-invalid="invalid || undefined"
    :required="required"
  />
</RlFieldShell>
```

- `id` goes on the control, because the label points at it.
- `describedBy` is the error id first, then the hint id, space-joined, and
  `undefined` when there is neither. That order is deliberate.
- `invalid` is true whenever `error` is set.
- `required` mirrors the prop, which keeps the asterisk decorative.

The visible boxes share one class, `rl-control`, declared in
`src/styles/control.css`. That is what makes a text field, a select and a
date field the same height in a row. It is global rather than scoped on
purpose, so a host rendering its own native `<input>` into the shell can
reach it and drop its duplicated control CSS. Everything else is scoped,
which means a component's look can only be adopted by rendering that
component. The split is the library's adoption seam: global styles can be
borrowed piecemeal, scoped ones come with the component.

An error is announced with `role="alert"` and read before the hint, because
someone who has just been told what went wrong should hear that first.

### Choosing one of a few

Three controls for one decision, picked by how much explaining the options
need. `RlSegmentedControl` fits two to four options of a word each.
`RlSelect` fits a long list whose shape the user already knows.
`RlRadioGroup` is the middle: three to six options that each need a line of
explanation, which neither of the others can show.

It renders a native `radiogroup` of native radios, so Tab enters and leaves
the group as one stop and the arrows move within it. `variant="card"` boxes
each option, which is right when they carry descriptions; `plain` is a bare
list for short labels, where the boxes would be louder than the question.
`inline` lays them in a row, for two or three short labels only.

The error and the description hang off the group rather than each option, so
a reader hears them once instead of once per choice.

### Naming a person

`RlPerson` draws the avatar-and-name pair, and is the only thing that should.
Every place that mentioned someone had been building its own, which is how
one ended up with a name and no avatar and another with an avatar and no
accessible name. Pass `secondaryHidden` where the row is one line tall.

Unassigned is a state rather than an absence: an empty row reads as a
rendering fault, so the component draws a dimmed "Unassigned" instead.

`RlPersonField` picks one. It is not `RlOptionSelect` with a person in the
slot, although that was the first attempt: that control keeps real focus on
its trigger, which is right for a list of statuses and fatal for a list of
people, because a filter box inside a panel that refuses focus can never be
typed into. So the filter is the trigger, as a real input, and the value is
drawn underneath it.

Typing matches the name and the secondary line both, since two people called
Jan are told apart by the line under the name. Unassigning is a row in the
list rather than a small x in the corner: it carries the same weight as
picking someone, and a corner button hides the decision from anyone not
using a mouse. Set `filter="false"` and listen to `update:query` when a
directory is searched on the server.

`variant="inline"` matches `RlOptionSelect` and `RlMultiSelect`: the value
alone until hovered, and sized to what it holds rather than to the row.

### On now versus on submit

`RlSwitch` and `RlCheckbox` mean different things and are not
interchangeable. A checkbox means "include this when I submit" and belongs
in a form with a button under it. A switch means "this is on now", and
flipping one is the act itself. Using a checkbox on a page that saves as you
go misstates when the change lands; using a switch inside a form that needs
submitting misstates it the other way.

The switch reports `role="switch"`, so a reader hears "on" and "off" rather
than "ticked". Pass `pending` while the change is in flight: it holds the
current position rather than flipping optimistically, so the control never
shows a state the server has not agreed to, and it blocks a second press so
a slow save cannot be queued twice.

### Picking a span of dates

`RlDateRangeField` is a from/to pair with one label and one error, not two
date fields side by side. It is its own component rather than a mode on
`RlDateField` because a range is two values with a rule between them, and a
field that switched between one input and two would change its own model
type.

The ends bound each other through `min` and `max`, so the browser's own
picker refuses an end before the start. Validating afterwards would tell the
user off for something the control let them do.

### Editing in place

Two patterns, and choosing between them matters more than either one.

**A value picked from a list keeps its control the whole time.** A status, a
priority, an assignee: `RlOptionSelect` with `variant="inline"` draws the
value bare and only shows the button chrome on hover and focus. Nothing is
swapped in on click, so the thing you hover is the thing that opens. This is
the better pattern wherever it applies, because the control is never
constructed mid-interaction: no focus to move by hand, no commit to infer
from a blur, no second element whose size can disagree with the first.

It also solves what a native `select` cannot. A native one holds only text,
so a status loses its dot and a priority loses its badge exactly when the
user is choosing between them. Each option renders through a slot instead,
and the pattern's keyboard behaviour is rebuilt to match: arrows, Home and
End, `aria-activedescendant` so real focus never leaves the trigger, Escape
to close without choosing.

**A value with no such control swaps.** Free text and a title have no
always-on control that reads as plain text; an input sitting there would make
the record look like a form. `RlInlineEdit` covers these: it reads as text,
becomes a control on click, moves focus in and back out again, commits on
Enter or on focus leaving, and cancels on Escape. A date swaps too, into
`RlDateField`.

Cancelling only works if the caller holds the in-progress value separately
and applies it on `commit`. Writing straight through on every keystroke makes
Escape meaningless, because there is nothing left to go back to.

**Prose swaps, for a different reason.** A description is not a restyled
editor, because the two states differ in what they *contain* and not only in
their chrome: the read view shows the comment threads anchored to the prose,
and the edit view hides them, since a marker anchored to text being rewritten
has nothing stable to hold on to. A class toggle cannot express that; two
renderings can. `readonly` on `RlMarkdownEditor` gives the read half.

The swap carries no commit semantics, though. Prose saves on change with a
debounce, shown through `RlAutoSaveIndicator`, so there is no blur to
interpret and no need to treat the toolbar or a comment popover as "still
editing". Leaving returns to the read view, and what is on screen is already
saved. Use `block` on `RlInlineEdit` for these: prose is as wide as its
column, and sizing it to its content would resize the box on every swap.

`auto-open-picker` opens a control's own picker as the edit view appears, so
a date takes one click rather than two — one to reveal the input and another
to open the calendar.

`RlDetailField` applies all three by field type, and `RlTaskDetail` passes
the choices down through `editable-fields`, `status-options` and
`tag-options`. A multi-tag field stays read-only: picking a single value
would silently drop the rest.

One sizing trap worth knowing, since it is invisible until someone looks
closely. These controls sit inside flex containers, which blockify
`inline-flex` to `flex` and then size them by the flex algorithm rather than
by their contents, giving a box narrower than its own text. They use
`flex: none` with `width: max-content` to opt out. It must be `max-content`
and not `fit-content`, and the negative margin that aligns the value with a
static one has to live on the wrapper rather than the control: a negative
margin shrinks what an element contributes to its parent's intrinsic width,
which reintroduces the same too-narrow box.

### Overlays

`RlModal` is the base for anything blocking: it owns the scrim, the focus
trap, the scroll lock and the focus restore, and `RlConfirmDialog` is built
on it. Overlays register with the shared stack in
`src/composables/useOverlayStack.ts`, so with two open, Escape closes only
the top one and the page scroll is released only when the last one goes.

A modal's root is a `Teleport`, so a class or a style at the call site has no
single element to fall through to. `RlModal` takes them by hand and puts them
on the panel, and `panelClass` does the same for a named class. Reach for
them when one caller needs a width or an override the `size` scale does not
cover, not as a general styling seam. Listeners land on the panel the same
way, so `@keydown` on `RlModal` reaches a key pressed anywhere inside the
dialog. Handle such a key on the panel rather than on a wrapper in the slot:
an event dispatched at the panel, which is what a test targeting
`[role="dialog"]` does, never reaches a descendant.

Two props exist for the cases a token cannot settle. `align` puts the panel on
the reading line instead of the middle, which is where a dialog the user types
into belongs, such as a command palette. `layer` overrides the z-index of the
whole overlay: a modal that raises its own confirm has to sit below it, and a
modal over a third-party editor that picks its own z-index has to sit above
it. Both teleport to the body, so without `layer` the winner is whichever
mounted later, which is the wrong one as often as the right one.

An app whose own CSS already uses a different z-index range can rebase the
`--rl-z-*` tokens onto it, but has to move the whole ladder rather than part
of it: the order is the contract, the numbers are not. Overlays teleport to
the body, so they do not inherit the stacking context of whatever opened
them and compete on these values directly. Raising the shell tokens alone
above `--rl-z-menu` leaves a menu opened inside a slide panel painted behind
it, drawn but unclickable.

`RlMenu` follows the ARIA menu button pattern. Arrow keys move between items,
Home and End jump to the ends, Escape closes and returns focus to the
trigger, and Tab leaves the menu rather than cycling inside it. It is the
right home for a Delete that opens an `RlConfirmDialog`.

A menu's panel takes three kinds of child. `RlMenuItem` is a row,
`RlMenuSeparator` divides groups of rows, and `RlMenuSection` is a block that
is not a row at all: the account a menu belongs to, or a label over a group.
Only the rows are focusable. The arrow-key walk selects `[role="menuitem"]`, so
a section and a separator are stepped over without either having to opt out.

`RlMenuSection` exists for its padding. It matches a row's, so the name in the
block starts on the same vertical line as the labels under it, and a caller
who used a plain `div` would restate those declarations at every call site and
be a few pixels out at one of them. Pass `label` for the first line and `lines`
for the quieter ones beneath. `lines` is an array rather than one joined
string, because the lines stack and a joined string reads as a single sentence
to a screen reader.

#### The account menu

The library has no `RlAccountMenu`. It carries an assembly when the
composition's shape is fixed by the thing itself, and this one is fixed by the
app: which rows exist depends on which pages the deployment's login owns, and
every row is optional. What it has instead is this recipe.

The menu belongs in `RlSidebar`'s `footer` slot, which is the band for chrome
rather than navigation. An account is chrome: it is the same on every page,
which is what separates it from `RlPageHeader`'s `actions` slot, where the
controls act on the page in front of you.

```vue
<RlSidebar v-bind="nav">
  <template #footer>
    <RlMenu align="start" placement="top">
      <template #trigger="{ toggle, attrs }">
        <AccountTrigger v-bind="attrs" :user="user" @click="toggle" />
      </template>

      <RlMenuSection :label="user.name" :lines="[user.email, user.org]" />
      <RlMenuSeparator />
      <RlMenuItem icon="user" :href="urls.profile">Profile</RlMenuItem>
      <RlMenuItem icon="organization" :href="urls.orgs">Switch org</RlMenuItem>
      <RlMenuSeparator />
      <RlMenuItem icon="sign-out" :href="urls.signOut">Sign out</RlMenuItem>
    </RlMenu>

    <RlThemeToggle :model-value="theme" @update:model-value="theme = $event" />
  </template>
</RlSidebar>
```

Four decisions in that are worth stating, because each has a plausible
alternative.

`placement="top"` because the footer sits at the bottom of the viewport. The
panel would flip there on its own, having no room below, but stating the
preference avoids a first paint in the wrong place.

Every row is a link. Each one goes to a page rather than running code, and a
real `a` can be middle-clicked, copied and is announced as going somewhere.
A row that submits a form is the exception, not the rule, and is a button.

Sign out keeps the default tone. `tone="danger"` is for a destructive entry
such as Delete, and signing out destroys nothing; colouring a row people use
daily red spends the warning colour where it is never needed and devalues it
where it is. The separator above it is what marks it as apart.

The theme toggle stays beside the menu rather than inside it. The theme is a
property of this browser and not of the account, and it is a three-way control
among rows that are otherwise all links.

The trigger is the app's own component because `RlMenu` requires a trigger to
spread the bound attributes. On a collapsed sidebar it has roughly 120px, so
it needs the treatment `RlWorkspaceSwitcher` already uses: the sidebar
declares `container: rl-sidebar / inline-size`, so match the same container
query and make the text **visually hidden** rather than `display: none`.

```css
@container rl-sidebar (max-width: 120px) {
  .account-trigger__text {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
}
```

`display: none` would leave a button whose only content is an avatar, and
`RlAvatar` is decorative when a name sits beside it, so the control would have
no accessible name at all. Hidden this way the name is still read. Drop
`decorative` at rail width if you would rather the avatar carry the name
itself; do one or the other, not both, or it is announced twice.

`RlAvatar` needs no fallback of its own: it shows the photo when `src` is set,
otherwise up to two initials from `name`, otherwise a person glyph. With no
organisation, leave the second line out and pass a shorter `lines`.

#### Anchored panels

`RlMenu`, `RlTooltip` and `RlMultiSelect` position their panels through
`src/composables/useAnchoredPanel.ts`, a thin wrapper over Floating UI. The
`placement` and `align` props are a preference, not a guarantee: a panel with
no room on the preferred side flips to the other, shifts along the cross axis
to stay on screen, and caps its height to the space left. A menu on the last
row of a scrolling table opens upward rather than off the bottom.

Panels are teleported to `<body>`, which has two consequences worth knowing
before adding a third. An ancestor with `overflow: hidden` can no longer clip
a panel, which is the point. But a teleported node leaves its component's DOM
subtree, so scoped CSS does not reach it — panel styles live in
`src/styles/panel.css` — and a `focusout` handler cannot test
`root.contains(target)`, because the panel is no longer inside the root. The
composable returns `containsTarget` for exactly that check.

This is the library's one runtime dependency beyond Vue.

`matchWidth` is a floor, not a fixed size. A trigger is often much narrower
than the options it opens, such as a filter reading "All" over options
reading "Visual regression". A panel pinned to the trigger's width squeezes
every option to fit a word the user did not choose, so the panel takes the
trigger's width as a minimum and grows to its content, capped by the
viewport.

### Sliding panels out of the navigation

`RlSlidePanelStack` is the pattern where a click in the sidebar slides a
panel out over the page, and a click on a row inside that panel slides a
second one out beside it. Closing one reveals what was under it, which is
the page itself once the last one goes.

The stack holds no state. Pass the open panels as an array and it draws them
in order, so opening a panel is pushing an entry and closing one is popping
it. Which panel is open is usually part of the route, and a component that
remembered it would fight the URL over who decides. `close` names the panel
to remove, because a close always has a subject.

Each panel's width comes from a token and each one's offset is computed from
the widths before it, so they tile exactly without measuring. Measuring
would settle a frame late and the panel would visibly jump as it slid.

This is not `RlDrawer`. A drawer is a place to finish a task: it takes the
keyboard, dims the page and locks the scroll. These panels are navigation
the user reads alongside the page, so there is no scrim, no focus trap and
no scroll lock, and clicking the page behind them is a normal thing to do.
That is also why `closeOnClickOutside` is off by default. Escape closes one
level rather than the whole stack, so a three-panel stack takes three
presses: collapsing it in one would throw away a navigation the user spent
two clicks building.

The stack fills its containing block, so the positioned wrapper you put it
in decides how far the panels reach. A wrapper that starts after the sidebar
gives panels that slide out of the sidebar; one that includes the sidebar
gives panels that slide over it. The component cannot tell which was meant.

Below 768px the stack shows one panel at a time, the innermost covering the
rest, because two panels and a page do not fit that width.

Each panel's header takes controls from `#panel-actions`, called once per
panel with `{ panel, index }`, so one slot can give each panel different
controls. Expand belongs here: a panel with a full page behind it offers a
way to go there. Both `#panel-actions` and `#panelActions` work.

Focus is left alone by default, because a panel read alongside the page must
not take the keyboard from work still in progress. Pass `manageFocus` where
the panel is the task: focus then moves into each panel as it opens, to the
panel beneath when one closes, and back to whatever opened the stack when
the last one goes. It is not a focus trap either way, so Tab still leaves
the panel and continues into the page.

A sidebar item that opens a panel should say so. Mark it `opensFlyout` and
pass the open panel's id to `RlSidebar` as `flyoutId`: the row then reports
`aria-expanded` and shows that it is the row the panel came from. This is
deliberately not `activeId`, which says where you are. A panel opened over
the page is not a place you went, and marking the row active would claim a
navigation that did not happen.

`RlPanelListRow` is the row such a panel usually holds: an optional
checkbox, a label that opens the next panel, and trailing meta such as a
time. The checkbox is a real control beside the label rather than inside it,
because ticking a task off is not the same action as opening it.

### Acting on a selection

`RlTable` carries `selectedIds` and emits `toggle` and `toggleAll`;
`RlBulkActionBar` is the half that acts on them. Give it the count and put
the actions in its slot.

It floats over the list rather than pushing it down, because a bar that
displaced the rows would move the next row a user is reaching for. It takes
the count rather than the rows: what the actions need is the caller's
business, and passing the items would invite the bar to filter them. The
count is announced politely, since it changes on every tick and an assertive
region would interrupt ten times while a user selects ten rows.

Gate it with the same permission that gates `show-add`: a list that cannot
take a new row usually cannot delete one either.

### Running a command

`RlCommandPalette` is the Ctrl+K dialog: type to narrow, arrow to move,
Enter to run. It is built on `RlModal` with `align="top"`, which is where a
dialog you type into belongs; centred, the list grows in both directions and
the eye has to chase it.

Commands are `{ id, label, description?, icon?, keys?, group?, disabled? }`.
Groups are drawn as headings but the cursor stays flat, so arrowing down
crosses a heading without stopping on it: a heading is not a destination.

Set `filter: false` where the caller is searching a server, and read
`update:query`. Filtering on the label locally would quietly disagree with
what the server returned. The palette never holds the open state, because an
app that binds Ctrl+K also wants to open the palette from a menu and from a
button, so the state has to live above all three.

### Pending states

Three indicators, one per kind of user act. Using more than one for a single
act is the mistake they exist to prevent.

- **Navigation** between screens: `RlActivityBar`, held back 300ms so a fast
  response never flashes a bar. Indeterminate on purpose.
- **An explicit action** the user just took: `loading` on `RlButton`, with
  `pendingLabel` to swap `Save` for `Saving`. The feedback is on the control
  they pressed.
- **Ambient background saving**: `RlAutoSaveIndicator`. Silent while idle,
  because a permanent "Saved" is reassurance nobody reads.

`RlSkeleton` is preferred over a spinner where the shape of the content is
known, since it reserves the space and stops the page jumping.

### History and activity

`RlTimeline` is what happened to something: an activity feed, an audit trail, the
history tab of a detail panel. It is the counterpart to the comment components
and deliberately separate from them. A comment is something a person wrote and
can reply to; a timeline entry is something that happened and cannot be
answered, so giving one thread both would leave every reply control meaningless
on half the rows.

There is no union of event kinds. A typed graph's event vocabulary belongs to the
app, so an entry arrives with its wording, its icon and its tone already chosen
and the timeline lays them out. That is the same boundary the calendar draws by
taking pre-formatted times, and it is what lets this be useful without the schema
contract `docs/component-gap-analysis.md` is still waiting on.

It renders an ordered list, because the sequence is the content: a screen reader
should say "3 of 12" as it moves through the events, which a stack of divs does
not. Grouping under day headings is the app's, for the same timezone reason
`calendarGrid.ts` gives, and those headings are sticky so a long trail never
scrolls away the only thing dating its rows. Pass `datetime` alongside a
formatted `timestamp` to get a real `<time>` element, which is what makes the
date behind "2 hours ago" recoverable.

### Showing code

`RlCodeBlock` is a block of code with a way to copy it: a Lua snippet, a YAML
metamodel fragment, an error's stack. `RlMarkdownEditor` renders fenced code
inside a document; this is the standalone block a help panel or an error dialog
needs.

It does no syntax highlighting. A highlighter means a grammar per language and a
tokeniser to run them, which is a large dependency for every consumer whether or
not they show code, and it is the part an app most often already owns for its
docs. So the code is a slot as well as a prop: pass `code` for plain text, or put
highlighted markup in the default slot and keep the frame, the scrolling and the
copy button.

Pass `code` even with a highlighted slot, because that is what the copy button
reads. Copying the rendered `textContent` instead would carry the line numbers
into the clipboard. The confirmation appears beside the button rather than as a
toast, since the acknowledgement belongs next to what was copied and a block
inside a modal may have no toast host above it. A failed copy reports through
`copy-error` and does not claim success: the clipboard is absent rather than
merely refusing outside a secure context, and a block that said "Copied" while
copying nothing is the worst outcome.

### Messages

For the library's own interface strings and how to translate them, see Composed
strings and translation above; this section is about messages an app raises.

`RlToast` is transient and interrupts; raise one from anywhere with
`useToasts()`. `RlBanner` stays until the condition behind it changes.
`RlCallout` is an aside inside a body of text and is never announced. Each
tone pairs its colour with an icon and its own wording, so no meaning rests
on hue alone.

A toast can carry one `action: { label, onAction }`, for the act the message
invites: undoing a delete, retrying a failed save, opening what was created.
One rather than several, because a toast leaves on a timer and a choice the
user has five seconds to weigh is not a choice; anything needing two options
needs a dialog. The control comes before the dismiss in the tab order, and
the toast dismisses itself once the action has run, since the message
described a state the action has just changed. The toast already pauses its
timer on hover and focus, so an action cannot vanish from under a reach.

Prefer undo to a confirmation for most destructive actions: it is one click
rather than two, and it is honest about the fact that people confirm dialogs
reflexively. Keep the confirmation for what undo cannot reverse.

## Responsive behaviour

Three breakpoints, documented in `src/styles/tokens.css`:

| Width | Sidebar | Task detail | Board | Table |
|---|---|---|---|---|
| `>= 1080px` | Fixed rail | 720px side panel | Columns side by side | Columns with headers |
| `768-1079px` | Drawer when a panel is open | Fluid panel | Columns side by side | Columns with headers |
| `<= 767px` | Off-canvas drawer | Full-screen overlay | 85vw snapping columns | Rows stack into cards |

The table's column is the one entry keyed to its own width rather than the
viewport's, so a narrow pane stacks its rows whatever the window is doing.

`RlAppShell` owns this. Pass `v-model:nav-open` for the drawer and
`panel-open` when a detail view is showing. The drawer closes on Escape or a
scrim tap and locks background scrolling while open.

Controls passed into a slot can use the `rl-compact-only` utility (shown
wherever the sidebar is a drawer) or `rl-phone-only` (shown only where the
detail pane covers the list).

### Scrolling out from under the overlay panel

An open overlay panel covers the right of the content. Without help the
content's scroll still ends at the edge of the row, so whatever the panel
covers cannot be brought out from under it. A board is the clearest case: its
last column sits under the panel and no amount of scrolling reaches it.

`RlAppShell` handles this. While the panel overlays, the shell sets
`--rl-overlay-inset` on `main` to the width the panel took and adds it to
`--rl-page-gutter-right`. Every scroll container in the library already pads
from that gutter, so each one gains the trailing space without changing. That
matters for the board, whose horizontal scroll track lives inside the board
where padding on `main` cannot reach it.

The inset is zero unless a layout sets it, so nothing moves while the panel
is closed or inline, and it returns to zero on a phone, where the panel
covers the screen instead of part of it.

A layout of your own that draws over the right of the content can do the
same. Set both properties together on the scrolling content's ancestor:

```text
.my-layout__main {
  --rl-overlay-inset: 400px;
  --rl-page-gutter-right: calc(
    var(--rl-page-gutter) + var(--rl-safe-inset-right) + var(--rl-overlay-inset)
  );
}
```

Both are needed because a custom property is resolved where it is declared.
The copy of the gutter at `:root` substituted the inset while it was still
zero, so setting the inset alone changes nothing further down the tree.

### Flagging a nav item

A nav item can carry a status: unread items, a failing sync, a section still
being set up. Give it `status: { tone, label, count? }`, where tone is one of
`new`, `info`, `warning`, `error` or `success`.

The tone says what is true, not what to draw. The sidebar picks the glyph and
the colour, so every row flagged `error` looks the same wherever it appears;
a caller choosing an icon per row would be deciding presentation one row at a
time. `new` draws a plain dot, because it means "something is here" and
nothing more precise, and a glyph would imply a kind of thing it does not
know.

`label` is required and is not decoration. An icon carries no text and neither
does a colour, so the label is what a screen reader reads and what the
tooltip shows; it joins the row's accessible name, giving "Deployments, last
deploy failed". `count` is drawn beside the glyph where a quantity adds
something, capped at "99+" so a large number cannot push the label out, and a
zero is treated as absent because it is not a state.

A collapsed parent stands in for the statuses it is hiding, so a branch with
a failure inside does not look calm while it is shut. The most severe tone
wins, and the label says how many rows are flagged rather than repeating one
child's message, which would name a row the user cannot currently see. A
count is summed only within the winning tone, and dropped unless every
status of that tone carries one: adding an unread count to a failed sync
gives a number that means nothing, and a partial sum under-reports while
reading as exact. Expanding hands the job back to the children, which can
say precisely which one is wrong.

Collapsed to a rail the indicator becomes one dot on the icon's corner: a
count has nowhere to render at 60px and a glyph beside a clipped label is
unreadable, so it keeps the part that survives losing the words. This is a
container query on the sidebar rather than a prop, because the sidebar has no
collapsed state of its own; see below.

### Collapsing the sidebar

`RlSidebar` has no collapsed state. It emits `toggle-collapse` and takes its
width from `--rl-sidebar-width`, so the app owns both the state and the
width:

```text
.app--rail { --rl-sidebar-width: 60px; }
```

Rebind the token rather than setting `width` on the sidebar. Setting `width`
does work, because scoped styles carry no specificity bonus, but it overrides
one rule and leaves the rest of the component reading the old value: the
drawer's `min(var(--rl-sidebar-width), 84vw)` keeps its own idea of the width
at narrow sizes. The token is the single place the width is decided.

An app that handles the event and changes neither collapses nothing and
reports nothing: the control works, the state flips, and the rail stays put.
A consumer arriving from a sidebar that owned its own collapsed state is the
one most likely to meet this.

### Resizing a pane

`RlResizer` is the draggable edge between two panes. It is a sibling of the
pane it sizes, not a part of it, for the reason above: the token is the one
place the width is decided, so the resizer writes the token and `RlSidebar`
is unchanged.

```text
<RlSidebar :groups="groups" />
<RlResizer v-model="width" :min="200" :max="420" :default-value="260" />
```

It reports a width and stores nothing. Where that width is kept is the
application's, because a component writing to `localStorage` would share one
key between every instance on the page, which is what `check:purity` exists
to catch.

Clamp the stored width at render rather than storing one width per
breakpoint:

```text
--rl-sidebar-width: clamp(200px, var(--rl-stored-width), 40vw);
```

Per-breakpoint widths look like the answer and are not. The buckets go stale
against each other, so a preference set on a desktop is invisibly replaced on
a laptop rather than reused, and a window dragged across a boundary makes the
pane jump. One number, constrained at render, survives a narrow window
instead of being overwritten by it. `RlAppShell` already does this for the
detail panel on phone.

A grip is drawn at the centre of the edge at rest. Without it the handle is
a 1px line in the same grey as an ordinary border, so the only thing
advertising the drag is a `col-resize` cursor that appears once the pointer
is already on it, which nobody finds by accident. Under the pointer the grip
takes the accent and grows; the line stays 1px, because thickening the full
height as well draws an accent bar down the whole viewport and reads as a
selected pane rather than as an edge.

The handle is a focusable `separator` carrying `aria-valuenow`, because a
drag target is the easiest control in a layout to leave unreachable. Arrows
nudge, Page Up and Page Down move further, Home and End take the ends, and
Enter or a double-click returns to `defaultValue`. On a coarse pointer the
grab area widens to the tap target while the drawn line stays 1px.

`resize-start` and `resize-end` bracket a drag, for a caller that would
rather save once than once a frame.

`RlAppShell` wires this up for the sidebar behind `sidebarWidth`. It is
opt-in: set it and the handle appears, leave it unset and the shell is
exactly as it was. The handle is hidden below 768px, where the sidebar is an
off-canvas drawer and there is no edge between two panes to move.

Anything positioned against the sidebar has to be given the width, not read
it from `--rl-sidebar-width`. The shell binds that token inside itself, so an
ancestor reading it gets whatever `:root` says, which is the default rather
than the dragged width.

### Safe areas

On a notched phone the viewport reaches under the status bar, the home
indicator and, in landscape, the notch itself. Every component that touches a
screen edge already keeps its content clear of those bands: the app shell and
its full-screen detail pane, the sidebar, drawers, the bottom-sheet modal, the
toast host, the activity bar, and the page gutters that body text sits in.

A consuming app needs one line for any of it to apply. Without
`viewport-fit=cover` the browser keeps the page inside the safe area itself and
reports every inset as zero, so the padding resolves to the values it always
had:

```html
<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover" />
```

That is the whole integration. Components read the insets through four tokens
rather than calling `env()` directly:

| Token | Value |
|---|---|
| `--rl-safe-inset-top` | `env(safe-area-inset-top, 0px)` |
| `--rl-safe-inset-right` | `env(safe-area-inset-right, 0px)` |
| `--rl-safe-inset-bottom` | `env(safe-area-inset-bottom, 0px)` |
| `--rl-safe-inset-left` | `env(safe-area-inset-left, 0px)` |

Set them to `0px` on a subtree to stop the padding being applied twice, where
an app renders the library inside a container it has already inset itself.

Each edge has exactly one owner, because a doubled inset is invisible in
testing: it still clears the notch, so it passes any lower-bound assertion,
and it collapses to zero on a device without one. `npm run check:safe-area`
enforces this structurally rather than by measurement.

The sidebar is the one place two components could both claim an edge. As a
drawer it is inset by `RlAppShell`, so that a consumer's own markup in the
sidebar slot is protected too; the shell then zeroes `--rl-sidebar-safe-top`,
`-bottom` and `-left` on it, and `RlSidebar` adds nothing on top. Everywhere
else `RlSidebar` insets itself through those same tokens. A container that
insets a sidebar itself should zero them the same way.

Page-level padding comes from `--rl-page-gutter-left` and
`--rl-page-gutter-right`, which add the relevant inset to `--rl-page-gutter`.
Use those two rather than `--rl-page-gutter` for horizontal padding in app-side
CSS, since in landscape only one edge is obscured.

## Accessibility

Targets WCAG 2.1 AA. Verified with axe-core against every component at 1440px
and 390px in both themes: zero colour-contrast violations in the dark theme.

The light theme has one known exception, which predates the dark theme: the
calendar event chip puts `--rl-color-text-muted` on a tag background, a
pairing the light palette was never tuned for, and lands between 4.15:1 and
4.4:1 instead of 4.5:1. The same pairing passes in the dark theme. Fixing it
means giving the chip its own meta-text colour per tone, or darkening the
muted token.

- Verified at 390px, 900px and 1440px; no horizontal overflow at any width.
- Touch targets reach 40px on coarse pointers via an expanded hit area, so
  the visual density is unchanged on desktop.
- All colour tokens meet AA in both themes: text at 4.5:1 against every
  surface it is used on (base, sunken, hover, active, selected and raised),
  tag text at 4.5:1 against its own background, and status dots at 3:1 as
  non-text content. `npm run check:contrast` asserts this.
- Colour is never the only carrier of meaning. Status dots always accompany
  a text label, and counts expose their unit to screen readers
  (`3` renders as "3 comments").
- Everything clickable is a real `<button>`. Rows and cards use a stretched
  pseudo-element for the full-area hit target rather than a click handler on
  a `<div>`, so they are keyboard operable and appear in the tab order.
- Every interactive control has a visible `:focus-visible` ring at 3:1. The
  shared ring in `src/styles/focus-ring.css` is two box-shadows, an opaque
  gap then the ring, so it follows a rounded corner and stays visible over
  any fill (SC 1.4.11).
- `RlConfirmDialog` is an `alertdialog` that traps focus while open and
  restores it to the opening control on close. `RlModal` and `RlDrawer` do
  the same, and share an overlay stack so nested overlays behave. Restore
  skips a control that was unmounted while the overlay was open, because
  focusing a detached node drops focus to `<body>`, which is what the
  restore exists to prevent (SC 3.2.1).
- Form fields carry their label, hint and error through `RlFieldShell`, which
  wires `aria-describedby` and `aria-invalid` for every input.
- A toast pauses its own timer while hovered or focused, so a message cannot
  disappear mid-read (SC 2.2.1).
- `RlTooltip` appears on focus as well as hover (SC 1.4.13), and never holds
  a meaning that is not available elsewhere.
- Menus, tooltips and option lists flip and shift to stay inside the viewport
  rather than being clipped, so a control near an edge is still usable (SC
  1.4.10).
- Animation respects `prefers-reduced-motion`: the spinner, skeleton and
  activity bar all fall back to a pulse or to no motion.
- The table uses `grid` / `rowgroup` / `row` / `gridcell` / `columnheader`
  roles. The view switcher is a `tablist` containing only tabs, with
  arrow-key roving focus.

Re-check after changing any colour token:

```text
npm run check                    # token contrast, both themes

npm run storybook                # then, against the running catalogue:
node scripts/axe-themes.mjs      # axe-core colour contrast, every story
```

## Conventions

- Components are prefixed `Rl` and are presentational: they take data via
  props and report interactions via events. None of them fetch or mutate.
- The collection components are generic over their row. `RlTable`,
  `RlTableSection`, `RlTableRow`, `RlBoard`, `RlBoardColumn` and
  `RlBoardColumnCollapsed` take `Section<T>`, where `T` needs only an `id` and
  a `title` (`CollectionItem`). Everything else a row carries is the
  consumer's, reached through a slot.
- `Task` is this library's demo row, not a type the collections require. The
  stories and pages pin it through the `RlTaskTable` and `RlTaskBoard`
  fixtures, which wire the tag, priority and card slots. Look at those for the
  shape of an adoption.
- `TableColumn` follows the conventions the established table libraries
  settled on, so an adopter's mental model carries over: `key` identifies the
  column and names its slot (Vuetify), `field` is the accessor (AG Grid, MUI,
  PrimeVue) and `header` is the label (TanStack, PrimeVue). The accessor is
  separate from the rendering in all of them, because sorting and export need
  a cell's value without caring how it looks.
- `RlTable` renders cells through `cell-<key>` slots, or `cell` for all of
  them, each receiving `{ item, column }`. `column.field` is a convenience for
  a plain string or number; anything richer needs a slot. A `meta` slot adds
  trailing content to the name cell.
- The `name` slot replaces a row's primary control, receiving
  `{ item, selected }`. A row that navigates should put a link here: an anchor
  gives middle click, cmd-click, copy-link and a visible target, and a button
  gives up all four. The row stretches whatever element the slot returns
  across the full row and handles truncation and the focus ring, so the caller
  writes a plain link with no class and no overlay of its own.
- Column keys must be unique. They identify a column for its cell slot, for
  hiding and for sorting, so two columns sharing one cannot be told apart:
  hiding either hides both. Cells still render correctly, which is what makes
  this worth warning about — `RlTable` logs a console warning in development
  when it sees a duplicate.
- `column.meta` is a namespaced bag for anything the caller carries per column
  that this library has no opinion about. Use it rather than adding your own
  top-level keys to the column object, which is how MUI works and makes every
  property this library later adds a potential collision.
- `column.align: 'end'` trails a cell's contents, for a column of numbers. A
  stacked row ignores it, so the value stays beside the label naming it.
- `RlTable`'s first column is headed `Name` unless `name-label` says otherwise.
  Give it `name-column` instead — a full `TableColumn` — when it needs to sort;
  its `key` is what `sort` and `sort-click` then carry for it.
- `show-section-header: false` suppresses a section's heading, count, collapse
  toggle and add button, for a flat list that is one unnamed section. It also
  moves the sticky column header to the top of the scroll area, which it must:
  left at its usual offset it floats over the first row and swallows the
  clicks meant for it, while still looking correct.
- `show-add: false` hides both add controls at once: the plus in a section
  header and the button at the foot of its list are the same affordance in
  two places, so gating one would just move the dead control. For a list that
  refuses creation, whether because of a permission or because there is no
  form to create with. The boards take the same prop under the same name.
- `empty-label` (and `empty-description`) is what a section with no rows
  says. Without it an empty section draws its header and then nothing, which
  reads as a list that failed to load rather than one that is genuinely
  empty. Sections that have rows ignore it, so it is set once for the table
  rather than conditionally per section.
- `column.isEmpty(item)` lets a slot-rendered column declare a cell empty, so
  a stacked row drops it rather than leaving a label beside a blank. `field`
  covers this for a plain value; a slot is opaque to the table, so a column
  that renders a widget has to answer for itself.
- `row-attrs(item)` stamps attributes onto a row. The caller never renders
  rows itself, so this is the only way to reach one — for a test hook, or to
  address a row whose cells are all withheld and which renders no link.
- Sorting is the caller's. Mark a column `sortable` and the header becomes a
  control that emits `sort-click`; pass the current state back as `sort`, an
  ordered `{ key, dir }[]` whose first entry is the primary key. The header
  draws the arrow and, once more than one column is in play, each column's
  position in that order. The table never reorders rows itself, because a
  collection of any size sorts on the server.
- `sort-click` carries a `SortClickEvent` alongside the column: `additive`
  (the press asked to add a key rather than replace the sort), plus the
  column's current `position` and `dir`. The header reports the gesture and
  does not interpret it — what a modifier means is the caller's decision, and
  the modifier is the one part the caller cannot recover for itself.
- Give a sorted column a three-state cycle: ascending, descending, then out of
  the sort. Without the third state a key can never be dropped once added
  except by discarding the whole sort, and dropping the primary key is how the
  key behind it gets promoted. The `Sortable` story implements this.
- Column visibility is state, not a flag on the column: `column-visibility` is
  a `Record<key, boolean>` where an absent key means visible, so `{}` shows
  everything. One record then serves a columns menu and a saved view at once,
  which a boolean on the definition cannot. `hideable: false` pins a column
  visible regardless. Fitting a narrow pane is not what it is for: the table
  stacks its own rows for that.
- The table does **not** hide columns to fit. What it does instead is stack
  each row into a labelled card once it is too narrow for fixed columns, which
  loses no data and needs no per-column declaration. PrimeVue shipped a
  built-in `responsiveLayout="stack"` with a breakpoint prop, deprecated it,
  and removed it in v4; column visibility stays the caller's to drive.
- It measures itself, not the window. The stacking is a container query on the
  table, because the table sits beside a detail panel that may or may not be
  open and a viewport media query would be measuring the wrong box. A 520px
  content pane in a 1500px window reads as a desktop to a media query, so the
  fixed columns would stay and the name column, the only child that can give,
  would absorb the whole shortfall and collapse to nothing: the row keeps its
  data and loses its title. Opening a panel restacks the rows without the
  window changing size at all.
- Selection is the caller's too. Passing `selected-ids` — even empty — turns on
  the select column; `toggle` reports a row and `toggle-all` a section with the
  new checked state. A `bulk` slot receives `{ section, count }` and takes the
  column headers' place while that section has rows checked. This is separate
  from `selected-id`, which marks the single row whose detail is open; a table
  can have both at once.
- `RlSidebar` opens three seams for a consumer whose sidebar is not just a
  list of links. `switcher` replaces the workspace control while keeping the
  header's layout, collapse and close buttons. `pinned` holds fixed entries
  above the navigation, outside the filter and without a group heading, since
  an entry the filter could hide is not pinned and one under a heading is
  claiming to be a category. `footer` takes the whole bottom band for chrome
  that is not navigation: a build indicator, a settings link, a theme toggle.
  Each falls back to the current markup, so a sidebar that sets none of them
  is unchanged.
- `RlSidebarNavItem` renders as a `button` by default. A row that navigates to
  a URL should set `as` to an anchor or a router link component, with `attrs`
  carrying its `to` or `href`: only a real link gives the middle click, the
  modifier click, the context menu and the status bar. The choice is per item,
  since one sidebar can hold both a link that goes somewhere and a button that
  runs an action. A link row's caret becomes its own control, so navigating
  and expanding stay separate presses.
- `RlBoardColumn` draws a card through its `card` slot, receiving
  `{ item, selected }`. Without one it falls back to the title alone, which is
  all it can know an item has; pass `RlTaskCard` or your own.
- The `task/` components (`RlTaskDetail`, `RlSubtaskRow`, `RlDetailPanel`) stay
  task-shaped on purpose. They are a feature rather than a collection
  primitive, and a subtask genuinely has an assignee and a due date.
- Repeated UI fragments are their own components rather than inline markup:
  `RlIconButton`, `RlAddButton`, `RlDisclosure`, `RlCount` and
  `RlSectionHeading` are shared by the sidebar, board, table and toolbars, so
  hover and focus behaviour stays consistent and is defined in one place.
- The Storybook toolbar has a Theme switcher (System / Light / Dark). It sets
  the class on the preview's `<html>` rather than wrapping the story, so
  teleported menus, tooltips and modals change with it.
- Every component has its own Storybook entry, grouped by area (Typography,
  Common, Sidebar, Layout, Board, Table, Task, Doc), with a `Playground`
  story wired to controls where the props are worth exploring. `Pages` holds
  the five full-page compositions.
