---
id: pin-dotnet-tool-versions
type: automated-measure
title: Pin dotnet/global tool versions in release workflow
description: All `dotnet tool install --global` invocations in the release workflow must pass an explicit `--version` so a silent upstream major bump cannot break a release. WiX is currently pinned to 6.0.1.
kind: ci
location: .github/workflows/release.yml
status: active
---
