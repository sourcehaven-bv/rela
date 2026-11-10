import { type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * The flow runs on an entity page and the flow views (TKT-ZAXRXND).
 *
 * Selectors mirror the components that render them: `FlowRunsPanel.vue`
 * (`flow-runs-panel`, `flow-start-<name>`), `FlowRunCard.vue` (`flow-run`,
 * `flow-run-state`, `flow-run-status`), `FlowWaiting.vue`
 * (`flow-waiting-roles`) and `FlowStepForm.vue` (`flow-step-form`,
 * `flow-complete`).
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

  /** Ticks the checkbox labelled `field` in the step form and completes the step. */
  async completeStepWithCheckbox(field: string) {
    const box = this.stepForm().getByRole('checkbox', { name: field });
    await expect(box).toBeVisible({ timeout: 15_000 });
    await box.check();
    await this.stepForm().getByTestId('flow-complete').click();
  }

  async expectNoStepForm() {
    await expect(this.page.getByTestId('flow-step-form')).toHaveCount(0);
  }
}
