#!/usr/bin/env bash
# Lists the OSV advisories that may affect rela's fork of the ygo relay
# (third_party/ygows, TKT-IBT44K5).
#
# Input on stdin: one or more OSV /v1/query responses for
# github.com/reearth/ygo at the fork's UpstreamVersion.
# Argument: an ignore file with one "<OSV id> <reason>" per line, for
# advisories the fork has already dealt with. "#" starts a comment.
#
# An advisory counts when it is not withdrawn, is not ignored, and names an
# import path in a forked package. When the script cannot tell which package
# an advisory touches, it counts too. That covers an advisory with no import
# paths (GHSA records carry none), an import entry without a path, and an
# advisory with no affected entry spelled exactly github.com/reearth/ygo.
#
# Output: "<id><TAB><summary>" per advisory. Exit 0 when there are none,
# 1 when there are some, 2 on bad input.
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: ygows-advisories.sh <ignore-file> < osv.json" >&2
  exit 2
fi
ignore_file=$1

ignored=$(sed -e 's/#.*//' "$ignore_file" | awk 'NF { print $1 }' | jq -R . | jq -s .) || exit 2

out=$(jq -rs --argjson ignored "$ignored" '
  def forked:
    . as $p
    | any(
        "github.com/reearth/ygo/provider/websocket",
        "github.com/reearth/ygo/internal/relaylane",
        "github.com/reearth/ygo/internal/roomname",
        "github.com/reearth/ygo/internal/updatecheck";
        $p == . or ($p | startswith(. + "/")));
  [ .[] | .vulns // [] | .[] ]
  | unique_by(.id)
  | .[]
  | select(.withdrawn == null)
  | select(.id as $id | $ignored | index($id) | not)
  | select(
      [ .affected[]?
        | select(.package.name == "github.com/reearth/ygo")
        | (.ecosystem_specific.imports // []) | map(.path) ]
      | length == 0 or any(length == 0 or any(.[]; . == null or forked)))
  | "\(.id)\t\(.summary // "(no summary)")"
') || exit 2

if [[ -z "$out" ]]; then
  exit 0
fi
printf '%s\n' "$out"
exit 1
