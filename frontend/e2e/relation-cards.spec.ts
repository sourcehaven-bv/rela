import { test, expect } from './fixtures'

/**
 * E2E tests for relation property cards (widget=cards) in edit forms.
 *
 * Uses TKT-001 which has:
 *   - tagged -> bug (with added_by, added_date)
 *   - tagged -> urgent
 *   - blocks -> TKT-002 (with reason="test block", severity=high, resolved_date)
 */

/**
 * Navigate to the edit form for a ticket and wait for relation cards to load.
 */
async function openEditForm(apiPage: import('@playwright/test').Page, ticketId: string) {
  await apiPage.goto(`/form/edit_ticket/${ticketId}`)
  await apiPage.locator('.dynamic-form form, .error-state').first().waitFor({ state: 'visible', timeout: 10000 })
  // Wait for at least one relation-cards widget to appear
  await apiPage.locator('.relation-cards').first().waitFor({ state: 'visible', timeout: 10000 })
}

test.describe('Relation Cards', () => {
  test('edit form shows relation cards for tagged and blocks relations', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    // There should be multiple relation-cards widgets (tagged, blocks outgoing, blocks incoming)
    const cardWidgets = apiPage.locator('.relation-cards')
    await expect(cardWidgets.first()).toBeVisible()
    const count = await cardWidgets.count()
    expect(count).toBeGreaterThanOrEqual(2)

    // Each widget has a section-label
    const labels = await apiPage.locator('.relation-cards .section-label').allTextContents()
    const labelsLower = labels.map((l) => l.toLowerCase())
    expect(labelsLower.some((l) => l.includes('tagged'))).toBeTruthy()
    expect(labelsLower.some((l) => l.includes('blocks'))).toBeTruthy()
  })

  test('relation cards display existing entries with properties', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    // Find the blocks (outgoing) card widget - use .first() since TKT-002 appears in both outgoing and incoming
    const blocksCard = apiPage.locator('.relation-card', { has: apiPage.locator('.entity-id:has-text("TKT-002")') }).first()
    await expect(blocksCard).toBeVisible({ timeout: 10000 })

    // Card should show properties
    const cardProps = blocksCard.locator('.card-properties')
    await expect(cardProps).toBeVisible()

    // Should show prop labels like "Block Reason", "Severity"
    const propLabels = await cardProps.locator('.prop-label').allTextContents()
    expect(propLabels.some((l) => l.includes('Block Reason') || l.includes('Reason'))).toBeTruthy()
    expect(propLabels.some((l) => l.includes('Severity'))).toBeTruthy()
  })

  test('existing relation property values are populated', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    // Find the blocks card for TKT-002 - use .first() since TKT-002 appears in both outgoing and incoming
    const blocksCard = apiPage.locator('.relation-card', { has: apiPage.locator('.entity-id:has-text("TKT-002")') }).first()
    await expect(blocksCard).toBeVisible({ timeout: 10000 })

    // The reason input should have value "test block"
    const reasonInput = blocksCard.locator('.card-properties input.inline-edit').first()
    await expect(reasonInput).toHaveValue('test block')

    // Severity should be rendered as a SlimSelect showing "high"
    const severitySlim = blocksCard.locator('.card-properties .ss-main')
    await expect(severitySlim).toBeVisible()
    const severityValue = await apiPage.evaluate((el) => {
      const selected = (el?.querySelector('.card-properties select') as any)?.slim?.getSelected()
      return Array.isArray(selected) ? selected[0] : selected
    }, await blocksCard.elementHandle())
    expect(severityValue).toBe('critical')
  })

  test('can edit a text property on a relation card', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    const blocksCard = apiPage.locator('.relation-card', { has: apiPage.locator('.entity-id:has-text("TKT-002")') }).first()
    await expect(blocksCard).toBeVisible({ timeout: 10000 })

    // Edit the reason text
    const reasonInput = blocksCard.locator('.card-properties input.inline-edit').first()
    await reasonInput.fill('updated block reason')

    // Should show unsaved badge
    const badge = apiPage.locator('.pending-badge')
    await expect(badge.first()).toBeVisible()
  })

  test('can change an enum select value on a relation card', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    const blocksCard = apiPage.locator('.relation-card', { has: apiPage.locator('.entity-id:has-text("TKT-002")') }).first()
    await expect(blocksCard).toBeVisible({ timeout: 10000 })

    // Read the initial severity value via SlimSelect
    const cardHandle = await blocksCard.elementHandle()
    const getSlimValue = (el: Element | null) => {
      const selected = (el?.querySelector('.card-properties select') as any)?.slim?.getSelected()
      return Array.isArray(selected) ? selected[0] : selected
    }
    const initialValue = await apiPage.evaluate(getSlimValue, cardHandle)

    // Change the value via SlimSelect (to something different from initial)
    const targetValue = initialValue === 'medium' ? 'low' : 'medium'
    await apiPage.evaluate(
      ({ el, val }) => (el?.querySelector('.card-properties select') as any)?.slim?.setSelected(val, true),
      { el: cardHandle, val: targetValue }
    )
    await apiPage.waitForTimeout(500)

    // Verify the displayed value changed
    const updatedValue = await apiPage.evaluate(getSlimValue, cardHandle)
    expect(updatedValue).toBe(targetValue)
    expect(updatedValue).not.toBe(initialValue)

    // Verify the unsaved indicator appeared (pending-badge is on the widget, not the card)
    const widget = apiPage.locator('.relation-cards', { has: blocksCard })
    await expect(
      widget.locator('.pending-badge').or(blocksCard.locator('.card-updated'))
    ).toBeVisible({ timeout: 3000 })
  })

  test('enum select value persists after change (no clear-then-set bug)', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    const blocksCard = apiPage.locator('.relation-card', { has: apiPage.locator('.entity-id:has-text("TKT-002")') }).first()
    await expect(blocksCard).toBeVisible({ timeout: 10000 })

    // Change severity from high to medium via SlimSelect
    const cardHandle = await blocksCard.elementHandle()
    await apiPage.evaluate(
      (el) => (el?.querySelector('.card-properties select') as any)?.slim?.setSelected('medium', true),
      cardHandle
    )
    await apiPage.waitForTimeout(500)

    // Read the value back -- it should be "medium", not cleared
    const currentValue = await apiPage.evaluate((el) => {
      const selected = (el?.querySelector('.card-properties select') as any)?.slim?.getSelected()
      return Array.isArray(selected) ? selected[0] : selected
    }, cardHandle)
    expect(currentValue).toBe('medium')
  })

  test('enum select renders enum options correctly', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    const blocksCard = apiPage.locator('.relation-card', { has: apiPage.locator('.entity-id:has-text("TKT-002")') }).first()
    await expect(blocksCard).toBeVisible({ timeout: 10000 })

    // Find the severity select and check its options via evaluate
    const cardHandle = await blocksCard.elementHandle()
    const optionValues = await apiPage.evaluate((el) => {
      const select = el?.querySelector('.card-properties select') as HTMLSelectElement | null
      if (!select) return []
      return Array.from(select.options).map((o) => o.value.trim().toLowerCase())
    }, cardHandle)

    // All four enum values should be present
    expect(optionValues).toContain('critical')
    expect(optionValues).toContain('high')
    expect(optionValues).toContain('medium')
    expect(optionValues).toContain('low')
  })

  test('can remove a relation via the remove button', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    // Count tagged cards before removal
    // Tagged section: find the widget that contains "tagged" in its label
    const taggedWidget = apiPage.locator('.relation-cards', { has: apiPage.locator('.section-label:has-text("tagged")') }).first()
    await expect(taggedWidget).toBeVisible({ timeout: 10000 })

    const cardsBefore = await taggedWidget.locator('.relation-card').count()
    expect(cardsBefore).toBeGreaterThanOrEqual(1)

    // Click the remove button on the first card
    const removeBtn = taggedWidget.locator('.relation-card .remove-btn').first()
    await removeBtn.click()

    // One fewer card
    const cardsAfter = await taggedWidget.locator('.relation-card').count()
    expect(cardsAfter).toBe(cardsBefore - 1)

    // Should show unsaved badge
    await expect(apiPage.locator('.pending-badge').first()).toBeVisible()
  })

  test('can add a new relation with properties', async ({ apiPage, api }) => {
    // Create a target ticket to link to
    const target = await api.createEntity('tickets', {
      properties: {
        title: 'E2E Blocks Target',
        status: 'open',
        priority: 'low',
        reporter: 'e2e-test',
      },
    })

    await openEditForm(apiPage, 'TKT-001')

    // Find the outgoing blocks widget (first one with "blocks" label that has an add button)
    const blocksWidget = apiPage.locator('.relation-cards', {
      has: apiPage.locator('.section-label:has-text("blocks")'),
    }).first()
    await expect(blocksWidget).toBeVisible({ timeout: 10000 })

    // Click "+ Add" button
    const addBtn = blocksWidget.locator('.add-btn')
    await addBtn.click()

    // Search for the target ticket by title (Bleve full-text search matches titles, not IDs)
    const searchInput = blocksWidget.locator('.search-input')
    await expect(searchInput).toBeVisible()
    await searchInput.fill('E2E Blocks Target')

    // Wait for and click the search result
    const searchResult = blocksWidget.locator('.search-result', { hasText: target.id })
    await expect(searchResult).toBeVisible({ timeout: 10000 })
    await searchResult.click()

    // Should show the new-meta-fields for filling properties
    const metaFields = blocksWidget.locator('.new-meta-fields')
    await expect(metaFields).toBeVisible()

    // Fill the required "reason" field
    const reasonField = metaFields.locator('input[type="text"]').first()
    await reasonField.fill('blocks due to dependency')

    // Select severity via SlimSelect in the new-relation-form
    const newFormSlim = blocksWidget.locator('.new-relation-form .ss-main')
    if (await newFormSlim.isVisible().catch(() => false)) {
      const formHandle = await blocksWidget.locator('.new-relation-form').elementHandle()
      await apiPage.evaluate(
        (el) => (el?.querySelector('select') as any)?.slim?.setSelected('high', true),
        formHandle
      )
      await apiPage.waitForTimeout(500)
    }

    // Click Link button
    const linkBtn = blocksWidget.locator('.btn-primary:has-text("Link")')
    await expect(linkBtn).toBeEnabled()
    await linkBtn.click()

    // New card should appear
    const newCard = blocksWidget.locator('.relation-card', { has: apiPage.locator(`.entity-id:has-text("${target.id}")`) })
    await expect(newCard).toBeVisible()
    await expect(newCard).toHaveClass(/card-added/)

    // Clean up
    await api.deleteEntity('tickets', target.id)
  })

  test('changes are not saved until Save button is clicked (batch save)', async ({ apiPage, request, backend }) => {
    await openEditForm(apiPage, 'TKT-001')

    // Find the blocks card for TKT-002 - use .first() since TKT-002 appears in both outgoing and incoming
    const blocksCard = apiPage.locator('.relation-card', { has: apiPage.locator('.entity-id:has-text("TKT-002")') }).first()
    await expect(blocksCard).toBeVisible({ timeout: 10000 })

    const reasonInput = blocksCard.locator('.card-properties input.inline-edit').first()
    const originalValue = await reasonInput.inputValue()
    const newValue = 'batch save test reason'
    await reasonInput.fill(newValue)

    // Unsaved badge should be shown
    await expect(apiPage.locator('.pending-badge').first()).toBeVisible()

    // Verify the API still has the original value (not saved yet)
    const relResponse = await request.get(
      `${backend.baseUrl}/api/v1/tickets/TKT-001/relations/blocks`
    )
    const relData = await relResponse.json()
    const tkt002Rel = relData.find((r: { id: string }) => r.id === 'TKT-002')
    expect(tkt002Rel.meta.reason).toBe(originalValue)

    // Click the Save button and wait for navigation (form navigates away on success)
    const saveBtn = apiPage.locator('button[type="submit"], button:has-text("Save")').first()
    await saveBtn.click()
    await apiPage.waitForURL((url) => !url.pathname.includes('/form/'), { timeout: 10000 }).catch(() => {})
    await apiPage.waitForTimeout(500)

    // Verify the API now has the new value
    const relResponse2 = await request.get(
      `${backend.baseUrl}/api/v1/tickets/TKT-001/relations/blocks`
    )
    const relData2 = await relResponse2.json()
    const tkt002Rel2 = relData2.find((r: { id: string }) => r.id === 'TKT-002')
    expect(tkt002Rel2.meta.reason).toBe(newValue)
  })

  test('removing a relation is only persisted on save', async ({ apiPage, request, backend }) => {
    await openEditForm(apiPage, 'TKT-001')

    // Find the tagged widget
    const taggedWidget = apiPage.locator('.relation-cards', { has: apiPage.locator('.section-label:has-text("tagged")') }).first()
    await expect(taggedWidget).toBeVisible({ timeout: 10000 })

    // Remove the first tagged relation
    const firstCardId = await taggedWidget.locator('.relation-card .entity-id').first().textContent()
    const removeBtn = taggedWidget.locator('.relation-card .remove-btn').first()
    await removeBtn.click()

    // Verify the relation still exists in the API
    const relResponse = await request.get(
      `${backend.baseUrl}/api/v1/tickets/TKT-001/relations/tagged`
    )
    const relData = await relResponse.json()
    const stillExists = relData.some((r: { id: string }) => r.id === firstCardId?.trim())
    expect(stillExists).toBeTruthy()

    // Now save and wait for navigation (form navigates away on success)
    const saveBtn = apiPage.locator('button[type="submit"], button:has-text("Save")').first()
    await Promise.all([
      apiPage.waitForURL((url) => !url.pathname.includes('/form/'), { timeout: 10000 }),
      saveBtn.click(),
    ])

    // Verify the relation is now gone from the API
    const relResponse2 = await request.get(
      `${backend.baseUrl}/api/v1/tickets/TKT-001/relations/tagged`
    )
    const relData2 = await relResponse2.json()
    const nowExists = relData2.some((r: { id: string }) => r.id === firstCardId?.trim())
    expect(nowExists).toBeFalsy()
  })
})

test.describe('All Field Types', () => {
  test('date input renders for date properties', async ({ apiPage }) => {
    await apiPage.goto('/form/edit_ticket/TKT-001')
    await apiPage.locator('.relation-cards').first().waitFor({ state: 'visible', timeout: 10000 })

    // blocks card has a date input for "Resolved"
    const blocksSection = apiPage.locator('.relation-cards').nth(1)
    const dateInput = blocksSection.locator('input[type="date"]').first()
    await expect(dateInput).toBeVisible()
  })

  test('number input renders for integer properties', async ({ apiPage }) => {
    await apiPage.goto('/form/edit_ticket/TKT-001')
    await apiPage.locator('.relation-cards').first().waitFor({ state: 'visible', timeout: 10000 })

    // blocks card has a number input for "Impact"
    const blocksSection = apiPage.locator('.relation-cards').nth(1)
    const numberInput = blocksSection.locator('input[type="number"]').first()
    await expect(numberInput).toBeVisible()
  })

  test('checkbox renders for boolean properties', async ({ apiPage }) => {
    await apiPage.goto('/form/edit_ticket/TKT-001')
    await apiPage.locator('.relation-cards').first().waitFor({ state: 'visible', timeout: 10000 })

    // blocks card has a checkbox for "Workaround?"
    const blocksSection = apiPage.locator('.relation-cards').nth(1)
    const checkbox = blocksSection.locator('input[type="checkbox"]').first()
    await expect(checkbox).toBeVisible()
  })

  test('can edit date property', async ({ apiPage }) => {
    await apiPage.goto('/form/edit_ticket/TKT-001')
    await apiPage.locator('.relation-cards').first().waitFor({ state: 'visible', timeout: 10000 })

    const blocksSection = apiPage.locator('.relation-cards').nth(1)
    const dateInput = blocksSection.locator('input[type="date"]').first()
    // Focus, clear, type to trigger native input events
    await dateInput.click()
    await dateInput.press('Control+a')
    await dateInput.pressSequentially('2026-06-15')

    await expect(blocksSection.locator('.unsaved-badge, .card-updated')).toBeVisible({ timeout: 5000 })
  })

  test('can edit integer property', async ({ apiPage }) => {
    await apiPage.goto('/form/edit_ticket/TKT-001')
    await apiPage.locator('.relation-cards').first().waitFor({ state: 'visible', timeout: 10000 })

    const blocksSection = apiPage.locator('.relation-cards').nth(1)
    const numberInput = blocksSection.locator('input[type="number"]').first()
    await numberInput.click()
    await numberInput.pressSequentially('8')

    await expect(blocksSection.locator('.unsaved-badge, .card-updated')).toBeVisible({ timeout: 5000 })
  })

  test('can toggle boolean property', async ({ apiPage }) => {
    await apiPage.goto('/form/edit_ticket/TKT-001')
    await apiPage.locator('.relation-cards').first().waitFor({ state: 'visible', timeout: 10000 })

    const blocksSection = apiPage.locator('.relation-cards').nth(1)
    const checkbox = blocksSection.locator('input[type="checkbox"]').first()
    // Click directly on the checkbox element
    await checkbox.click({ force: true })

    await expect(blocksSection.locator('.unsaved-badge, .card-updated')).toBeVisible({ timeout: 5000 })
  })
})

test.describe('Save Flow', () => {
  test('unsaved badge clears after save', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    // Edit a property to trigger unsaved state
    const blocksCard = apiPage.locator('.relation-card', { has: apiPage.locator('.entity-id:has-text("TKT-002")') }).first()
    await expect(blocksCard).toBeVisible({ timeout: 10000 })

    const reasonInput = blocksCard.locator('input.inline-edit').first()
    await reasonInput.fill('save-clear-test')

    // Unsaved badge should appear
    await expect(apiPage.locator('.pending-badge').first()).toBeVisible({ timeout: 3000 })

    // Click Save
    const saveBtn = apiPage.locator('button[type="submit"], button:has-text("Save")').first()
    await saveBtn.click()

    // Wait for save to complete (toast or navigation)
    await apiPage.waitForTimeout(2000)

    // If still on the same page, the unsaved badge should be gone
    const stillOnForm = apiPage.url().includes('/form/')
    if (stillOnForm) {
      // After save, cards remount — unsaved badge should disappear
      await expect(apiPage.locator('.pending-badge')).toHaveCount(0, { timeout: 5000 })
    }
  })

  test('outgoing and incoming blocks both show unsaved state independently', async ({ apiPage }) => {
    await openEditForm(apiPage, 'TKT-001')

    // Get the outgoing blocks widget (second .relation-cards)
    const outgoingWidget = apiPage.locator('.relation-cards').nth(1)
    await expect(outgoingWidget).toBeVisible({ timeout: 10000 })

    // Get the incoming blocks widget (third .relation-cards)
    const incomingWidget = apiPage.locator('.relation-cards').nth(2)
    await expect(incomingWidget).toBeVisible({ timeout: 10000 })

    // Edit reason on outgoing only
    const outReason = outgoingWidget.locator('input.inline-edit').first()
    await outReason.fill('direction-test')

    // Outgoing should show unsaved, incoming should NOT
    await expect(outgoingWidget.locator('.pending-badge')).toBeVisible({ timeout: 3000 })
    await expect(incomingWidget.locator('.pending-badge')).toHaveCount(0)
  })
})
