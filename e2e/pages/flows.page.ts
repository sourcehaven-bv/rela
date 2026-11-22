import { type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * The flow runs on an entity page and the flow views (TKT-ZAXRXND).
 *
 * Selectors mirror the components that render them: `FlowRunsPanel.vue`
 * (`flow-runs-panel`, `flow-start-<name>`), `FlowRunCard.vue` (`flow-run`,
 * `flow-run-state`, `flow-run-status`), `FlowWaiting.vue`
 * (`flow-waiting-roles`, `flow-branch`) and `FlowStepForm.vue`
 * (`flow-step-form`, `flow-complete`).
 */
export class FlowsPage extends BasePage {
  panel(): Locator {
    return this.page.getByTestId('flow-runs-panel');
  }

  /** The run cards on the page, newest first. */
  runs(): Locator {
    return this.page.getByTestId('flow-run');
  }

  /** Open `/entity/<type>/<id>` and wait for its header. */
  async openEntity(type: string, id: string) {
    await this.navigateTo(`/entity/${type}/${id}`);
    await this.waitForSpinnerToDisappear();
    await expect(this.page.locator('main h1').first()).toBeVisible({ timeout: 15_000 });
  }

  /** Click the Start button of `flow` in the runs panel. */
  async start(flow: string) {
    await this.panel().getByTestId(`flow-start-${flow}`).click();
  }

  async expectStartNotOffered(flow: string) {
    await expect(this.panel().getByTestId(`flow-start-${flow}`)).toHaveCount(0, { timeout: 15_000 });
  }

  async expectStartOffered(flow: string, label: string) {
    await expect(this.panel().getByTestId(`flow-start-${flow}`)).toContainText(label, {
      timeout: 15_000,
    });
  }

  /** Waits for exactly one run card, in `state`. */
  async expectOneRunIn(state: string, timeout = 15_000) {
    await expect(this.runs()).toHaveCount(1, { timeout });
    await expect(this.runs().first().getByTestId('flow-run-state')).toHaveText(state, { timeout });
  }

  async expectStatus(status: string, timeout = 15_000) {
    await expect(this.runs().first().getByTestId('flow-run-status')).toContainText(status, {
      timeout,
    });
  }

  /** Waits for the "why it waits" line to name `role` and say whether the reader holds it. */
  async expectWaitingFor(role: string, youHold: boolean) {
    const line = this.runs().first().getByTestId('flow-waiting-roles');
    await expect(line).toContainText(role);
    await expect(line).toContainText(
      youHold ? 'You have one of these roles.' : 'You do not have one of these roles.',
    );
  }

  stepForm(): Locator {
    return this.runs().first().getByTestId('flow-step-form');
  }

  /**
   * The top of the step form is inside the viewport once the page has
   * settled: `?run=` keeps the form in view while the panels above it render.
   */
  async expectStepFormTopInView(): Promise<void> {
    const form = this.stepForm();
    await expect(form).toBeVisible();
    await expect
      .poll(async () => {
        const top = await form.evaluate((el) => el.getBoundingClientRect().top);
        const height = await this.page.evaluate(() => window.innerHeight);
        // The bug left the form at the bottom edge; allow for a short page.
        return top >= 0 && top < height - 150;
      })
      .toBe(true);
  }

  /** Ticks the checkbox labelled `field` in the step form and completes the step. */
  async completeStepWithCheckbox(field: string) {
    const box = this.stepForm().getByRole('checkbox', { name: field });
    await expect(box).toBeVisible({ timeout: 15_000 });
    await box.check();
    await this.stepForm().getByTestId('flow-complete').click();
  }

  /**
   * Waits for the step form to show `labels` in order, each field below the
   * previous one and as wide as the form.
   */
  async expectStepFieldsStacked(labels: readonly string[]) {
    const fields = this.stepForm().locator('.form-field');
    await expect(fields).toHaveCount(labels.length, { timeout: 15_000 });
    const form = await this.stepForm().boundingBox();
    let bottom = 0;
    for (const [i, label] of labels.entries()) {
      await expect(fields.nth(i).locator('label').first()).toContainText(label);
      const box = await fields.nth(i).boundingBox();
      expect(box && form, `field ${label} has a box`).toBeTruthy();
      expect(box!.y, `${label} sits below the field before it`).toBeGreaterThanOrEqual(bottom);
      expect(box!.width, `${label} spans the form`).toBeGreaterThan(form!.width * 0.9);
      bottom = box!.y + box!.height;
    }
  }

  /** Checks that the run shows none of the flow's workings: roles, guard, diagram, timeline or cancel. */
  async expectNoManagerDetails(guard: string) {
    const run = this.runs().first();
    await expect(run.getByTestId('flow-waiting-roles')).toHaveCount(0);
    await expect(run.getByTestId('flow-branch')).toHaveCount(0);
    await expect(run).not.toContainText(guard);
    await expect(run.locator('details')).toHaveCount(0);
    await expect(run.getByTestId('flow-cancel')).toHaveCount(0);
  }

  async expectCancelOffered() {
    await expect(this.runs().first().getByTestId('flow-cancel')).toBeVisible();
  }

  /** Ticks the checkbox labelled `field`, fills `text` into the field labelled `textField`, and completes the step. */
  async completeStep(field: string, textField: string, text: string) {
    await this.stepForm().getByLabel(textField).fill(text);
    await this.completeStepWithCheckbox(field);
  }

  async expectNoStepForm() {
    await expect(this.page.getByTestId('flow-step-form')).toHaveCount(0);
  }
}
