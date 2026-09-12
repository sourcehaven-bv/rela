---
id: RR-POACUV
type: review-response
title: Platform skip list incomplete and go1.x / ignore tags treated as opt-in
finding: 'scripts/check-tagged-tests.sh: the GOOS/GOARCH skip list omitted ppc, sparc, sparc64, riscv, mips64p32, mips64p32le and boringcrypto. Separately, `//go:build go1.24` would yield tag go1.24 (release tags come from the toolchain, so passing them via -tags is meaningless) and `//go:build ignore` conventionally means never build this file, yet the guard would try to vet it. None appear in the repo today.'
severity: minor
resolution: Skip list completed in scripts/tagged_build_tags.go, plus a go1.* prefix rule and an explicit `ignore` entry. Cases "go1.x release tag skipped" and "ignore tag skipped" added.
status: addressed
---
