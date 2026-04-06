---
id: TKT-CCUG
type: ticket
title: Add npm install step to frontend build
kind: chore
priority: low
effort: xs
status: done
---

The justfile `build-frontend` recipe assumed `node_modules` was already
installed. Added an `install-frontend` recipe as a dependency so `npm install`
runs automatically before building.
