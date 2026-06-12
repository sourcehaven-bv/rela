---
id: RR-FC1B
type: review-response
title: 'S1: click propagation — pick one strategy'
finding: |
  Plan mooted two approaches (`@click.stop` on widget cells vs. move navigation to a more specific subelement) without committing. The risks section says "move handler"; the Technical Approach section says "@click.stop". Implementer shouldn't have to choose between two abstraction-coupling shapes.
severity: significant
status: addressed
resolution: |
  PLAN commits to: move `navigateToEntity` off the row-level `<article>`/`<li>` onto the title/header element. This keeps SectionEditForm uncoupled from its host's navigation semantics (no `@click.stop` inside the form). The card's `.card-header` and the list item's `.list-link` already exist as natural navigation targets. Both have a `cursor: pointer` style already implied by their interactive role.

  AC 8 amended: assert click on a widget input does NOT trigger navigation (host moved the handler) AND click on the card's title DOES navigate.
---
