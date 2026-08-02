import { type Locator, expect } from "@playwright/test";
import { BasePage } from "./base.page";

/** Escape a string for safe embedding in a RegExp (entity ids are hyphenated). */
function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** Page object for the relation history view
 *  (`/relation-history/:fromType/:from/:relType/:to`, RelationHistoryView.vue)
 *  and the per-relation History affordance on relation cards
 *  (RelationCards.vue, outgoing edges only). Selectors mirror the component's
 *  stable class names; see the template for `.timeline`, `.compare-select`,
 *  `.prop-diff`, etc. */
export class RelationHistoryPage extends BasePage {
  /** The History button rendered on an outgoing relation card whose target is
   *  `targetId`. Absent on incoming cards (the FROM entity owns the history).
   *  The `.entity-id` text is matched EXACTLY (anchored regex), not as a
   *  substring — with sequential ids a substring `FEAT-3` would also match the
   *  card for `FEAT-30`. */
  historyButtonForCard(targetId: string): Locator {
    const exactId = new RegExp(`^\\s*${escapeRegExp(targetId)}\\s*$`);
    return this.page
      .locator(".relation-card")
      .filter({ has: this.page.locator(".entity-id", { hasText: exactId }) })
      .locator(".history-btn");
  }

  /** Every History button in the currently-rendered relation widgets. */
  allHistoryButtons(): Locator {
    return this.page.locator(".relation-card .history-btn");
  }

  /** Open the history view by clicking the affordance on the outgoing card. */
  async openHistoryForCard(targetId: string) {
    await this.historyButtonForCard(targetId).click();
    await expect(this.page.locator(".history-view")).toBeVisible();
  }

  /** Wait until the timeline has at least `n` version rows (the sweep captures
   *  create/update asynchronously; the e2e server runs it on a short interval). */
  async waitForTimeline(n: number) {
    await expect(this.timelineItems()).toHaveCount(n, { timeout: 15_000 });
  }

  timelineItems(): Locator {
    return this.page.locator(".timeline .timeline-item");
  }

  async timelineCount(): Promise<number> {
    return this.timelineItems().count();
  }

  /** The op badges down the timeline, newest last (create/update/rename/delete). */
  async timelineOps(): Promise<string[]> {
    return this.page.locator(".timeline .timeline-badge").allTextContents();
  }

  async selectVersion(version: number) {
    await this.page
      .locator(`.timeline-item[data-version="${version}"] .timeline-select`)
      .click();
  }

  /** Choose the two compared versions in the diff pane and read back the
   *  rendered property changes. Accepts the `latest` sentinel, which is
   *  spelled `current` in the option value and the URL. */
  async compare(baseVersion: number | "current", targetVersion: number | "current") {
    const selects = this.page.locator(".compare-select");
    await selects.nth(0).selectOption(String(baseVersion));
    await selects.nth(1).selectOption(String(targetVersion));
  }

  /** The pair the dropdowns currently show. Reading the SELECT values (rather
   *  than the URL) is what catches a v-model type mismatch: a numeric side
   *  parsed as a string matches no <option> and leaves the control blank. */
  async selectedPair(): Promise<{ base: string; target: string }> {
    const selects = this.page.locator(".compare-select");
    return {
      base: await selects.nth(0).inputValue(),
      target: await selects.nth(1).inputValue(),
    };
  }

  /** The `?base=`/`?target=` params currently in the address bar (TKT-YOHC3N —
   *  the compared pair is mirrored into the URL so a diff is linkable). */
  urlSelection(): { base: string | null; target: string | null } {
    const params = new URL(this.page.url()).searchParams;
    return { base: params.get("base"), target: params.get("target") };
  }

  /** Wait until the URL reflects a specific pair (the write is a
   *  router.replace, so it lands a tick after the interaction). */
  async expectUrlSelection(base: number | "current", target: number | "current") {
    await expect
      .poll(() => this.urlSelection())
      .toEqual({ base: String(base), target: String(target) });
  }

  /** Assert the view rendered (used after a raw navigation). */
  async expectLoaded() {
    await expect(this.page.locator(".history-view")).toBeVisible();
  }

  /** Wait until the timeline has at least `n` rows, polling because
   *  create/update capture is asynchronous (the reconciliation sweep). */
  async waitForTimelineAtLeast(n: number) {
    await expect
      .poll(async () => this.timelineCount(), { timeout: 20_000 })
      .toBeGreaterThanOrEqual(n);
  }

  propDiffRows(): Locator {
    return this.page.locator(".prop-diff .prop-row");
  }

  async propDiffLabels(): Promise<string[]> {
    return this.page.locator(".prop-diff .prop-row .prop-label").allTextContents();
  }

  /** Click Restore on a specific version row (rows for op=delete have none). */
  async restoreVersion(version: number) {
    await this.page
      .locator(`.timeline-item[data-version="${version}"]`)
      .locator("button", { hasText: "Restore" })
      .click();
  }

  /** The "backend does not support version history" state (fsstore). */
  async expectUnsupported() {
    await expect(
      this.page.locator(".loading-state", { hasText: /does not support/i }),
    ).toBeVisible();
  }
}
