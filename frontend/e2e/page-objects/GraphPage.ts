import { Page, Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Page object for the Graph Explorer view.
 * Handles graph visualization interactions.
 */
export class GraphPage extends BasePage {
  constructor(page: Page) {
    super(page)
  }

  // Selectors

  /** The graph container */
  get graphContainer(): Locator {
    return this.page.locator('.graph-view, .graph-container, svg, canvas, main').first()
  }

  /** Graph nodes */
  get nodes(): Locator {
    return this.page.locator('.node, circle, [data-node]')
  }

  /** Graph edges/links */
  get edges(): Locator {
    return this.page.locator('.edge, line, path.link, [data-edge]')
  }

  /** Node labels */
  get nodeLabels(): Locator {
    return this.page.locator('.node-label, text, [data-label]')
  }

  /** Zoom controls */
  get zoomControls(): Locator {
    return this.page.locator('.zoom-controls, [data-zoom]')
  }

  /** Zoom in button */
  get zoomInButton(): Locator {
    return this.page.locator('button:has-text("+"), .zoom-in, [data-zoom="in"]').first()
  }

  /** Zoom out button */
  get zoomOutButton(): Locator {
    return this.page.locator('button:has-text("-"), .zoom-out, [data-zoom="out"]').first()
  }

  /** Reset/fit view button */
  get resetViewButton(): Locator {
    return this.page.locator('button:has-text("Reset"), .reset-view, [data-zoom="reset"]').first()
  }

  /** Filter panel */
  get filterPanel(): Locator {
    return this.page.locator('.filter-panel, .graph-filters, aside')
  }

  /** Search/focus input */
  get searchInput(): Locator {
    return this.page.locator('input[type="search"], input[placeholder*="search" i], .graph-search').first()
  }

  /** Node info panel/tooltip */
  get nodeInfo(): Locator {
    return this.page.locator('.node-info, .tooltip, .info-panel, [data-node-info]')
  }

  // Page navigation

  async goto(): Promise<void> {
    await this.page.goto('/graph')
  }

  async waitForLoad(): Promise<void> {
    await this.graphContainer.waitFor({ state: 'visible', timeout: 10000 })
  }

  // Actions

  /**
   * Click on a node by its label text
   */
  async clickNode(label: string): Promise<void> {
    await this.page.locator(`.node:has-text("${label}"), [data-node*="${label}"]`).first().click()
  }

  /**
   * Search/focus on a specific node
   */
  async searchNode(query: string): Promise<void> {
    await this.searchInput.fill(query)
    await this.searchInput.press('Enter')
  }

  /**
   * Zoom in on the graph
   */
  async zoomIn(): Promise<void> {
    await this.zoomInButton.click()
  }

  /**
   * Zoom out on the graph
   */
  async zoomOut(): Promise<void> {
    await this.zoomOutButton.click()
  }

  /**
   * Reset the view to default
   */
  async resetView(): Promise<void> {
    await this.resetViewButton.click()
  }

  /**
   * Toggle a filter option
   */
  async toggleFilter(filterName: string): Promise<void> {
    await this.filterPanel.locator(`label:has-text("${filterName}"), input[name="${filterName}"]`).first().click()
  }

  /**
   * Hover over a node to show info
   */
  async hoverNode(label: string): Promise<void> {
    await this.page.locator(`.node:has-text("${label}"), [data-node*="${label}"]`).first().hover()
  }

  // State queries

  /**
   * Get the number of visible nodes
   */
  async getNodeCount(): Promise<number> {
    return this.nodes.count()
  }

  /**
   * Get the number of visible edges
   */
  async getEdgeCount(): Promise<number> {
    return this.edges.count()
  }

  /**
   * Check if a specific node is visible
   */
  async isNodeVisible(label: string): Promise<boolean> {
    const node = this.page.locator(`.node:has-text("${label}"), [data-node*="${label}"]`).first()
    return node.isVisible()
  }

  /**
   * Check if node info panel is visible
   */
  async isNodeInfoVisible(): Promise<boolean> {
    return this.nodeInfo.isVisible()
  }

  /**
   * Get node info panel content
   */
  async getNodeInfoContent(): Promise<string> {
    return (await this.nodeInfo.textContent()) || ''
  }

  /**
   * Check if zoom controls are available
   */
  async hasZoomControls(): Promise<boolean> {
    return this.zoomControls.isVisible()
  }
}
