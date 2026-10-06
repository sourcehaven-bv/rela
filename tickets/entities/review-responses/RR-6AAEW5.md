---
id: RR-6AAEW5
type: review-response
title: Switcher used undefined --radius and --shadow with literal fallbacks
finding: ProjectSwitcher and WorldSwitcher read var(--radius, 6px) and var(--shadow, ...), which are defined nowhere, so the dark-theme shadow never applied.
severity: minor
resolution: Replaced with --radius-md and --shadow-md from scales.css.
status: addressed
---
