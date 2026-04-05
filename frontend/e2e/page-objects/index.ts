/**
 * Page Objects Index
 *
 * Central export for all page objects used in e2e tests.
 * Import page objects from this file to ensure consistency.
 *
 * Usage:
 *   import { SearchPage, EntityDetailPage, createFormPage } from './page-objects'
 */

// Base class
export { BasePage } from './BasePage'

// View-specific page objects
export { SearchPage } from './SearchPage'
export { EntityDetailPage, createEntityDetailPage } from './EntityDetailPage'
export { FormPage, CreateTicketFormPage, EditTicketFormPage, createFormPage } from './FormPage'
export { ListPage, createListPage } from './ListPage'
export { KanbanPage, createKanbanPage } from './KanbanPage'
export { DashboardPage } from './DashboardPage'
export { GraphPage } from './GraphPage'

// Re-export Page type for convenience
export type { Page, Locator } from '@playwright/test'
