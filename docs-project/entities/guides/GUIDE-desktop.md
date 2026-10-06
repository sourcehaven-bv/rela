---
id: GUIDE-desktop
type: guide
title: "Rela Desktop: Documents, Menus and Notifications"
status: published
summary: Open projects and single-file .rela documents in the desktop app, and send notifications with desktop.yaml
order: "27"
audience: beginner
---

Rela Desktop is the rela app for macOS, Windows and Linux. It opens a project in
a native window, with a menu bar, notifications and save dialogs. It runs the
same data-entry app as `rela-server`, on your own machine, with no server or
login.

This guide covers what is specific to the desktop app: opening a project,
`.rela` documents, the menu bar, downloads, and `desktop.yaml` for
notifications. For the app itself (lists, forms, spaces), see the data-entry
guide.

## Opening a project

Rela Desktop opens two kinds of project:

- **A project folder.** A folder with a `schema.yaml`, as used by the `rela`
  CLI. Open it with **File > Open Project…** or **File > Clone from Git…**.
- **A `.rela` document.** One file that holds the whole project. See below.

**File > Open Recent** lists both kinds. Opening a project that is already open
brings its window to the front.

The desktop app does not apply `acl.yaml`. You own the machine and the files, so
an ACL could not restrict you. The ACL still applies when the same project is
served by `rela-server`.

## Rela documents

A `.rela` document is a single file, for example `Budget.rela`. It holds:

- the schema and every config file (`data-entry.yaml`, `desktop.yaml`,
  `scripts/`, `templates/` and the rest);
- all entities, relations and attachments;
- comments, version history and app settings.

You can copy, back up or send the file like any other document. Double-click it
in Finder to open it.

### Creating a document

Choose **File > New from Template…** (⌘⇧N). Then:

1. Pick a template: a project folder with a `schema.yaml`.
2. Choose where to save the new `.rela` file.

Rela copies the template's config into the document. If the template folder has
an `entities/` folder, its data is copied too. The document does not depend on
the template afterwards; you can move or delete the template.

If the template has no `data-entry.yaml`, Rela offers to generate one the first
time the document opens. The generated file is stored inside the document.

To turn an existing document back into a folder, or to change its config, use
the **File > Project Database** menu. It exports and imports the config and the
data. The `rela db load` and `rela db dump` commands do the same from the
command line; see the SQLite backend guide.

### Secrets, AI and mail settings

Open **File > Project Settings** to set these for the project in the front
window:

- **Secrets**, such as API tokens, are stored in your computer's keychain:
  the Keychain on macOS, Credential Manager on Windows, and the Secret Service
  (GNOME Keyring or KWallet) on Linux. They are never stored in the document.
  Scripts read them with `rela.secret()`. The window shows only their names.
- **AI settings** (`ai.yaml`) name the AI provider and model. Put the API key
  in a secret named `ai_api_key`.
- **Mail settings** (`mail.yaml`) name the mail server. Put the SMTP password
  in a secret named `smtp_password`. Saving them reopens the project.

The AI and mail settings are stored inside the document, so they travel with
it. The secrets stay on your computer. They are linked to the document by an
ID inside it, not by its path. On another computer, set them again.

A document opened from a new location, such as a moved, renamed or copied
document, cannot use the secrets until you allow it. **File > Project
Settings** then shows **Allow This Document to Use Them**, which asks you to
confirm in a system dialog. Until then, the document can neither add nor remove
secrets. Allow it only for a document you moved or copied yourself. A document from someone else can carry
the same ID, and its scripts would then read your secrets.

Any person or program that can use your user account can read these
secrets, as it could read a file in your home folder.

### What stays outside the document

The search index is stored inside the document. Documents created by an
earlier version also had a private folder in your user configuration folder:

- macOS: `~/Library/Application Support/Rela Desktop/documents/`
- Windows: `%AppData%\Rela Desktop\documents\`
- Linux: `~/.config/Rela Desktop/documents/`

A `secrets.yaml`, `mail.yaml` or `ai.yaml` in a document's folder there is
still read. Move those values to **File > Project Settings**; a secret in the
keychain wins over one with the same name in `secrets.yaml`. The folder's old
`search/` and `audit/` folders are no longer used, and you can delete them.

While a document is open, SQLite keeps two more files next to it,
`Budget.rela-wal` and `Budget.rela-shm`. Recent changes may still be in the
`-wal` file, so close the document before you copy it. A third file,
`Budget.rela.lock`, stops a second app from opening the document at the same
time. It stays after the document closes; it is empty then and safe to delete.

### Where to keep documents

Keep documents on a local disk. Do not keep an open document in iCloud Drive,
Dropbox, OneDrive or a network share: SQLite cannot write safely there, and a
sync can corrupt the file. Rela refuses to open a document when it detects such
a folder, but it cannot detect every case. To share a document, close it and
copy it. On a Mac with **Desktop & Documents Folders** turned on in iCloud
settings, your Documents folder is in iCloud Drive. Use a folder outside it, for
example a folder in your home folder.

A document can be open in one app at a time. If it is already open elsewhere,
Rela refuses to open it a second time.

A document carries code: Lua scripts, external commands and custom JavaScript.
Only open documents from people you trust, as you would with a project
repository. To check a document first, export its config with **File > Project
Database > Export Config to Folder…** and read it.

## The menu bar

The menu bar works like other desktop apps. Most items apply to the whole app.
The **Go** menu also lists the spaces of the project in the front window. When
you switch windows, the list changes to that window's project. The first nine
spaces have shortcuts ⌘1 to ⌘9.

| Shortcut | Action                     |
| -------- | -------------------------- |
| ⌘N       | New window                 |
| ⌘⇧N      | New document from template |
| ⌘O       | Open project or document   |
| ⌘⇧O      | Clone from Git             |
| ⌘W       | Close window               |
| ⌘,       | Settings                   |
| ⌘[ / ⌘]  | Back / forward             |
| ⌘K       | Command palette            |
| ⌃⌘S      | Toggle sidebar             |
| ⌘/       | Keyboard shortcuts         |

On Windows and Linux, use Ctrl instead of ⌘.

Right-click a link to an entity or a page to open it in a new window, or to copy
its title. For an entity you can also copy its ID.

## Downloads

Exports, attachments and command results open a save dialog, as in a browser.
The file name comes from the export. Choose where to save it, or cancel.

## Notifications and the Dock badge

`desktop.yaml` at the project root turns entities into native notifications and
a count on the app icon. It is optional; without it, the app sends nothing. In a
document, `desktop.yaml` is stored inside the document like any other config
file.

```yaml
notifications:
  - id: overdue
    type: task
    condition: "entity.status != 'done' and entity.due < today()"
    title: "Overdue: {{entity.title}}"
    body: "Was due {{entity.due}}"
  - id: new-incident
    type: incident
    condition: "entity.severity == 'high'"

badge:
  type: task
  condition: "entity.status != 'done' and entity.due <= today()"
```

### Notification rules

Each rule in `notifications` has:

| Key         | Required | Meaning                                               |
| ----------- | -------- | ----------------------------------------------------- |
| `id`        | yes      | A unique name: lowercase letters, digits, `-` and `_` |
| `type`      | yes      | The entity type the rule watches                      |
| `condition` | yes      | A predicate expression an entity must match           |
| `title`     | no       | The notification title. Default: `{{entity.title}}`   |
| `body`      | no       | The notification text                                 |

`title` and `body` may contain `{{entity.<property>}}` placeholders. Each must
name a property of the rule's type. Other placeholders are refused.

A notification is shown when an entity **starts** matching a rule. It is not
shown again while the entity keeps matching. If the entity stops matching and
later matches again, it notifies again.

The first time a rule runs, it shows nothing. This applies when you first open a
project and when you add a rule. Without it, opening a project with fifty
overdue tasks would show fifty notifications. From then on, only changes notify.

Clicking a notification opens the entity.

### The badge

`badge` has a `type` and a `condition`. The app icon shows how many entities of
that type match. Without a `badge` section, the icon shows no number. With
several projects open, the badge shows the total.

### Conditions

Conditions use the same expression language as list `condition:` in
`data-entry.yaml`, including `today()` for dates. Two things are not available:

- `current_user`: the desktop app has no person record for its user.
- `related(...)`: conditions see the entity's own properties only.

### When rules run

Rela checks the rules when a project opens, two seconds after a change, and
every five minutes. The five-minute check catches conditions on `today()` that
start matching at midnight without any change.

Edits to `desktop.yaml` apply at the next check; you do not need to reopen the
project. If the file has an error, Rela logs it, clears the badge and sends no
notifications until the error is fixed.

### Permission

The first notification asks for permission to show notifications. If you refuse,
Rela shows none; the badge still works. You can change this later in the system
notification settings.

On macOS, notifications only work in the installed app, not in a binary run from
a terminal.
