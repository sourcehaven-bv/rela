---
id: build-desktop-frontend-dep
type: automated-measure
title: build-desktop depends on build-frontend
description: The just build-desktop and build-desktop-debug recipes depend on build-frontend, and the desktop CI release job builds the Vue SPA before compiling the binary, so the embedded static/v2 directory always contains the SPA.
kind: ci
location: justfile, .github/workflows/release.yml
status: active
---
