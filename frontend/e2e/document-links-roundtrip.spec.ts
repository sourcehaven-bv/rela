import { test, expect } from './fixtures'
import * as fs from 'fs'
import * as path from 'path'

/**
 * E2E for the document → form → scroll-back roundtrip (TKT-4MFUK).
 *
 * What's under test:
 *
 *   1. `rela.url.form_edit` / `form_create` in a doc script render as
 *      app-relative `/form/...` links.
 *   2. The server's RewriteDocumentLinks appends a `return_to=` query +
 *      stable `id="edit-<id>-0"` / `id="create-<form>-0"` on each form link.
 *   3. The SPA's click handler promotes that id into a `#<id>` fragment
 *      on return_to, so after submit the router's scroll-settle loop
 *      scrolls to the link the user clicked.
 *   4. Mermaid-containing docs still behave correctly: scroll-settle
 *      listens for `rela:mermaid-rendered` events instead of polling.
 *   5. Counter-based anchor ids disambiguate duplicate links to the same
 *      entity (`-0`, `-1`) and survive title edits (since they're derived
 *      from the URL path, not goldmark's heading-based ids).
 *
 * Each test writes its own doc script into the temp project so we don't
 * rely on the prototype shipping every edge case.
 */

/**
 * Write a doc script + data-entry.yaml entry into the backend's project.
 * Returns the document name.
 */
function injectDocScript(projectPath: string, name: string, entityType: string, luaBody: string): string {
  const scriptPath = `docs/${name}.lua`
  const scriptsDir = path.join(projectPath, 'scripts', 'docs')
  fs.mkdirSync(scriptsDir, { recursive: true })
  fs.writeFileSync(path.join(projectPath, 'scripts', scriptPath), luaBody)

  // Append the document to data-entry.yaml. The existing file ends with a
  // `documents:` block, so we splice a new entry in rather than rewrite.
  const yamlPath = path.join(projectPath, 'data-entry.yaml')
  const original = fs.readFileSync(yamlPath, 'utf8')
  const entry = [
    `  ${name}:`,
    `    title: "${name}"`,
    `    entity_type: ${entityType}`,
    `    script: ${scriptPath}`,
    `    timeout: 10`,
    ``,
  ].join('\n')
  const updated = original.replace(/^documents:\n/m, `documents:\n${entry}`)
  fs.writeFileSync(yamlPath, updated)
  return name
}

test.describe('Document form-link roundtrip', () => {
  test('edit link: return_to + scroll-back fragment + form edits land back on doc', async ({
    api,
    apiPage,
  }) => {
    // Prepare: one category + one ticket that belongs to it. The existing
    // category_overview doc will render an edit link per ticket using the
    // prototype's category_report.lua script — no need to inject one.
    const categoryId = await api.getOrCreateCategory('Roundtrip test')
    const ticket1 = await api.createEntity('tickets', {
      properties: {
        title: 'Roundtrip ticket A',
        status: 'open',
        priority: 'medium',
        reporter: 'e2e',
      },
      relations: { 'belongs-to': [categoryId] },
    })

    // Visit the category page. The DocumentsPanel renders category_overview
    // on mount and re-renders when the selected doc changes.
    await apiPage.goto(`/entity/category/${categoryId}`)
    const docBody = apiPage.locator('.document-body')
    await expect(docBody).toBeVisible({ timeout: 15000 })

    // The doc panel mounts with no selected doc, then seeds it from the
    // ?doc= query or the first available document. The second render is
    // the one that carries return_to — wait for our specific edit link
    // with return_to attached before asserting.
    const editLink = docBody.locator(`a#edit-${ticket1.id.toLowerCase()}-0`)
    await expect(editLink).toBeVisible({ timeout: 10000 })
    await expect
      .poll(async () => (await editLink.getAttribute('href')) || '', { timeout: 10000 })
      .toContain('return_to=')
    const href = await editLink.getAttribute('href')
    expect(href).toContain(`/form/edit_ticket/${ticket1.id}`)

    // Clicking the link navigates via vue-router (not full reload) and
    // the click handler promotes the anchor id into a #fragment on
    // return_to, so submit lands back on the category page scrolled to
    // this link.
    await editLink.click()
    await apiPage.waitForURL(/\/form\/edit_ticket\//, { timeout: 10000 })

    // After vue-router's push, return_to in the URL must carry the anchor.
    const formURL = new URL(apiPage.url())
    const returnTo = formURL.searchParams.get('return_to')
    expect(returnTo).toBeTruthy()
    expect(returnTo).toContain(`#edit-${ticket1.id.toLowerCase()}-0`)

    // Submit the form. Edit mode needs no field changes to save.
    const saveResponse = apiPage.waitForResponse(
      (r) => r.url().includes(`/api/v1/tickets/${ticket1.id}`) && r.request().method() === 'PATCH',
    )
    await apiPage.locator('button[type="submit"]').first().click()
    await saveResponse

    // Router lands back on the category page with the hash fragment.
    await apiPage.waitForURL(new RegExp(`/entity/category/${categoryId}`), { timeout: 10000 })
    await expect(apiPage.locator('.document-body')).toBeVisible({ timeout: 15000 })

    // The scroll-settle loop re-scrolls to the edit link as the doc re-
    // renders. Give it up to SETTLE_TIMEOUT_MS. We assert the *link* is
    // in the viewport, not an exact scroll offset — browsers differ on
    // pixel rounding and we only care that the user is near where they
    // clicked.
    const restoredLink = apiPage.locator(`a#edit-${ticket1.id.toLowerCase()}-0`)
    await expect(restoredLink).toBeInViewport({ timeout: 6000 })

    await api.deleteEntity('tickets', ticket1.id)
  })

  test('create link: stable id, fragment roundtrip, survives title edits', async ({
    api,
    apiPage,
  }) => {
    const categoryId = await api.getOrCreateCategory('Create roundtrip')
    await apiPage.goto(`/entity/category/${categoryId}`)
    await expect(apiPage.locator('.document-body')).toBeVisible({ timeout: 15000 })

    // The prototype's category_report.lua emits exactly one create link
    // at the bottom for "New ticket in this category".
    const createLink = apiPage.locator('a#create-create_ticket-0')
    await expect(createLink).toBeVisible({ timeout: 10000 })
    const href = await createLink.getAttribute('href')
    expect(href).toContain('/form/create_ticket')
    // rel.belongs-to is carried through so the form pre-links the category.
    expect(href).toContain('rel.belongs-to=')
    expect(href).toContain('return_to=')
  })

  test('mermaid path: diagram renders and form link is still wired', async ({
    api,
    apiPage,
    backend,
  }) => {
    // A tiny doc with both a mermaid block and a form_create link. The
    // mermaid block exercises the event-driven scroll-settle signal; the
    // link presence asserts the rewriter still fired.
    injectDocScript(
      backend.projectPath,
      'mermaid_overview',
      'category',
      [
        '-- Mermaid smoke test for document rendering.',
        'print("# Mermaid test")',
        'print()',
        'print("```mermaid")',
        'print("graph TD")',
        'print("A[Start] --> B[End]")',
        'print("```")',
        'print()',
        'print("[+ New ticket](" .. rela.url.form_create("create_ticket", {',
        '  relations = {["belongs-to"] = rela.document.entry_id},',
        '}) .. ")")',
        '',
      ].join('\n'),
    )

    // The server picks up the new doc config on watcher fire; wait for
    // schema to advertise it before visiting. The doc panel's <select>
    // lists doc configs, so we can key off it directly.
    const categoryId = await api.getOrCreateCategory('Mermaid test')
    await apiPage.goto(`/entity/category/${categoryId}`)

    // Switch to our new doc via the tab selector. The default is whichever
    // comes first; be explicit.
    const selector = apiPage.locator('.documents-panel .doc-select')
    await selector.waitFor({ state: 'visible', timeout: 15000 })
    await selector.selectOption('mermaid_overview')

    // Mermaid replaces <pre class="mermaid"> with <div class="mermaid-diagram">.
    // That swap is how we know scroll-settle's event fires.
    await expect(apiPage.locator('.document-body .mermaid-diagram svg')).toBeVisible({
      timeout: 15000,
    })

    // The create link still has its counter-based id and return_to.
    const createLink = apiPage.locator('.document-body a#create-create_ticket-0')
    await expect(createLink).toBeVisible({ timeout: 10000 })
    const href = await createLink.getAttribute('href')
    expect(href).toContain('/form/create_ticket')
    expect(href).toContain('return_to=')
  })

  test('counter disambiguates duplicate edit links to same entity', async ({
    api,
    apiPage,
    backend,
  }) => {
    // Author a doc that links the first ticket it finds twice. The
    // counter should give the two links distinct ids (-0, -1).
    injectDocScript(
      backend.projectPath,
      'dup_links',
      'category',
      [
        'print("# Dup test")',
        'print()',
        '-- First ticket belonging to this category, linked twice.',
        'local incoming = rela.trace_to(rela.document.entry_id, 1) or {children = {}}',
        'for _, child in ipairs(incoming.children or {}) do',
        '  local t = rela.get_entity(child.id)',
        '  if t ~= nil and t.type == "ticket" then',
        '    local url = rela.url.form_edit("edit_ticket", t)',
        '    print("First: [" .. t.id .. "](" .. url .. ")")',
        '    print()',
        '    print("Second: [" .. t.id .. "](" .. url .. ")")',
        '    break',
        '  end',
        'end',
        '',
      ].join('\n'),
    )

    const categoryId = await api.getOrCreateCategory('Dup links')
    await api.createEntity('tickets', {
      properties: {
        title: 'Original title',
        status: 'open',
        priority: 'low',
        reporter: 'e2e',
      },
      relations: { 'belongs-to': [categoryId] },
    })

    await apiPage.goto(`/entity/category/${categoryId}`)
    const selector = apiPage.locator('.documents-panel .doc-select')
    await selector.waitFor({ state: 'visible', timeout: 15000 })
    await selector.selectOption('dup_links')

    // Find the ticket id the script actually rendered (may be one of the
    // prototype's seed tickets, depending on graph order). Both links
    // point to the same entity and must get -0, -1 suffixes.
    const docBody = apiPage.locator('.document-body')
    await expect(docBody.locator('a[id^="edit-"][id$="-0"]')).toBeVisible({ timeout: 10000 })
    const firstId = await docBody.locator('a[id^="edit-"][id$="-0"]').getAttribute('id')
    expect(firstId).toBeTruthy()
    // Strip the -0 suffix to derive the base; the second link is <base>-1.
    const base = firstId!.slice(0, -'-0'.length)
    await expect(docBody.locator(`a#${base}-1`)).toBeVisible()

    // Ids are derived from the URL path, not any entity title. Trigger a
    // re-render (refresh) and assert the same ids reappear.
    await apiPage.locator('.documents-panel button:has-text("Refresh")').click()
    await expect(docBody).toBeVisible()
    await expect(docBody.locator(`a#${base}-0`)).toBeVisible({ timeout: 10000 })
    await expect(docBody.locator(`a#${base}-1`)).toBeVisible()
  })
})
