package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/secrets"

	"gopkg.in/yaml.v3"
)

// settingsPath is the route of the project settings window.
const settingsPath = "/_desktop/settings"

// settingsView is what the settings window shows. It carries secret NAMES
// only: a value goes into the keychain and never comes back to a page.
type settingsView struct {
	Project     string   `json:"project"`
	Secrets     []string `json:"secrets"`
	FileSecrets []string `json:"fileSecrets"`
	AI          string   `json:"ai"`
	Mail        string   `json:"mail"`
	// Untrusted is set when the keychain holds secrets for this document's
	// ID that this place may not read yet (see keychainSecrets).
	Untrusted bool   `json:"untrusted"`
	Error     string `json:"error,omitempty"`
}

// ProjectSettings is the bound service behind the settings window. It edits
// one project's secrets (in the keychain) and its AI and mail settings (in
// the project's state). Every method takes the project's id: "" means the
// active project.
//
// Every page can call these methods, including a document's own custom.js,
// so none of them may widen what a document's scripts can read. Approving
// secrets therefore goes through confirm, which page script cannot answer.
type ProjectSettings struct {
	d *Desktop
	// confirm asks the user to approve message in a native dialog and
	// reports the answer.
	//
	// Nil: accepted — every approval is refused.
	confirm func(title, message, approve string) bool
}

// errNoProject is returned when no project is open.
var errNoProject = errors.New("open a project first")

// settingsTarget is the project a settings call edits.
type settingsTarget struct {
	host *desktopHost
	name string
	// active is set when the project is the one at the bare root, the only
	// one SaveFile can reopen.
	active bool
}

// target resolves projectID to a loaded project: "" is the active one.
func (s *ProjectSettings) target(projectID string) (settingsTarget, error) {
	d := s.d
	d.mu.RLock()
	svc, app := d.svc, d.app
	d.mu.RUnlock()
	if projectID != "" {
		p := d.registry.get(projectID)
		if p == nil {
			return settingsTarget{}, errors.New("that project is no longer open")
		}
		svc, app = p.svc, p.app
	}
	if svc == nil || app == nil || svc.Paths() == nil {
		return settingsTarget{}, errNoProject
	}
	if d.keychain == nil {
		return settingsTarget{}, errors.New("the keychain is not available")
	}
	host, err := newDesktopHost(context.Background(), svc.Paths().CacheDir, svc.State(), d.keychain)
	if err != nil {
		return settingsTarget{}, err
	}
	d.mu.RLock()
	active := d.app == app
	d.mu.RUnlock()
	return settingsTarget{host: host, name: app.ProjectName(), active: active}, nil
}

// Load returns a project's settings.
func (s *ProjectSettings) Load(projectID string) settingsView {
	t, err := s.target(projectID)
	if err != nil {
		return settingsView{Error: err.Error()}
	}
	host := t.host
	view := settingsView{Project: t.name}
	fromKeychain, err := host.secrets.all(host.docID)
	if err != nil {
		view.Error = err.Error()
	}
	view.Secrets = slices.Sorted(maps.Keys(fromKeychain))
	if _, trusted, err := host.secrets.forPlace(host.docID, host.Path()); err == nil {
		view.Untrusted = !trusted
	}
	if fromFile, err := host.dir.Secrets(""); err == nil {
		view.FileSecrets = slices.Sorted(maps.Keys(fromFile))
	} else if !errors.Is(err, secrets.ErrNotFound) && view.Error == "" {
		view.Error = err.Error()
	}
	view.AI = hostFileText(host, hostconfig.AIFile)
	view.Mail = hostFileText(host, hostconfig.MailFile)
	return view
}

// SetSecret stores a secret in the keychain. It returns "" or an error message.
func (s *ProjectSettings) SetSecret(projectID, name, value string) string {
	t, err := s.target(projectID)
	if err == nil {
		err = t.host.secrets.set(t.host.docID, t.host.Path(), strings.TrimSpace(name), value)
	}
	return errText(err)
}

// errTrustDeclined is returned when the user does not approve.
var errTrustDeclined = errors.New("not allowed")

// TrustSecrets lets a project's scripts read the keychain secrets stored for
// its document ID, once the user approves in a native dialog. It returns ""
// or an error message.
func (s *ProjectSettings) TrustSecrets(projectID string) string {
	t, err := s.target(projectID)
	if err != nil {
		return err.Error()
	}
	msg := fmt.Sprintf("Allow %q to use the secrets stored for this document?\n\n"+
		"Only allow a document you moved or copied yourself. A document from "+
		"someone else can carry the same ID, and its scripts would then read "+
		"your secrets.", t.name)
	if s.confirm == nil || !s.confirm("Allow Secrets", msg, "Allow") {
		return errTrustDeclined.Error()
	}
	return errText(t.host.secrets.trust(t.host.docID, t.host.Path()))
}

// DeleteSecret removes a secret from the keychain. It returns "" or an
// error message.
func (s *ProjectSettings) DeleteSecret(projectID, name string) string {
	t, err := s.target(projectID)
	if err == nil {
		err = t.host.secrets.remove(t.host.docID, t.host.Path(), name)
	}
	return errText(err)
}

// SaveFile stores ai.yaml or mail.yaml in the project's state after checking
// it parses; empty content removes it. Saving mail settings reopens the
// project, because its mail sender is built once when the project opens.
// It returns "" or an error message.
func (s *ProjectSettings) SaveFile(projectID, name, content string) string {
	t, err := s.target(projectID)
	if err != nil {
		return err.Error()
	}
	host := t.host
	if err = hostconfig.CheckName(name); err != nil {
		return err.Error()
	}
	ctx := context.Background()
	key := hostFileKeyPrefix + name
	if strings.TrimSpace(content) == "" {
		err = host.kv.Delete(ctx, key)
	} else if err = checkHostFile(name, []byte(content)); err == nil {
		err = host.kv.Put(ctx, key, []byte(content))
	}
	if err != nil {
		return err.Error()
	}
	if name == hostconfig.MailFile {
		if !t.active {
			return "Saved. Reopen the project to use the new mail settings."
		}
		s.d.mu.RLock()
		path := s.d.activePath
		s.d.mu.RUnlock()
		if msg := s.d.loadProject(path, false); msg != "" && msg != "needs_setup" {
			return "Saved, but reopening the project failed: " + msg
		}
	}
	return ""
}

// checkHostFile parses content as the named file would be parsed on use.
func checkHostFile(name string, content []byte) error {
	if name == hostconfig.AIFile {
		// Strict, so a key typed in as api_key is refused rather than stored
		// in the document, where it would travel with the file.
		dec := yaml.NewDecoder(bytes.NewReader(content))
		dec.KnownFields(true)
		var cfg ai.Config
		if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("%s: %w (put the API key in the secret %s)", name, err, ai.SecretKey)
		}
		_, err := ai.ParseConfig(content, name)
		return err
	}
	_, err := mail.ParseConfig(content, name)
	return err
}

// hostFileText returns a host config file's content, or "" when absent or
// unreadable.
func hostFileText(host *desktopHost, name string) string {
	data, err := host.File(name)
	if err != nil {
		return ""
	}
	return string(data)
}

// errText is err's message, or "" for nil.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// serveSettingsPage writes the settings window. A static page: it reads
// everything through the ProjectSettings service.
func serveSettingsPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(settingsPage))
}

const settingsPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Project Settings</title>
<style>
  :root { color-scheme: light dark; font: 13px -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
  body { margin: 0; padding: 28px 24px 24px; }
  h1 { font-size: 18px; margin: 0 0 4px; }
  h2 { font-size: 14px; margin: 24px 0 6px; }
  p.hint { color: GrayText; margin: 0 0 8px; }
  table { border-collapse: collapse; width: 100%; }
  td { padding: 4px 0; border-bottom: 1px solid color-mix(in srgb, CanvasText 12%, transparent); }
  td.actions { text-align: right; }
  input, textarea, button { font: inherit; }
  input[type=text], input[type=password] { width: 100%; box-sizing: border-box; }
  textarea { width: 100%; box-sizing: border-box; min-height: 110px; font-family: ui-monospace, Menlo, monospace; }
  .row { display: flex; gap: 8px; margin-top: 8px; }
  .row > * { flex: 1; }
  .row > button { flex: none; }
  #status { min-height: 1.4em; margin-top: 12px; }
  #status.error { color: #c0392b; }
</style>
</head>
<body>
<h1>Project Settings</h1>
<p class="hint" id="project"></p>

<h2>Secrets</h2>
<p class="hint">Stored in this computer's keychain, not in the project. Scripts read them with rela.secret().</p>
<div id="untrusted" hidden>
  <p>This document's scripts cannot read the secrets below yet. They were stored for a document with the
  same ID at another location. If you did not copy or move this document yourself, it may come from
  someone else, and its scripts would read your secrets.</p>
  <div class="row"><span></span><button id="trust">Allow This Document to Use Them</button></div>
</div>
<table id="secrets"></table>
<p class="hint" id="file-secrets"></p>
<form id="add-secret" class="row">
  <input type="text" id="secret-name" placeholder="Name" autocomplete="off" spellcheck="false">
  <input type="password" id="secret-value" placeholder="Value" autocomplete="off">
  <button type="submit">Save Secret</button>
</form>

<h2>AI</h2>
<p class="hint">ai.yaml: provider, base_url, model. Put the API key in a secret named ai_api_key.</p>
<textarea id="ai" spellcheck="false"></textarea>
<div class="row"><span></span><button id="save-ai">Save AI Settings</button></div>

<h2>Mail</h2>
<p class="hint">mail.yaml. Put the SMTP password in a secret named smtp_password. Saving reopens the project.</p>
<textarea id="mail" spellcheck="false"></textarea>
<div class="row"><span></span><button id="save-mail">Save Mail Settings</button></div>

<div id="status"></div>

<script type="module">
import "/wails/runtime.js";

const project = new URLSearchParams(location.search).get("project") || "";
const call = (method, ...args) => window.wails.Call.ByName("main.ProjectSettings." + method, project, ...args);
const $ = (id) => document.getElementById(id);

function status(message, isError) {
  $("status").textContent = message || "";
  $("status").className = isError ? "error" : "";
}

async function load() {
  const view = await call("Load");
  $("project").textContent = view.project ? "For " + view.project + "." : "";
  const table = $("secrets");
  table.replaceChildren();
  for (const name of view.secrets || []) {
    const row = table.insertRow();
    row.insertCell().textContent = name;
    const actions = row.insertCell();
    actions.className = "actions";
    if (view.untrusted) continue; // removing is refused until allowed
    const remove = document.createElement("button");
    remove.textContent = "Remove";
    remove.onclick = async () => done(await call("DeleteSecret", name), "Removed " + name + ".");
    actions.append(remove);
  }
  if (!(view.secrets || []).length) table.insertRow().insertCell().textContent = "No secrets in the keychain.";
  const fromFile = view.fileSecrets || [];
  $("file-secrets").textContent = fromFile.length
    ? "Also read from .rela/secrets.yaml: " + fromFile.join(", ") + ". A keychain secret with the same name wins."
    : "";
  $("untrusted").hidden = !view.untrusted;
  $("ai").value = view.ai || "";
  $("mail").value = view.mail || "";
  if (view.error) status(view.error, true);
}

async function done(err, message) {
  if (err) { status(err, true); return; }
  status(message, false);
  await load();
}

$("trust").onclick = async () => done(await call("TrustSecrets"), "This document can use the secrets now.");

$("add-secret").onsubmit = async (event) => {
  event.preventDefault();
  const name = $("secret-name").value.trim();
  const err = await call("SetSecret", name, $("secret-value").value);
  if (!err) { $("secret-name").value = ""; $("secret-value").value = ""; }
  await done(err, "Saved " + name + ".");
};
$("save-ai").onclick = async () => done(await call("SaveFile", "ai.yaml", $("ai").value), "Saved AI settings.");
$("save-mail").onclick = async () => done(await call("SaveFile", "mail.yaml", $("mail").value), "Saved mail settings.");

load().catch((err) => status(String(err), true));
</script>
</body>
</html>
`
