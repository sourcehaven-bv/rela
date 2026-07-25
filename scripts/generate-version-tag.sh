#!/usr/bin/env bash
set -euo pipefail

# Generate a CalVer release tag.
#
# Usage: generate-version-tag.sh [--alpha]
#
# Format: vYY.M.BUILD
#   - YY     two-digit UTC year
#   - M      month, no leading zero (1-12)
#   - BUILD  0 for the month's first release, then 1, 2, ...
#   - --alpha appends -alpha, which GoReleaser's `prerelease: auto` picks up
#
#   v26.7.0        first release of July 2026
#   v26.7.1        second release that month
#   v26.8.0        first release of August
#
# Why this shape? Two hard constraints, satisfied without any remapping:
#
#   1. GoReleaser enforces semantic versioning and errors on non-compliant
#      tags, and rela's whole pipeline is GoReleaser-driven. vYY.M.BUILD is
#      valid semver with all three fields carrying real values.
#
#   2. Windows MSI ProductVersion is major.minor.build with maxima
#      255 / 255 / 65535. YY <= 99 and M <= 12 fit trivially, so the tag is
#      used verbatim in the installer — no separate MSI version to derive.
#
# Constraint 2 is what rules out openvwr's vYYYYMMDD, which rela's scheme is
# otherwise adapted from. That format is fine for GoReleaser (it parses as
# major=20260725 and orders correctly), but the 20260725 major blows the MSI
# 255 cap and `wix build` fails. openvwr never hits this: it ships a single
# PHP tar.gz where the version is only a filename label, never a parsed
# version field. rela ships .msi/.dmg/.deb/.rpm installers.
#
# The month is not zero-padded because semver forbids leading zeros in a
# numeric component (v26.07.0 would be non-canonical).
#
# The day is deliberately not in the tag; `git log -1 <tag>` and the GitHub
# release date carry it. The build counter resets each month, so it stays a
# small readable number rather than an encoded date.

usage() {
  echo "Usage: $0 [--alpha]" >&2
}

ALPHA=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --alpha)
      ALPHA=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

YEAR=$(date -u +%y)
# %-m strips the leading zero; GNU and BSD date both support it.
MONTH=$(date -u +%-m)
BASE="v${YEAR}.${MONTH}"

# Best-effort: in CI the checkout already has tags, and a failure here must not
# silently restart the counter at 0 over an existing tag.
git fetch --tags --quiet 2>/dev/null || true

# Highest build number already used this month, across both stable and -alpha
# tags: an alpha and a stable release must never land on the same tag string.
HIGHEST=$(git tag -l "${BASE}.*" \
  | sed -nE "s/^${BASE//./\\.}\.([0-9]+)(-alpha)?$/\1/p" \
  | sort -n \
  | tail -1 || true)

if [ -z "$HIGHEST" ]; then
  BUILD=0
else
  BUILD=$((HIGHEST + 1))
fi

VERSION="${BASE}.${BUILD}"

if [ "$ALPHA" = true ]; then
  VERSION="${VERSION}-alpha"
fi

echo "$VERSION"
