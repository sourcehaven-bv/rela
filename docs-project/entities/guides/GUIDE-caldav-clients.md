---
id: GUIDE-caldav-clients
type: guide
title: "CalDAV to-do (VTODO) client compatibility"
status: published
order: 24
audience: intermediate
summary: "Which task apps speak VTODO, and what each does with formatted descriptions"
---

Which task apps can talk to a rela CalDAV collection, and what happens to
formatted descriptions when they do.

The short version: **plain text in `DESCRIPTION` is the only representation that
survives everywhere.** No client verified below *renders* HTML from the standard
`ALTREP` mechanism, and several destroy it on the first edit. See
[Rich text: the two mechanisms](#rich-text-the-two-mechanisms) for why.

## Client table

"Round-trip" is the column that matters operationally: it describes what happens
to a `DESCRIPTION;ALTREP=...` that rela sent when the client edits the task and
PUTs it back.

| Client | Platforms | VTODO | Rich-text notes | Round-trip of `ALTREP` | Notes |
|---|---|---|---|---|---|
| **Mozilla Thunderbird** | Win / macOS / Linux | Full (VEVENT + VTODO, no VJOURNAL) | **Emits and renders `DESCRIPTION;ALTREP="data:text/html,…"`** — the only client verified to do so | Preserves its own; **clears `ALTREP` whenever the plain text is set** | Since TB 91. Rich-text editor in the task dialog. HTML is sanitised before display. **Auto-discovers from the calendar HOME SET** (`/principal/calendars/`) — it walks the discovery chain and offers a subscribe picker. Given a single COLLECTION URL it treats that as the whole calendar and never finds siblings, so an account set up that way misses collections added later. |
| **Apple Reminders** | macOS / iOS | Full — two-way, verified against self-hosted CalDAV. Preserves `LOCATION`, `CATEGORIES` and `PRIORITY` on write, but displays only PRIORITY; its "Locatie" is a geofence, not `LOCATION` (see below) | Plain text only (no formatting UI) | **Emits a bare `DESCRIPTION` with no `ALTREP`** (wire capture, iOS 26.5.1 PRODID). Whether it would *preserve* an `ALTREP` we sent is untested | **TLS mandatory** (plaintext → endless 401). Needs explicit `https://`. Rewrites the whole VCALENDAR on PUT. Client-created UIDs are bare UUIDs. See [caldav.md](caldav.md). |
| **Apple Calendar** | macOS / iOS | **None** — events only; to-dos are Reminders' job | n/a | n/a | Will not display a VTODO collection. |
| **eM Client** | Win / macOS | Full | **Emits `X-ALT-DESC;FMTTYPE=text/html`** (Microsoft mechanism, not `ALTREP`) | Unknown for `ALTREP`; **prefers `X-ALT-DESC` when present** | If you change `DESCRIPTION` but leave a stale `X-ALT-DESC`, eM Client shows the **old** HTML and the edit looks lost. Auto-discovery, Basic auth. |
| **Nextcloud Tasks** (web) | Web | Full | **Markdown**, rendered and sanitised (raw HTML is escaped, not rendered) | **Destroys it on edit** — removes and re-adds `DESCRIPTION`, discarding all parameters | Deliberate anti-stale-HTML behaviour (issue #2239, fixed 0.15.0). Untouched tasks round-trip fine. |
| **GNOME Evolution** | Linux | Full | Renders **Markdown**, not `ALTREP` HTML | **Preserves** (stores `altrep`, writes it back) — but can leave it **stale** after a plain-text edit | Stores and round-trips `ALTREP`; never dereferences or displays it. |
| **Endeavour** (ex-GNOME To Do) | Linux | Full, via evolution-data-server | Inherits EDS: no `ALTREP` rendering | Same as Evolution | Renamed from GNOME To Do (~2022). Maintained but low-velocity. |
| **KDE KOrganizer / Merkuro** | Linux | Full | Plain text | **Drops it** — KCalendarCore has *zero* `ALTREP` references in 178 source files | Verified by full-repo scan with a positive control. |
| **Tasks.org** | Android (+ JVM desktop) | Full, native CalDAV | Plain text | **Drops it** | **Requires the server to advertise `<C:comp name="VTODO"/>`** or the collection is silently hidden from the picker with no error. |
| **jtx Board** | Android | Full (VTODO + VJOURNAL) | Markdown-ish editor, stored and sent as **plain text** | **Drops it** | No CalDAV stack of its own — **needs DAVx⁵**. Handles `ALTREP` for `LOCATION`/`COMMENT` but deliberately not `DESCRIPTION`. |
| **DAVx⁵** | Android | Full (sync adapter; no task UI) | n/a — sync layer | **Drops it** | Pairs with jtx Board / Tasks.org / OpenTasks. Full auto-discovery; Basic, client certs, limited OAuth. Defaults to permissive when `supported-calendar-component-set` is absent. |
| **OpenTasks** | Android | UI/provider only — no iCalendar or CalDAV code | n/a | n/a | Needs DAVx⁵. Effectively unmaintained (last commit 2021). |
| **Vikunja** | Self-hosted web | CalDAV **server**, not a client | Rich text in its own API; plain over CalDAV (inferred) | n/a | Its docs call CalDAV *"early alpha … has bugs"*. No `PERCENT-COMPLETE` or `LOCATION`. |
| **Microsoft Outlook** | Win / macOS | **No native CalDAV at all** | Uses `X-ALT-DESC;FMTTYPE=text/html` | n/a | Needs the third-party **Outlook CalDav Synchronizer** add-in (Windows), which *does* sync VTODO and maps Outlook RTF to `X-ALT-DESC`. |
| **BusyCal** | macOS / iOS | Full (generic CalDAV to-dos, plus a separate EventKit path) | Unknown | Unknown | Closed source. |
| **Cfait** | Linux / Win / Android / FreeBSD (macOS third-party builds) | Full | Not documented | Unknown | Open source (GPL-3.0, Rust), offline-first. |
| **2Do** | iOS / macOS | Full | Unknown | Unknown | Its own docs note CalDAV drops List Groups, Tag Groups and Smart Lists. |
| **Fantastical** | macOS / iOS | **Unconfirmed** — task features documented are all non-CalDAV (iCloud Reminders, Google Tasks, Todoist) | Unknown | Unknown | Do not assume it is a generic VTODO client. |
| **GoodTask** | macOS / iOS | **Not a CalDAV client** — EventKit over Apple Reminders | n/a | n/a | Only sees what the OS account exposes. |
| **Mozilla Sunbird** | — | Discontinued (2010) | n/a | n/a | Superseded by Thunderbird's built-in calendar; Lightning is no longer a separate add-in. |

### Explicit unknowns

Closed-source clients (**BusyCal, 2Do, Fantastical, eM Client's `ALTREP`
handling**) cannot be verified without live wire capture. They are marked
"unknown" rather than guessed. **Apple Reminders' `ALTREP` round-trip is
likewise unverified** — see below.

## What a client does when a write is refused

Verified on the wire (2026-08-12) against a rela deployment whose ACL granted
reads but denied `update` on the collection's entity type, so every client edit
received a `403`.

| Client | On `403` (refused) | On `500` (transient) |
|---|---|---|
| Apple Reminders | reverts the edit within seconds ✅ | shows the edit as saved ❌ |
| Mozilla Thunderbird | declines the edit, but keeps the stale local copy ⚠️ | shows the edit as saved ❌ |

**Apple Reminders handles `403` well.** The edit reverts, the user sees the
server's value, and polling continues normally.

**Thunderbird keeps its optimistic copy.** The edit is not applied server-side,
but the local view still shows it — including a phantom checked state — and this
survives an app restart. Each poll re-sends the queued edit, is refused again,
and the stale copy stays. The user has no indication their change never landed.

Note Thunderbird polls on a **fixed 30-minute schedule** (verified across a full
day of captures: `08:52`, `09:22`, `09:52`, … all `:22`/`:52`). It also offers an
explicit *Synchronize* command. When testing refusal behaviour, that schedule —
not the server — governs when anything happens, and a quiet half hour is normal
rather than a sign the client has given up.

The operator consequence: a write-denying ACL leaves a Thunderbird user with a
local view that silently diverges from the server. Prefer `read_only:` (which
accepts the write and drops the field) when the goal is "clients may see this
but not change it".

**rela therefore does not refuse a write on the wire at all.** A denied update
answers `2xx` carrying the entity as it actually stands, with the `ETag`
suppressed — RFC 4791 §5.3.4 forbids a strong ETag when the stored bytes differ
from the submitted ones, and here rela stored none. With no valid tag the client
must re-read, which is exactly the reconciliation that unsticks it: an item
stuck across restarts cleared instantly when tested this way.

This follows Apple's CalendarServer, the reference implementation, which does
the same for VTODO — `replaceMissingToDoProperties` restores organizer and
attendee properties a client tried to remove, keeps the `2xx`, and suppresses
the ETag. Accept-and-normalize is the norm across CalDAV servers; sabre/Baikal
and Radicale rewrite submitted data too.

The refusal is still real: the entity is untouched and an audit row records the
principal, the rule and the attempted op. What changes is only what the *client*
is told — and it is told the truth in the representation. The trade-off is that
a CalDAV client cannot be used to probe permissions; the HTTP API still answers
`403`. A denied **create** still answers `404`, since nothing exists at that
href to serve back.

The table above records behaviour observed against `403`, before that change.
See `refusedWriteResponse` in `internal/dataentry/caldav_write.go`.

**`500` is shown as success by both**, which is defensible — a to-do client
treating a server error as transient and retrying is reasonable — but it means
an operator misconfiguration that produces `5xx` is invisible to users, who go
on believing their edits were saved. Watch the server log, not the client.

Two further observations, both from the same session:

- **A refused write is retried indefinitely.** 24 denied writes accumulated from
  a handful of edits. A deployed write-denying ACL produces a permanent trickle
  of `denied-write` audit rows; that is the clients working as designed, not a
  fault.

- **Thunderbird strands a locally-modified copy.** One to-do kept an optimistic
  edit and a phantom checked state across an app restart. This follows from the
  disconnect above — once the collection stops syncing, nothing re-fetches the
  server's version, so the phantom is permanent. Whether removing and re-adding
  the calendar clears it is unverified.

## LOCATION is not "Locatie" in Apple Reminders

Verified on the wire (`remindd/3976`, macOS 26.5.1, 2026-08-12). Two unrelated
things share the word:

| | |
|---|---|
| `LOCATION:Albert Heijn` | The iCalendar free-text field. Reminders **preserves it byte-for-byte** across a round trip but **displays it nowhere**. |
| Reminders' "Locatie" toggle | A **proximity alarm**, emitted as `X-APPLE-STRUCTURED-LOCATION` inside a `VALARM` — an address, a `geo:` coordinate pair and a radius, with `X-APPLE-PROXIMITY:ARRIVE`. |

A location typed into Reminders therefore arrives as:

```text
BEGIN:VALARM
ACTION:DISPLAY
X-APPLE-PROXIMITY:ARRIVE
X-APPLE-STRUCTURED-LOCATION;VALUE=URI;X-ADDRESS=Dam\n1012 JS Amsterdam\nNederland;
 X-APPLE-RADIUS=100;X-TITLE=Dam:geo:52.372707,4.894164
END:VALARM
```

rela maps `location:` to `LOCATION` and does not model alarms, so **a geofence
set in Reminders is discarded on the next sync**. Thunderbird, by contrast, both
shows and edits `LOCATION` as ordinary text.

Two consequences for an operator: a `location:` mapping is useful for
Thunderbird and invisible in Reminders, and a user who sets a Reminders location
loses it silently.

## Rich text: the two mechanisms

There are two competing ways to attach HTML to a description.

**`ALTREP` (RFC 5545 §3.2.1)** — the standard one. A property parameter holding a
URI that "points to an alternate representation for a textual property value".
The RFC requires the property value itself to still carry the plain-text
version, so a client that ignores `ALTREP` degrades correctly:

```text
DESCRIPTION;ALTREP="data:text/html,%3Cb%3Ebold%3C%2Fb%3E":bold
```

The RFC notes "there is no restriction imposed on the URI schemes allowed", which
is what makes a `data:` URL legal here.

**`X-ALT-DESC;FMTTYPE=text/html`** — Microsoft's non-standard property, used by
Outlook/Exchange and eM Client. It carries HTML inline rather than as a URI.

The unhappy result: **the standard mechanism is implemented by Thunderbird, and
the Microsoft mechanism by the Microsoft-adjacent clients, with almost no
overlap.** Neither is safe to rely on alone.

### The stale-HTML failure mode

Both mechanisms share one dangerous property: a client that *prefers* the HTML
alternate will show it even when the plain-text `DESCRIPTION` has since changed.
Update one without the other and the user sees the **old** text and concludes
their edit was lost.

This is a real, repeatedly-encountered bug, not a theoretical one — Nextcloud
Calendar fixed it for `ALTREP` (#3863) and again for `X-ALT-DESC` (PR #4744,
v4.2.0), and Nextcloud Tasks needed the same fix (#2239, v0.15.0). Thunderbird
guards against it in the other direction: setting `descriptionText` explicitly
nulls the `ALTREP` parameter.

> **If rela ever emits an HTML alternate, it must clear or rewrite it on every
> `DESCRIPTION` change.** Never update one half of the pair.

### Guidance for rela

1. **Emit plain text.** It renders correctly in every client in the table.
2. **Emitting `ALTREP` alongside it is safe but low-value** — RFC 5545 mandates
   the plain-text fallback, so non-supporting clients degrade gracefully. Only
   Thunderbird will show the formatting.
3. **Expect round-trip loss.** Tasks.org, jtx Board, DAVx⁵, KOrganizer and
   Nextcloud Tasks all strip `ALTREP` when they PUT a task back. If rela stores
   HTML, it must re-merge server-side rather than trusting the client to return
   it — otherwise the first edit from an Android client silently destroys the
   formatting.
4. **Markdown in the plain `DESCRIPTION` value is the pragmatic choice.** It
   survives every client intact, and Nextcloud Tasks, Evolution and jtx Board
   actually render it.

## Is there a convention for the HTML subset?

**No published standard or de-facto convention exists.** RFC 5545 defines the
`ALTREP` transport but says nothing about permitted markup, and no specification
constrains the HTML subset.

Observed practice diverges sharply:

- **Thunderbird** round-trips whatever its editor produces — inline formatting
  (`<b>`, `<i>`, `<u>`, `<br>`) — and **sanitises HTML before display**. Its
  Bugzilla discussion used `<h1>` and `<body>` wrappers as examples.

- **Nextcloud** captures show Thunderbird emitting a minimal `<body>test</body>`
  wrapper.

- **eM Client** emits a fuller fragment including a `<style>` block.
- **Outlook `X-ALT-DESC` examples** in the wild are complete HTML documents,
  `<!DOCTYPE …><HTML><BODY>…`, sometimes with base64-embedded images.

So implementations range from a bare inline fragment to a full document with
CSS. Since every consumer sanitises (or ignores) the markup anyway, **emit the
smallest inline fragment that expresses the formatting** — `<b>`, `<i>`, `<u>`,
`<br>`, `<p>`, `<a>`, lists — and do not rely on CSS, `<style>`, scripts or
embedded images surviving. Always URL-encode the payload in the `data:` URL, and
always keep a faithful plain-text rendering in the property value.

## Sources

**Standards**

- RFC 5545 §3.2.1, `ALTREP` — <https://www.rfc-editor.org/rfc/rfc5545#section-3.2.1>
- RFC 4791 §7.8.9 (completed to-dos are the client's business) — <https://www.rfc-editor.org/rfc/rfc4791>

**Thunderbird** (primary: source + Bugzilla)

- `CalItemBase.sys.mjs`, `descriptionHTML` / `descriptionText` — the exact
  `"data:text/html," + encodeURIComponent(html)` emit, and the `ALTREP`-nulling
  on plain-text set — <https://github.com/mozilla/releases-comm-central/blob/master/calendar/base/src/CalItemBase.sys.mjs>

- `CalTodo.sys.mjs` — `CalTodo.prototype.__proto__ = calItemBase.prototype`,
  confirming the same rich-text path applies to **VTODO**, not just events —
  <https://github.com/mozilla/releases-comm-central/blob/master/calendar/base/src/CalTodo.sys.mjs>

- Bug 1607834, "Separate HTML descriptions in calendar events" (shipped TB 91) — <https://bugzilla.mozilla.org/show_bug.cgi?id=1607834>
- Bug 1659363, sanitise-and-render descriptions as HTML — <https://bugzilla.mozilla.org/show_bug.cgi?id=1659363>
- Item model: "Thunderbird implements `VEVENT` events and `VTODO` tasks but not `VJOURNAL`" — <https://source-docs.thunderbird.net/en/latest/calendar/item_model.html>

**Apple Reminders** — verified empirically twice: against a local Radicale
(macOS 26.5.1, 2026-08-09) and against rela behind Pratique (2026-08-11).
Two-way VTODO sync, TLS mandatory, whole-VCALENDAR rewrite on PUT, bare-UUID
UIDs. Its PUTs carry a plain `DESCRIPTION:some notes` with **no `ALTREP`
parameter** (`PRODID:-//Apple Inc.//iOS 26.5.1//EN`), consistent with having no
formatting UI. Whether it would *preserve* an `ALTREP` that a server sent is a
different question and remains untested — it would require seeding one and
watching the round-trip.

**Mozilla Thunderbird** — also verified against rela behind Pratique
(2026-08-11), corroborating the source reading: it emitted
`DESCRIPTION;ALTREP="data:text/html,test%3Cbr%3E…%3Cb%3E%3Ci%3E%3Cu%3Emark…":test\n\nwith markup\n\n🙂`
for a VTODO — inline `<b><i><u>` and `<br>`, with a faithful plain-text value
alongside. It sends no `If-Match`, using `SEQUENCE` + `X-MOZ-GENERATION`
instead, and uses `REPORT` rather than full `PROPFIND` enumeration after a
write.

- Apple dropped CalDAV only for *iCloud-hosted* Reminders (private CloudKit
  silo); third-party CalDAV accounts still work — <https://www.busymac.com/docs/faqs/112990-reminders-in-ios-13-and-macos-catalina-drops-support-for-caldav/>

**Android**

- ical4j preserves `ALTREP` byte-identically on round-trip; the apps lose it in
  their own mapping layer by reading `prop.value` (a bare `String`).

- DAVx⁵ `DescriptionBuilder.kt` / `UnknownPropertiesBuilder.kt` (DESCRIPTION is in
  `KNOWN_PROPERTY_NAMES`, so the unknown-property preservation fallback never
  rescues it) — <https://github.com/bitfireAT/davx5-ose>

- ical4android `Task.kt` — <https://github.com/bitfireAT/ical4android/blob/main/lib/src/main/kotlin/at/bitfire/ical4android/Task.kt>
- Tasks.org `caldav/Task.kt` and `CaldavClient.kt` (the `supportsTasks` check that
  hides collections lacking `<comp name="VTODO"/>`) — <https://github.com/tasks/tasks>

- jtx Board `MarkdownState.kt` — markdown is an editor affordance, stored as plain text — <https://github.com/TechbeeAT/jtxBoard>

**Linux desktop**

- KCalendarCore — full-repo scan, zero `ALTREP` references — <https://github.com/KDE/kcalendarcore>
- evolution-data-server `e-cal-component-text.c` — parses `ALTREP` in and writes it
  back out via `e_cal_component_text_fill_property()` — <https://gitlab.gnome.org/GNOME/evolution-data-server/-/blob/master/src/calendar/libecal/e-cal-component-text.c>

- Evolution renders Markdown, never dereferences `ALTREP` (`comp-util.c`, `itip-view.c`)
- Evolution CalDAV task lists — <https://help.gnome.org/users/evolution/stable/tasks-caldav.html.en>
- Endeavour (ex-GNOME To Do), depends on `libecal-2.0` — <https://gitlab.gnome.org/World/Endeavour>

**Nextcloud**

- Calendar #3863, ignores `ALTREP` when updating a description — <https://github.com/nextcloud/calendar/issues/3863>
- Calendar PR #4744, clear `X-ALT-DESC` on description change (v4.2.0) — <https://github.com/nextcloud/calendar/pull/4744>
- Tasks #2239 + PR #2240, same fix for tasks (v0.15.0); `src/models/task.js` `set note`
  removes and re-adds `description`, discarding its parameters — <https://github.com/nextcloud/tasks/issues/2239>

**Commercial / other**

- eM Client emits `X-ALT-DESC` (wire capture in Nextcloud Tasks #2239); staff
  confirmation "We sync both fields" — <https://forum.emclient.com/t/caldav-sync-of-notes-field-in-tasks-does-not-work-properly/76642?page=2>

- Outlook CalDav Synchronizer — VTODO sync, "Map Outlook formatted RTFBody to html
  description via X-ALT-DESC attribute" — <https://caldavsynchronizer.org/about/features/>

- Vikunja CalDAV supported-property matrix, "early alpha" warning — <https://vikunja.io/help/caldav/>
- 2Do sync-method limitations — <http://www.2doapp.com/docs/faqs/which-sync-method-should-i-use>
- GoodTask is EventKit-based — <http://goodtaskapp.com/how-are-reminders-and-calendar-events-show-on-goodtask/>
- Cfait — <https://codeberg.org/trougnouf/cfait>
- `X-ALT-DESC` background and full-document examples — <https://www.limilabs.com/blog/html-formatted-content-in-the-description-field-of-an-icalendar>
  <https://blog.worldline.tech/2023/06/20/html-icalendar.html>
