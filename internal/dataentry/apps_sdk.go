package dataentry

import "strings"

// appHandshakeType is the message type the host posts to the iframe (carrying
// the MessageChannel port) in reply to the iframe's hello. Must match
// HANDSHAKE_TYPE in AppHostView.vue.
const appHandshakeType = "rela:port"

// appHelloType is the message the iframe SDK posts to the host to initiate the
// handshake. The host verifies ev.source === the iframe before replying with
// the port bound to ev.origin, so the capability is never broadcast to a guessed
// origin. Must match HELLO_TYPE in AppHostView.vue.
const appHelloType = "rela:hello"

// appThemeType is a host→iframe message carrying the current theme
// ({type, dark:bool}) so an app linking _rela.css follows the host's light/dark
// setting. Sent live when the host theme changes (the initial value also rides
// on the handshake reply). Must match THEME_TYPE in AppHostView.vue.
const appThemeType = "rela:theme"

// appSDKMethods is the closed bridge method allow-list the in-iframe SDK
// exposes as rela.<method>. Must stay in sync with BRIDGE_METHODS in
// frontend/src/bridge/relaBridge.ts (the host dispatcher that actually
// authorizes each call).
var appSDKMethods = []string{
	"schema", "config", "list", "get", "search", "analyze", "templates",
	"position", "create", "update", "delete",
	"relationCreate", "relationUpdate", "relationDelete", "action",
}

// appSDKSource returns the in-iframe rela SDK as a self-contained IIFE. It is
// served verbatim at /api/v1/_apps/<id>/_rela.js; the app includes it with
// <script src="_rela.js"></script>. The SDK waits for the host to hand it a
// MessagePort (one-time window 'message' from the parent only), then exposes a
// promise-based window.rela that forwards each call over the port. The host
// dispatcher is the actual authorization point; this is just transport.
func appSDKSource() string {
	methods := `["` + strings.Join(appSDKMethods, `","`) + `"]`
	return `(function () {
  var port = null, nextId = 1, pending = {}, queue = [];
  // Replayable readiness: resolves when the bridge port arrives. Exposed as
  // rela.ready (a Promise) and rela.whenReady(cb). This is the robust way for
  // an app to run code on connect — unlike the 'rela:ready' EVENT, which a
  // listener added AFTER the handshake (e.g. registered below a large
  // <script src> that delayed the inline code) would miss. The event is kept
  // for back-compat; new apps should prefer rela.ready / whenReady.
  var isReady = false, resolveReady = null;
  var readyPromise = new Promise(function (res) { resolveReady = res; });
  function send(method, params) {
    return new Promise(function (resolve, reject) {
      var id = nextId++;
      pending[id] = { resolve: resolve, reject: reject };
      var msg = { id: id, method: method, params: params || {} };
      if (port) { port.postMessage(msg); } else { queue.push(msg); }
    });
  }
  function onPortMessage(ev) {
    var res = ev.data;
    if (!res || typeof res.id !== 'number') return;
    var p = pending[res.id];
    if (!p) return;
    delete pending[res.id];
    if (res.ok) { p.resolve(res.result); }
    else { var e = new Error(res.error ? res.error.message : 'request failed'); e.code = res.error ? res.error.code : 'error'; p.reject(e); }
  }
  // Accept the port only from our parent (the host), one time, first wins —
  // so a nested frame the app creates cannot race the handshake and MITM us.
  // Apply the host theme by toggling .dark on <html> — matches the selector
  // _rela.css uses (:root.dark), so an app that links _rela.css follows the
  // host's light/dark setting automatically.
  function applyTheme(dark) {
    try { document.documentElement.classList.toggle('dark', !!dark); } catch (e) {}
  }
  window.addEventListener('message', function (ev) {
    if (ev.source !== window.parent) return;
    if (!ev.data) return;
    // Live theme updates (host theme toggled after handshake).
    if (ev.data.type === ` + jsString(appThemeType) + `) { applyTheme(ev.data.dark); return; }
    if (ev.data.type !== ` + jsString(appHandshakeType) + `) return;
    if (port || !ev.ports || !ev.ports[0]) return;
    applyTheme(ev.data.dark);
    port = ev.ports[0];
    port.onmessage = onPortMessage;
    port.start && port.start();
    for (var i = 0; i < queue.length; i++) port.postMessage(queue[i]);
    queue = [];
    isReady = true;
    if (resolveReady) resolveReady();
    window.dispatchEvent(new Event('rela:ready'));
  });
  // Iframe-initiated handshake: ask the host for a port. The host verifies our
  // source and replies with the port bound to our actual origin, so the host
  // never has to broadcast the port to a guessed targetOrigin. The hello itself
  // carries no capability, so '*' is fine here. Retry until the port arrives so
  // a hello sent before the host's listener is ready is recovered (the host
  // hands out at most one port, so extra hellos are harmless).
  if (window.parent && window.parent !== window) {
    var hello = function () {
      if (port) return;
      window.parent.postMessage({ type: ` + jsString(appHelloType) + ` }, '*');
    };
    hello();
    var tries = 0;
    var helloTimer = setInterval(function () {
      if (port || ++tries > 20) { clearInterval(helloTimer); return; }
      hello();
    }, 100);
  }
  var rela = {};
  ` + methods + `.forEach(function (m) { rela[m] = function (params) { return send(m, params); }; });
  // Replayable readiness — safe to call/await at any time, before or after the
  // handshake completes (see readyPromise above).
  rela.ready = readyPromise;
  rela.whenReady = function (cb) {
    if (typeof cb === 'function') { readyPromise.then(cb); return; }
    return readyPromise;
  };
  Object.defineProperty(rela, 'isReady', { get: function () { return isReady; } });
  window.rela = rela;
})();`
}

// jsString renders s as a double-quoted JS string literal. Used only for the
// fixed message-type constants (appHandshakeType / appHelloType / appThemeType);
// the escaping is sufficient for those but this is not hardened for arbitrary
// untrusted input.
func jsString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`)
	return `"` + r.Replace(s) + `"`
}
