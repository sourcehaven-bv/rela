---
id: BUG-GGSY
type: bug
title: v0.9 release blocked by WiX v7 OSMF EULA prompt
description: 'The v0.9 release pipeline failed at the Windows MSI step with `error WIX7015: You must accept the Open Source Maintenance Fee (OSMF) EULA to use WiX Toolset v7`. The release.yml workflow installed wix as `dotnet tool install --global wix` (unpinned), which now resolves to WiX v7 and refuses to build without an interactive EULA acceptance.'
priority: high
effort: xs
why1: The release workflow installed `wix` as a dotnet tool without a version pin, so a fresh runner picked up WiX v7.
why2: WiX v7 introduced a new OSMF EULA gate (WIX7015) that blocks `wix build` until the user accepts the license, which is not possible in headless CI.
why3: We pinned neither the WiX major version nor the dotnet tool version, so any breaking change in the upstream tool reaches release builds immediately.
why4: There is no policy/convention requiring explicit pinning of build-time toolchain dependencies installed by the release workflow.
why5: Release-only tooling (only exercised when cutting a tag) lacks the same supply-chain hygiene we apply to compile-time dependencies in go.mod.
prevention: Pinned `wix` to v6.0.1 in .github/workflows/release.yml so the existing `wix build` invocation keeps working without an EULA prompt. Going forward, all dotnet/global tool installs in workflows should pass an explicit `--version` to avoid silent major-version upgrades.
status: done
---
