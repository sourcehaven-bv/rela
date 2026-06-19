<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { dispatchBridgeRequest, type BridgeRequest } from '@/bridge/relaBridge'

const HANDSHAKE_TYPE = 'rela:port'
const HELLO_TYPE = 'rela:hello'

const route = useRoute()

const iframeRef = ref<HTMLIFrameElement | null>(null)

// The app loads directly from its own served URL (same-origin); its files
// resolve relative to this path and the server applies the path-scoped CSP. The
// :key on the iframe (the app id) gives each app a fresh element, so switching
// apps tears down cleanly with no stale `load` events.
const appId = computed(() => (typeof route.params.id === 'string' ? route.params.id : ''))
const appSrc = computed(() => (appId.value ? `/api/v1/_apps/${encodeURIComponent(appId.value)}/` : ''))

// The host owns one MessageChannel per loaded app. port1 stays here and runs
// the dispatcher; port2 is handed to the iframe. handshakeDone gates the
// handshake to once per app: a second hello (e.g. after the app navigates its
// own iframe) must NOT mint a fresh port, which would hand the bridge capability
// to whatever it navigated to. The :key=appId remounts the iframe per app,
// which resets this for a genuinely new app.
let hostPort: MessagePort | null = null
let handshakeDone = false

function teardownPort() {
  if (hostPort) {
    hostPort.onmessage = null
    hostPort.close()
    hostPort = null
  }
}

// Iframe-initiated handshake. The app's SDK posts `rela:hello`; we verify the
// message came from *our* iframe's window, then reply with the MessageChannel
// port targeted at the iframe's ACTUAL origin (ev.origin) — never a guessed or
// wildcard origin, so the port can't leak to anything but the frame that asked.
// Once-only per app (handshakeDone) so a post-navigation hello can't re-mint a
// port for a different document.
function onHostMessage(ev: MessageEvent) {
  if (ev.data?.type !== HELLO_TYPE) return
  const iframe = iframeRef.value
  if (!iframe || ev.source !== iframe.contentWindow) return
  if (handshakeDone) return
  handshakeDone = true

  teardownPort()
  const channel = new MessageChannel()
  hostPort = channel.port1
  hostPort.onmessage = async (portEv: MessageEvent) => {
    const req = portEv.data as BridgeRequest
    if (!req || typeof req.id !== 'number' || typeof req.method !== 'string') return
    const res = await dispatchBridgeRequest(req)
    hostPort?.postMessage(res)
  }
  hostPort.start()
  // Reply to the EXACT window that sent the verified hello (ev.source, already
  // checked === our iframe). targetOrigin is '*' because the sandboxed iframe's
  // origin is the opaque "null", which Chrome does NOT match against the literal
  // 'null' string (delivery silently fails) — so '*' is the only value that
  // works. This is safe: the port goes to the one pinned ev.source window, not
  // broadcast, and we hand out at most one port per app (handshakeDone). The
  // SDK additionally only accepts a port whose ev.source === window.parent.
  ;(ev.source as Window).postMessage({ type: HANDSHAKE_TYPE }, '*', [channel.port2])
}

// Reset the once-only handshake when switching to a different app (the iframe
// remounts via :key, so a fresh handshake is correct for the new app).
watch(appId, () => {
  handshakeDone = false
})

window.addEventListener('message', onHostMessage)

onBeforeUnmount(() => {
  window.removeEventListener('message', onHostMessage)
  teardownPort()
})
</script>

<template>
  <div class="app-host">
    <!--
      Sandboxed iframe loaded from the app's own served URL. allow-scripts (the
      app needs JS) + allow-forms, but NEVER allow-same-origin — the app stays a
      distinct browsing context. It IS same-origin with the API (loaded from
      /api/v1/_apps/{id}/), so its confinement is the server's path-scoped CSP
      header (connect-src 'none' → the MessageChannel bridge is the only way to
      the API). Keyed by app id so each app gets a fresh element.
    -->
    <iframe
      v-if="appSrc"
      :key="appId"
      ref="iframeRef"
      class="app-host__frame"
      sandbox="allow-scripts allow-forms"
      :src="appSrc"
    />
  </div>
</template>

<style scoped>
.app-host {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.app-host__frame {
  flex: 1;
  width: 100%;
  border: none;
}
</style>
