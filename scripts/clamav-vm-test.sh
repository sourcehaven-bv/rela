#!/usr/bin/env bash
# Verify attachment scanning end-to-end in a real Debian VM: real clamd, real
# bubblewrap, a real systemd unit, real HTTP uploads.
#
# This covers what no Go test can reach. `just test-clamav` exercises the scan
# command through the sandbox, but the subtlest failure in TKT-ZP1EE3 was that a
# conventionally hardened systemd unit silently disables the sandbox — and
# scanning is fail-closed, so the symptom is "every upload is rejected" with a
# startup log that blamed the kernel. Only a real unit under real systemd shows
# that.
#
# The unit under test is extracted from the GENERATED guide, not written here on
# purpose: an operator copies the published unit, so the published unit is what
# must be verified. If someone edits the guide's unit into something that does
# not work, this fails.
#
# Requires limactl (brew install lima). Costs a few minutes on first run — the
# VM is created once and reused; freshclam pulls the signature databases.
set -euo pipefail

VM_NAME="${CLAMAV_VM_NAME:-rela-clamav-test}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GUIDE="$REPO_ROOT/docs/attachment-security.md"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

say()  { printf '\n\033[1m==> %s\033[0m\n' "$1"; }
pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; FAILURES=$((FAILURES + 1)); }
FAILURES=0

command -v limactl >/dev/null || { echo "limactl not found (brew install lima)"; exit 1; }

# ── The unit under test comes from the published guide ───────────────────────
say "Extracting the systemd unit from $GUIDE"
python3 - "$GUIDE" "$WORK/rela-server.service" <<'PY'
import re, sys
doc = open(sys.argv[1]).read()
m = re.search(r'```ini\n(.*?)```', doc, re.S)
if not m:
    sys.exit("no ```ini unit block found in the guide")
open(sys.argv[2], 'w').write(m.group(1))
print(f"  {len(m.group(1).splitlines())} lines")
PY

# The scan config comes from the guide too, so the recipe itself is under test.
say "Extracting the ClamAV scan_cmd from the guide"
SCAN_CMD="$(grep -m1 -oE 'scan_cmd: \[clamdscan[^]]*\]' "$GUIDE")"
[ -n "$SCAN_CMD" ] || { echo "no clamdscan scan_cmd found in the guide"; exit 1; }
echo "  $SCAN_CMD"

# ── VM ───────────────────────────────────────────────────────────────────────
if ! limactl list "$VM_NAME" --format '{{.Name}}' 2>/dev/null | grep -q .; then
  say "Creating Debian VM '$VM_NAME' (first run only)"
  cat > "$WORK/vm.yaml" <<'YAML'
vmType: "vz"
rosetta:
  enabled: false
images:
  - location: "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-arm64.qcow2"
    arch: "aarch64"
  - location: "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2"
    arch: "x86_64"
cpus: 2
memory: "4GiB"
disk: "20GiB"
mounts: []
YAML
  limactl start --name="$VM_NAME" --tty=false "$WORK/vm.yaml"
elif [ "$(limactl list "$VM_NAME" --format '{{.Status}}')" != "Running" ]; then
  say "Starting VM '$VM_NAME'"
  limactl start "$VM_NAME" --tty=false
fi

vm() { limactl shell "$VM_NAME" -- "$@"; }

say "Provisioning (clamav-daemon, bubblewrap; idempotent)"
vm bash -s <<'PROV'
set -euo pipefail
if ! command -v clamdscan >/dev/null || ! command -v bwrap >/dev/null; then
  sudo apt-get update -qq
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq clamav-daemon bubblewrap
fi
# clamd refuses to start without signature databases.
if [ ! -f /var/lib/clamav/daily.cvd ] && [ ! -f /var/lib/clamav/daily.cld ]; then
  sudo systemctl stop clamav-freshclam 2>/dev/null || true
  sudo freshclam --quiet || true
  sudo systemctl start clamav-freshclam 2>/dev/null || true
fi
sudo systemctl restart clamav-daemon
for _ in $(seq 1 60); do [ -S /var/run/clamav/clamd.ctl ] && break; sleep 2; done
[ -S /var/run/clamav/clamd.ctl ] || { echo "clamd socket never appeared"; exit 1; }
id rela >/dev/null 2>&1 || sudo useradd --system --home-dir /var/lib/rela --create-home --shell /usr/sbin/nologin rela
sudo usermod -aG clamav rela
sudo mkdir -p /var/lib/rela/project/entities/documents
PROV

say "Building rela-server for the VM"
ARCH="$(vm uname -m)"
case "$ARCH" in
  aarch64) GOARCH=arm64 ;;
  x86_64)  GOARCH=amd64 ;;
  *) echo "unsupported VM arch: $ARCH"; exit 1 ;;
esac
# The embedded SPA is irrelevant to scanning but its absence is a startup error,
# so provide a stub when no real frontend build is present.
STUB_SPA=""
if [ ! -f "$REPO_ROOT/internal/dataentry/static/v2/index.html" ]; then
  mkdir -p "$REPO_ROOT/internal/dataentry/static/v2"
  printf '<!doctype html><title>stub</title>\n' > "$REPO_ROOT/internal/dataentry/static/v2/index.html"
  STUB_SPA="$REPO_ROOT/internal/dataentry/static/v2"
  trap 'rm -rf "$WORK" "$STUB_SPA"' EXIT
fi
(cd "$REPO_ROOT" && GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build -o "$WORK/rela-server" ./cmd/rela-server)

say "Deploying binary, project and the guide's unit"
# The project uses ONLY the guide's scan_cmd — no scan_sockets — so the
# zero-config claim is what gets tested.
cat > "$WORK/schema.yaml" <<YAML
attachments:
  $SCAN_CMD

entities:
  document:
    label: Document
    id_prefix: DOC
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      body:
        type: file
YAML
cat "$WORK/rela-server" | vm bash -c 'cat > /tmp/rs && sudo install -m 0755 /tmp/rs /usr/local/bin/rela-server'
cat "$WORK/schema.yaml"          | vm bash -c 'sudo tee /var/lib/rela/project/schema.yaml >/dev/null'
cat "$WORK/rela-server.service"  | vm bash -c 'sudo tee /etc/systemd/system/rela-server.service >/dev/null'
vm bash -s <<'DEPLOY'
set -euo pipefail
sudo tee /var/lib/rela/project/data-entry.yaml >/dev/null <<'YAML'
version: "1.0"
app:
  name: "ClamAV Scan Test"
YAML
sudo tee /var/lib/rela/project/entities/documents/DOC-0001.md >/dev/null <<'MD'
---
id: DOC-0001
type: document
title: Scan target
---
MD
sudo chown -R rela:rela /var/lib/rela
printf 'a perfectly ordinary file\n' | sudo tee /tmp/clean.txt >/dev/null
# Assembled in two halves so this script is not itself a signature match.
printf 'X5O!P%%@AP[4\\PZX54(P^)7CC)7}$%s' 'EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*' \
  | sudo tee /tmp/eicar.txt >/dev/null
sudo systemctl daemon-reload
sudo systemctl reset-failed rela-server 2>/dev/null || true
sudo systemctl restart rela-server
sleep 5
DEPLOY

# journal_now prints only the CURRENT invocation's log. Plain `journalctl -n N`
# reaches back into previous restarts, where a warning may legitimately have
# fired — which made the "no spurious warning" assertion fail against a stale
# line from an earlier probe rather than the run under test.
journal_now() {
  vm bash -c 'inv="$(systemctl show -p InvocationID --value rela-server)"
              sudo journalctl -u rela-server --no-pager _SYSTEMD_INVOCATION_ID="$inv"'
}

upload() { # upload <file> -> HTTP status
  vm curl -s -o /dev/null -w '%{http_code}' -X POST \
    -F "file=@$1;type=text/plain" \
    http://127.0.0.1:8080/api/v1/documents/DOC-0001/_attachments/body
}

# ── Assertions ───────────────────────────────────────────────────────────────
say "The guide's unit + guide's recipe, with NO scan_sockets"

if [ "$(vm systemctl is-active rela-server)" = "active" ]; then
  pass "service started"
else
  fail "service did not start"
  journal_now | tail -20
fi

if journal_now | grep -q 'sandbox bubblewrap'; then
  pass "sandbox is bubblewrap (the documented unit does not break confinement)"
else
  fail "sandbox unavailable under the documented unit"
  journal_now | grep -i 'confinement' | tail -2
fi

code="$(upload /tmp/clean.txt)"
[ "$code" = "200" ] && pass "clean upload accepted (200)" \
                    || fail "clean upload got $code, want 200 (scanner unable to run?)"

code="$(upload /tmp/eicar.txt)"
[ "$code" = "422" ] && pass "EICAR upload rejected (422)" \
                    || fail "EICAR upload got $code, want 422"

if journal_now | grep -q 'a virus scan is configured but'; then
  fail "startup warned that scanning cannot run, but it can"
else
  pass "no spurious scan-cannot-run warning"
fi

# ── The three directives, each broken in isolation ───────────────────────────
# Each must (a) disable the sandbox, (b) still reject a CLEAN upload — proving
# fail-closed holds — and (c) produce the WARN naming the consequence.
say "Each hardening directive, broken in isolation"

for probe in \
  "RestrictNamespaces|RestrictNamespaces=yes" \
  "SystemCallFilter|SystemCallFilter=@system-service" \
  "RestrictAddressFamilies|RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6"
do
  key="${probe%%|*}"; bad="${probe#*|}"
  vm sudo sed -i "s|^$key=.*|$bad|" /etc/systemd/system/rela-server.service
  vm sudo systemctl daemon-reload
  vm sudo systemctl reset-failed rela-server 2>/dev/null || true
  vm sudo systemctl restart rela-server
  sleep 5

  log="$(journal_now)"
  clean_code="$(upload /tmp/clean.txt)"

  if grep -q 'sandbox unavailable' <<<"$log"; then
    pass "$key=… disables the sandbox (as documented)"
  else
    fail "$key=… did NOT disable the sandbox — the guide's claim is stale"
  fi

  if [ "$clean_code" = "422" ]; then
    pass "$key=… still fails closed (clean upload rejected, not stored unscanned)"
  else
    fail "$key=… clean upload got $clean_code — FAIL-CLOSED IS BROKEN"
  fi

  if grep -q 'a virus scan is configured but' <<<"$log"; then
    pass "$key=… warns that every upload will be rejected"
  else
    fail "$key=… no operator warning; the failure is silent"
  fi

  # Restore from the guide's unit so the next probe starts from a good state.
  cat "$WORK/rela-server.service" | vm bash -c 'sudo tee /etc/systemd/system/rela-server.service >/dev/null'
  vm sudo systemctl daemon-reload
done

say "Restoring the documented unit"
vm sudo systemctl reset-failed rela-server 2>/dev/null || true
vm sudo systemctl restart rela-server
sleep 5
code="$(upload /tmp/clean.txt)"
[ "$code" = "200" ] && pass "recovered: clean upload accepted again" \
                    || fail "did not recover after restoring the unit (got $code)"

echo
if [ "$FAILURES" -eq 0 ]; then
  printf '\033[32mAll checks passed.\033[0m VM left running — stop it with: limactl stop %s\n' "$VM_NAME"
else
  printf '\033[31m%d check(s) failed.\033[0m VM left running for inspection: limactl shell %s\n' "$FAILURES" "$VM_NAME"
  exit 1
fi
