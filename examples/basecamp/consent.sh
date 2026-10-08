#!/usr/bin/env bash
# Get a Basecamp refresh token for rela, once per connection.
#
#   BASECAMP_CLIENT_ID=... BASECAMP_REDIRECT_URI=https://example.com/callback \
#     ./consent.sh | rela token set basecamp
#
# It asks for the client secret without echo, unless BASECAMP_CLIENT_SECRET
# is set. Do not type the secret on the command line: it would land in
# your shell history.
#
# It prints an authorization URL. Open it, approve, and paste the `code`
# parameter from the address your browser is sent to. The code is read
# without echo. The script then exchanges it for tokens and lists the
# Basecamp accounts the token can reach, so you can pick the
# basecamp_account_id secret.
#
# The token JSON goes to stdout and everything else to stderr, so a pipe
# into `rela token set` carries only the token. Without a pipe it prints
# the JSON; paste it into the desktop app (Settings, Connections).
#
# Needs bash, curl and jq.
set -euo pipefail

: "${BASECAMP_CLIENT_ID:?set BASECAMP_CLIENT_ID}"
: "${BASECAMP_REDIRECT_URI:?set BASECAMP_REDIRECT_URI to the redirect URI registered for the app}"
if [ -z "${BASECAMP_CLIENT_SECRET:-}" ]; then
  printf 'Client secret: ' >&2
  IFS= read -rs BASECAMP_CLIENT_SECRET </dev/tty
  echo >&2
  [ -n "$BASECAMP_CLIENT_SECRET" ] || { echo "consent.sh: no client secret entered" >&2; exit 1; }
fi
export BASECAMP_CLIENT_ID BASECAMP_CLIENT_SECRET BASECAMP_REDIRECT_URI
# Basecamp refuses a request whose User-Agent names no contact.
USER_AGENT="${BASECAMP_USER_AGENT:-rela-basecamp-example (ops@example.com)}"
LAUNCHPAD="https://launchpad.37signals.com"

for tool in curl jq; do
  command -v "$tool" >/dev/null || { echo "consent.sh: $tool is required" >&2; exit 1; }
done

uri=$(jq -rn 'env.BASECAMP_REDIRECT_URI|@uri')
cid=$(jq -rn 'env.BASECAMP_CLIENT_ID|@uri')
{
  echo "Open this URL, approve, and copy the code parameter from the address you land on:"
  echo
  echo "  $LAUNCHPAD/authorization/new?type=web_server&client_id=$cid&redirect_uri=$uri"
  echo
  printf 'Code: '
} >&2
IFS= read -rs code </dev/tty
echo >&2
[ -n "$code" ] || { echo "consent.sh: no code entered" >&2; exit 1; }

# The secret and the code reach jq through the environment and curl through
# stdin, never as arguments, so they stay out of `ps`.
export BASECAMP_CODE="$code"
tokens=$(
  jq -rn '"type=web_server&code=\(env.BASECAMP_CODE|@uri)&client_id=\(env.BASECAMP_CLIENT_ID|@uri)" +
    "&client_secret=\(env.BASECAMP_CLIENT_SECRET|@uri)&redirect_uri=\(env.BASECAMP_REDIRECT_URI|@uri)"' |
    curl -fsS -A "$USER_AGENT" --data-binary @- "$LAUNCHPAD/authorization/token"
) || { echo "consent.sh: the token exchange failed; codes are single-use, so start again" >&2; exit 1; }

access=$(jq -r '.access_token // empty' <<<"$tokens")
[ -n "$access" ] || { echo "consent.sh: the answer held no access token" >&2; exit 1; }

# The header goes through stdin too, so the token stays out of `ps`.
echo "Basecamp accounts this token reaches (use the id as basecamp_account_id):" >&2
printf 'header = "Authorization: Bearer %s"\n' "$access" |
  curl -fsS -A "$USER_AGENT" -K - "$LAUNCHPAD/authorization.json" |
  jq -r '.accounts[] | select(.product == "bc3") | "  \(.id)  \(.name)"' >&2 ||
  echo "  (could not list accounts)" >&2

if [ -t 1 ]; then
  echo "The next line holds your tokens. Paste it into the desktop app, then clear the screen." >&2
fi
jq -c '{refresh_token, access_token, expires_in}' <<<"$tokens"
