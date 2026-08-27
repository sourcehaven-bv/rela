# Operator customisation hooks (`custom/`)

Customise the rela web UI in place from your own project directory — no fork,
no Go, no build step.

Create a `custom/` directory in your project:

```text
your-project/
├── metamodel.yaml
├── data-entry.yaml
└── custom/
    ├── custom.css      ← optional, injected into the app shell
    ├── custom.js       ← optional, injected into the app shell
    ├── logo.svg        ← any asset your CSS/JS references
    └── fonts/brand.woff2
```

`custom.css` and `custom.js` are referenced from the app shell **only when they
exist** — a stock deployment's HTML is byte-for-byte unchanged. Every file in
the directory is served at `/_custom/<path>`, so your stylesheet can write:

```css
.sidebar { background-image: url(/_custom/logo.svg); }
```

> ## ⚠ Everything in `custom/` is published
>
> This directory is served **publicly and without authentication** — the app
> shell's stylesheet has to load before anyone logs in, so its assets cannot be
> gated. Any file you put here is readable by anyone who can reach the server,
> by guessing or by reading your CSS.
>
> **Do not put anything in `custom/` that you would not publish.**
>
> This is a wider exposure than `apps/`, which sits behind authentication —
> don't assume they behave the same. Paths with a dot-prefixed segment
> (`.env`, `.git/config`, `.DS_Store`) are refused as a safety net, but that is
> a crude filename check, not a secrets scanner: `notes.md`, `backup.sql`,
> `id_rsa` and editor backups like `custom.css~` are all served.

---

> **Read this before you start.**
>
> `custom.js` and `custom.css` run against rela's internal DOM. `<rela-slot>` is
> a supported contract. The `rela-` prefixed classes and `data-` attributes are
> documented but best-effort. Everything else — element structure, internal
> class names, timing — is private and may change in any release. If an upgrade
> breaks your customisation, that is expected. If the SPA misbehaves, remove
> `custom.js` and retry before filing a bug.

This is the escape hatch, not the front door. For ordinary branding —
recolouring, logo, fonts — use the palette and theme system, which *is* a
supported contract.

## Three tiers of contract

| Tier | Mechanism | Stability |
|------|-----------|-----------|
| 1 | Custom elements (`<rela-slot>`) | **Real contract**: tag name, attributes in, events out. A break is a rela bug. *(Reserved — no slot is emitted yet; see below.)* |
| 2 | `rela-` classes + `data-` attributes | **Best-effort.** Documented, but positional — structural changes may break skins. |
| 3 | Anything else (internal classes, scoped-CSS hashes, DOM structure) | **No contract.** |

Tier 3 is not a wart — it is what makes tier 1 affordable. Because an escape
hatch exists, rela does not have to predict every extension point up front.
Customisations that recur in the wild are the signal for what to promote into
tier 1 later.

**Vendor CSS is tier 3.** The markdown editor's internals (`.CodeMirror`,
`.editor-toolbar`, `.fa-*`) come from bundled third-party libraries. They are
skinnable, but they are explicitly outside the stability promise and *will*
break if the editor is ever swapped.

## Writing `custom.css`

Your stylesheet wins by default. All of rela's own CSS is wrapped in a cascade
layer (`@layer rela`); your unlayered rules **outrank every layered rule
regardless of source order or specificity**. A single class selector beats
rela's more-specific scoped selectors:

```css
/* Wins, despite rela's .sidebar[data-v-abc123] being more specific. */
.sidebar {
  background-color: #101820;
}
```

This matters more than it looks. rela ships ~19 stylesheets and loads most of
them lazily as you navigate. Without the layer, whether your rule won would
depend on which route you were on.

### The one exception: `!important`

Cascade layers **invert** `!important`. An important declaration *inside* a
layer beats an important declaration *outside* it — the reverse of the normal
rule. rela has a small number of `!important` declarations, and yours cannot
override them:

```css
.ss-content { border: none !important; }   /* still loses to rela's !important */
```

There is no workaround from CSS alone. If you hit this, the practical options
are to target a different element, or to restructure with `custom.js`. This is
a permanent property of the design, not a bug.

### Design tokens are not layered

The `:root` custom properties (`--accent-color`, `--sidebar-bg`, …) are
deliberately left **outside** the layer, because the same declarations are
served to custom apps in iframes where there is no rela CSS to layer against.
Overriding a token is therefore ordinary CSS, and re-themes the whole UI:

```css
:root {
  --accent-color: #d64545;
}
```

Note that `--accent-color` reaches further than the obvious accented surfaces:
focus rings derive from it (`--focus-ring`), so overriding it re-themes every
focused control too. That is intentional — a focus ring in the stock blue on
an otherwise re-skinned UI looks like a bug.

If you want the ring to differ from the accent, override it directly:

```css
:root {
  --accent-color: #d64545;
  --focus-ring: #1a73e8; /* ring stays blue while the accent goes red */
  --focus-ring-gap: var(--bg-color); /* the band between control and ring */
}
```

Two constraints worth knowing before you do. Keep the ring **opaque**: a
translucent value cannot meet WCAG 2.2 §1.4.11's 3:1 contrast minimum for
non-text UI, which is what the stock ring was failing before it became a
token. And keep `--focus-ring-gap` distinct from `--focus-ring`, since it is
the band that stops the ring merging into the control's own border.

Focus indication under Windows High Contrast is **not** overridable this way,
by design — it is an accessibility floor rather than a skin.

## Writing `custom.js`

Served as `<script type="module">`, so it defers automatically and you get
`import` and top-level `await` with no build step.

```js
document.body.classList.add('acme-deployment')

const res = await fetch('/api/v1/tickets', { headers: { Accept: 'application/json' } })
console.log(await res.json())
```

Requests carry the session cookie, so the REST API is available as your own
logged-in user.

### `custom.js` is fully trusted — unlike custom apps

Do not confuse the two mechanisms:

| | Custom apps (`apps/<id>/`) | `custom.js` |
|---|---|---|
| Runs in | Sandboxed iframe | The app's own document |
| Origin | null (sandboxed) | Same-origin |
| CSP | Path-scoped, `connect-src 'none'` | None |
| API access | Closed method allow-list bridge | Unrestricted `fetch` |
| Intended for | *Distributable* apps others install | *Your own* deployment |

Custom apps are confined because an installable app is untrusted. `custom.js`
is unconfined because you already control the metamodel, Lua scripts and ACL —
there is no privilege boundary left to defend. **Never paste third-party code
into `custom.js`** on the assumption that rela sandboxes it. It does not.

## Two gotchas

**1. DOM you inject inside `#app` will be destroyed.** Vue owns that subtree and
re-renders it. Append to `document.body` and position over the app, or use
`<rela-slot>`.

**2. The SPA mounts *after* your module runs, and there are no route-change
events.** `type="module"` defers past DOM parse, but Vue mounts later still — so
`document.querySelector('.sidebar')` returns `null` at module time even though
the sidebar appears moments later. Wait for what you need:

```js
const whenPresent = (selector) =>
  new Promise((resolve) => {
    const found = () => document.querySelector(selector)
    if (found()) return resolve(found())
    const obs = new MutationObserver(() => {
      if (found()) { obs.disconnect(); resolve(found()) }
    })
    obs.observe(document.body, { childList: true, subtree: true })
  })

const sidebar = await whenPresent('.sidebar')
```

For route changes, poll `location.pathname` or use a `MutationObserver`. rela
exposes no router hooks to outside JS, deliberately: naming one would make a
promise this tier does not offer.

## `<rela-slot>` — the tier-1 contract

> **Not yet emitted.** rela does not currently render a `<rela-slot>` anywhere,
> so defining one today has no visible effect. The tag name and the interop
> contract below are reserved and supported; the first slot ships with the
> next-action widget. This section documents the contract you can rely on when
> it arrives — it is here so the contract is fixed before the first consumer,
> not so you can use it now.

An inert placeholder rela renders at designated extension points. If nothing
defines it, it renders nothing. If `custom.js` defines it, you own its interior
entirely:

```js
customElements.define('rela-slot', class extends HTMLElement {
  static observedAttributes = ['name', 'data-band']

  connectedCallback() {
    if (this.getAttribute('name') !== 'companion') return
    this.textContent = '🐿️'
  }

  attributeChangedCallback(name, _old, value) {
    // rela → you: fires whenever rela updates the attribute.
    if (name === 'data-band') this.dataset.mood = value
  }

  disconnectedCallback() {
    // Vue tearing down the subtree disposes your code cleanly. This is why a
    // slot beats injecting DOM, which gets wiped unpredictably on re-render.
  }
})
```

Two-way interop comes from the platform, with no Vue internals involved:

- **rela → you**: rela sets attributes (`name`, `data-band`);
  `attributeChangedCallback` fires on change.
- **you → rela**: dispatch a `CustomEvent`; the Vue component listens normally.

Use the generic `name=` attribute rather than expecting a bespoke tag per
feature.

## Turning it off

To guarantee a stock UI — or to check whether a customisation is causing a bug:

```yaml
# data-entry.yaml
app:
  disable_custom_injection: true
```

The files stay fetchable under `/_custom/`; only the shell references are
suppressed.

## Notes

- `custom.css` and `custom.js` are optional and independent. Absent means no
  reference is injected, and the URL 404s.
- Changes are picked up without a server restart. Reload the page.
- Served with `Cache-Control: no-cache` plus an `ETag`, so a browser
  revalidates on every request but an unchanged asset comes back as a bodiless
  `304` — your edits appear immediately, and a large font or image is not
  re-downloaded on every page. `Range` requests are supported.
- Every file in `custom/` is served, including nested paths. Files **outside**
  `custom/` are never reachable through `/_custom/`, and there is no directory
  listing and no `index.html` resolution — `/_custom/fonts/` is a 404, not a
  page.
- Unknown file types are served as a download (`application/octet-stream`)
  rather than being refused, so a new image format works without waiting for a
  rela release.
- Files larger than 4 MiB are not served; the server logs a warning naming the
  file, so an oversized image is diagnosable rather than a silent 404.
- Errors in `custom.js` surface through rela's own error logging and will look
  like rela bugs in a report — hence the "remove `custom.js` and retry" line
  above.
