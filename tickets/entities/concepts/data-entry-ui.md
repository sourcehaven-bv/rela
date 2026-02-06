---
id: data-entry-ui
type: concept
title: Data Entry Web UI
summary: HTMX-powered web interface for entity/relation management
description: |
  Config-driven web application for data entry operations. Built with:
  - Go HTML templates (internal/dataentry/templates.go)
  - HTMX for dynamic updates
  - CSS-in-Go embedded styles
  - JavaScript enhancements (SlimSelect, sticky detection, etc.)

  Key UI components:
  - Sidebar navigation (fixed)
  - Page header with title and actions
  - Filter bars for list views
  - Jump bars for entity navigation
  - Detail views with markdown rendering
layer: server
package: internal/dataentry
status: stable
---
