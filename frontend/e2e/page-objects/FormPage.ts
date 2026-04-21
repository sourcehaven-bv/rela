import { Page, Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Page object for Form views (create and edit).
 * Handles form interactions for entity creation and editing.
 */
export class FormPage extends BasePage {
  private formName: string

  constructor(page: Page, formName: string) {
    super(page)
    this.formName = formName
  }

  // Selectors

  /** The form container */
  get formContainer(): Locator {
    return this.page.locator('.dynamic-form').first()
  }

  /** The form element */
  get form(): Locator {
    return this.page.locator('.dynamic-form form').first()
  }

  /** Form header (shows when loaded successfully) */
  get formHeader(): Locator {
    return this.page.locator('.form-header')
  }

  /** Loading state indicator */
  get loadingState(): Locator {
    return this.page.locator('.dynamic-form .loading-state')
  }

  /** Error state (form not found) */
  get errorState(): Locator {
    return this.page.locator('.error-state')
  }

  /** Submit button */
  get submitButton(): Locator {
    return this.page.locator('button[type="submit"], .btn-submit, button:has-text("Save"), button:has-text("Create")').first()
  }

  /** Cancel button */
  get cancelButton(): Locator {
    return this.page.locator('button:has-text("Cancel"), .btn-cancel, a:has-text("Cancel")').first()
  }

  /** Form validation errors */
  get validationErrors(): Locator {
    return this.page.locator('.field-error, .error, .form-error, .validation-error, [data-error]')
  }

  /** Success message */
  get successMessage(): Locator {
    return this.page.locator('.success, .toast-success, [data-success]')
  }

  // Page navigation

  async goto(): Promise<void> {
    await this.page.goto(`/form/${this.formName}`)
  }

  async waitForLoad(): Promise<void> {
    await this.formContainer.waitFor({ state: 'visible', timeout: 10000 })
    // Wait for loading to complete and either form or error state to appear
    await this.page
      .locator('.dynamic-form form, .error-state')
      .first()
      .waitFor({ state: 'visible', timeout: 10000 })
  }

  // Actions

  /**
   * Fill a text field by label or name
   * Vue components use id="field-{property}" pattern
   */
  async fillField(fieldName: string, value: string): Promise<void> {
    const field = this.page.locator(`#field-${fieldName}, input[name="${fieldName}"], textarea[name="${fieldName}"]`).first()
    await field.fill(value)
  }

  /**
   * Fill a text field by its label text
   */
  async fillFieldByLabel(label: string, value: string): Promise<void> {
    const field = this.page.getByLabel(label)
    await field.fill(value)
  }

  /**
   * Select an option from a dropdown
   * Vue components use id="field-{property}" pattern
   */
  async selectOption(fieldName: string, value: string): Promise<void> {
    const select = this.page.locator(`#field-${fieldName}, select[name="${fieldName}"]`).first()
    await select.selectOption(value)
  }

  /**
   * Select an option from a SlimSelect dropdown
   */
  async selectSlimOption(fieldName: string, optionText: string): Promise<void> {
    // Click the SlimSelect container to open it
    const container = this.page.locator(`[data-field="${fieldName}"] .ss-main, .ss-main`).first()
    await container.click()
    // Click the option
    await this.page.locator(`.ss-option:has-text("${optionText}")`).first().click()
  }

  /**
   * Check a checkbox
   */
  async checkCheckbox(fieldName: string): Promise<void> {
    const checkbox = this.page.locator(`input[name="${fieldName}"][type="checkbox"], #${fieldName}`).first()
    await checkbox.check()
  }

  /**
   * Submit the form
   */
  async submit(): Promise<void> {
    await this.submitButton.click()
  }

  /**
   * Cancel and go back
   */
  async cancel(): Promise<void> {
    await this.cancelButton.click()
  }

  // State queries

  /**
   * Check if form has validation errors
   */
  async hasErrors(): Promise<boolean> {
    // Use first() to avoid strict mode violation with multiple error elements
    return this.validationErrors.first().isVisible()
  }

  /**
   * Check if form has validation error (alias for hasErrors)
   */
  async hasValidationError(): Promise<boolean> {
    return this.hasErrors()
  }

  /**
   * Check if a field exists in the form
   * Vue components use id="field-{property}" pattern
   */
  async hasField(fieldName: string): Promise<boolean> {
    const field = this.page.locator(`#field-${fieldName}, [name="${fieldName}"], [data-field="${fieldName}"]`).first()
    return field.isVisible()
  }

  /**
   * Check if submit button is visible
   */
  async hasSubmitButton(): Promise<boolean> {
    return this.submitButton.isVisible()
  }

  /**
   * Select a field value (alias for selectOption, tries SlimSelect first)
   * Vue components use id="field-{property}" pattern
   */
  async selectField(fieldName: string, value: string): Promise<void> {
    // Try regular select first (Vue uses field-{property} IDs)
    const select = this.page.locator(`select#field-${fieldName}, select[name="${fieldName}"]`).first()
    if (await select.isVisible()) {
      await select.selectOption(value)
      return
    }
    // Try SlimSelect
    await this.selectSlimOption(fieldName, value)
  }

  /**
   * Get all options from a select element
   * Vue components use id="field-{property}" pattern
   */
  async getSelectOptions(fieldName: string): Promise<string[]> {
    const select = this.page.locator(`select#field-${fieldName}, select[name="${fieldName}"]`).first()
    const options = await select.locator('option').allTextContents()
    return options.filter((o) => o.trim() !== '')
  }

  /**
   * Select the first available relation option
   */
  async selectFirstRelation(relationType: string): Promise<void> {
    // Try SlimSelect first
    const container = this.page.locator(`[data-field="${relationType}"] .ss-main, .ss-main`).first()
    if (await container.isVisible()) {
      await container.click()
      const option = this.page.locator('.ss-option').first()
      if (await option.isVisible()) {
        await option.click()
        return
      }
    }
    // Try regular select
    const select = this.page.locator(`select[name="${relationType}"]`).first()
    if (await select.isVisible()) {
      const options = await select.locator('option').all()
      if (options.length > 1) {
        await select.selectOption({ index: 1 })
      }
    }
  }

  /**
   * Get validation error messages
   */
  async getErrorMessages(): Promise<string[]> {
    const errors = await this.validationErrors.allTextContents()
    return errors
  }

  /**
   * Check if field is required
   */
  async isFieldRequired(fieldName: string): Promise<boolean> {
    const field = this.page.locator(`[name="${fieldName}"]`).first()
    return (await field.getAttribute('required')) !== null
  }

  /**
   * Get current value of a field
   * Vue components use id="field-{property}" pattern
   */
  async getFieldValue(fieldName: string): Promise<string> {
    const field = this.page.locator(`#field-${fieldName}, [name="${fieldName}"]`).first()
    return (await field.inputValue()) || ''
  }

  /**
   * Wait for the default RelationPicker widget to render. Use this after
   * navigating to a form that has at least one picker; it tolerates the
   * picker's async candidate-load by waiting on the input rather than on
   * any candidate being present.
   */
  async waitForRelationPicker(): Promise<void> {
    await this.page.locator('input[placeholder^="Search "]').first().waitFor({ state: 'visible', timeout: 10000 })
  }

  /**
   * Pick a relation target in the default RelationPicker widget. Types
   * the target id into the picker's search input and clicks the matching
   * dropdown item. Raises if no item matching targetId appears within the
   * timeout, which surfaces schema/metadata mismatches as clear failures
   * rather than silent no-ops.
   */
  async pickRelationTarget(targetType: string, targetId: string): Promise<void> {
    const search = this.page.locator(`input[placeholder^="Search ${targetType}"]`).first()
    await search.fill(targetId)
    await this.page.locator(`.dropdown-item:has-text("${targetId}")`).first().click({ timeout: 5000 })
  }

  /**
   * Submit the form and wait for the PATCH that the Save click triggers
   * on the named entity, returning the response for assertions. Only
   * valid for edit forms — create forms POST a different URL shape.
   */
  async saveAndWaitForPatch(plural: string, entityId: string): Promise<import('@playwright/test').Response> {
    const saved = this.page.waitForResponse(
      (r) => r.url().includes(`/api/v1/${plural}/${entityId}`) && r.request().method() === 'PATCH',
    )
    await this.submit()
    return saved
  }
}

/**
 * Factory function to create FormPage for different form types
 */
export function createFormPage(page: Page, formName: string): FormPage {
  return new FormPage(page, formName)
}

/**
 * Specialized page for ticket creation form
 */
export class CreateTicketFormPage extends FormPage {
  constructor(page: Page) {
    super(page, 'create_ticket')
  }

  /**
   * Fill the ticket form with common fields
   */
  async fillTicketForm(data: {
    title: string
    description?: string
    status?: string
    priority?: string
    reporter?: string
    categoryId?: string
  }): Promise<void> {
    await this.fillField('title', data.title)
    if (data.description) {
      await this.fillField('description', data.description)
    }
    if (data.status) {
      await this.selectOption('status', data.status)
    }
    if (data.priority) {
      await this.selectOption('priority', data.priority)
    }
    if (data.reporter) {
      await this.fillField('reporter', data.reporter)
    }
    if (data.categoryId) {
      await this.selectSlimOption('belongs-to', data.categoryId)
    }
  }
}

/**
 * Specialized page for ticket edit form
 */
export class EditTicketFormPage extends FormPage {
  private ticketId: string

  constructor(page: Page, ticketId: string) {
    super(page, `edit_ticket/${ticketId}`)
    this.ticketId = ticketId
  }
}
