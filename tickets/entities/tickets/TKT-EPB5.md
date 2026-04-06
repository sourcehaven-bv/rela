---
id: TKT-EPB5
type: ticket
title: Add palette import helper with smart color assignment
kind: enhancement
priority: medium
effort: l
status: done
---

# Add Palette Import Helper with Smart Color Assignment

## Problem

Setting up a custom palette requires manually picking and assigning 8 theme
colors + 7 badge colors. Users who find a palette on Lospec or similar sites
have to manually map each color to a role. This is tedious and error-prone.

## Goal

Provide a palette import flow that:
1. Accepts a palette in common formats (hex list, GPL, etc.)
2. Automatically assigns colors to roles using heuristic algorithms
3. Shows the full imported palette as swatches
4. Lets users click swatches to assign/reassign individual colors
5. Produces a good-enough starting point that users can fine-tune
