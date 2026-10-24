#!/usr/bin/env bash
# Prints a -keep regexp for `seqtrace diagram` and `seqtrace stitch` that
# matches every function declared with //archplan:guard= under internal/, so
# a guard called inside the handler's own package is still drawn. Names are
# qualified by receiver as seqtrace records them: "(*Request).PermitsRead"
# for a pointer receiver, "gatedAuthorizer.Authorize" for a value receiver,
# "readAddressedOr404" for a function. Prints nothing when there are none.
set -euo pipefail
cd "$(dirname "$0")/../.."
names=$(grep -rh -A1 '^//archplan:guard=' internal --include='*.go' |
    sed -nE \
        -e 's/^func \([A-Za-z0-9_]* *\*([A-Za-z0-9_]+)(\[[^]]*\])?\) ([A-Za-z0-9_]+).*/\\(\\*\1\\)\\.\3/p' \
        -e 's/^func \(([A-Za-z0-9_]+ +)?([A-Za-z0-9_]+)(\[[^]]*\])?\) ([A-Za-z0-9_]+).*/\2\\.\4/p' \
        -e 's/^func ([A-Za-z0-9_]+).*/\1/p' |
    sort -u | paste -sd'|' -)
[[ -n $names ]] && printf '(^|\\.)(%s)$\n' "$names"
exit 0
