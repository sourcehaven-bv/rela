import { type Locator, expect } from "@playwright/test";
import { BasePage } from "./base.page";

/** The Configure space (TKT-F5NGMG): editing the app's setup in the app. */
export class ConfigurePage extends BasePage {
  /** Open the space switcher, whose button names the current space or app. */
  async openSwitcher() {
    await this.page
      .locator("#main-sidebar")
      .getByRole("button")
      .first()
      .click();
  }

  get configureMenuItem(): Locator {
    return this.page.getByRole("menuitem", { name: "Configure" });
  }

  async goto(serverUrl: string, path = "") {
    await this.page.goto(`${serverUrl}/configure${path}`);
  }

  get heading(): Locator {
    return this.page.getByRole("heading", { level: 1 });
  }

  get entityTypes(): Locator {
    return this.page.getByTestId("config-entity-types");
  }

  /** A field of the entity type settings, by its label. */
  entitySetting(label: string): Locator {
    return this.page
      .getByTestId("config-entity-settings")
      .getByRole("textbox", { name: label, exact: true });
  }

  /** Renames a property through its drawer, opened from the entity type screen. */
  async renameProperty(
    serverUrl: string,
    entityType: string,
    from: string,
    to: string,
  ) {
    await this.page.goto(
      `${serverUrl}/configure/entity-types/${entityType}?property=${from}`,
    );
    const drawer = this.page.getByTestId("config-property-drawer");
    await drawer.getByRole("textbox", { name: "Name", exact: true }).fill(to);
    await this.page.getByTestId("config-apply-property").click();
  }

  /** The header field of one board column, named by the value it shows. */
  boardColumnHeader(value: string): Locator {
    return this.page
      .getByTestId("config-board-columns")
      .getByRole("textbox", { name: `Header of ${value}`, exact: true });
  }

  /** Adds a key to the end of the sort order of the list or board on screen. */
  async addSortKey(property: string) {
    const sort = this.page.getByTestId("config-sort");
    await sort
      .getByRole("combobox", { name: /^(Then )?sort by$/i })
      .selectOption(property);
    await sort.getByRole("button", { name: "Add sort key" }).click();
  }

  async setSortDirection(property: string, direction: "asc" | "desc") {
    await this.page
      .getByTestId("config-sort")
      .getByRole("combobox", { name: `Order of ${property}` })
      .selectOption(direction);
  }

  async chooseQueryScope(scope: string) {
    await this.page
      .getByRole("combobox", { name: "Query scope", exact: true })
      .selectOption(scope);
  }

  /** Adds a filter every record must meet: `property` is `value`. */
  async addFixedFilter(property: string, value: string) {
    const filters = this.page.getByTestId("config-fixed-filters");
    await filters
      .getByRole("combobox", { name: "Add a condition", exact: true })
      .selectOption(property);
    await filters.getByRole("button", { name: "Add condition" }).click();
    await filters
      .getByRole("textbox", { name: `Value for ${property}` })
      .fill(value);
  }

  async addNavGroup(label: string) {
    await this.page
      .getByRole("textbox", { name: "New group", exact: true })
      .fill(label);
    await this.page.getByRole("button", { name: "Add group" }).click();
  }

  /** Adds a sidebar entry; `target` is the option value, such as `list:bugs`. */
  async addNavEntry(target: string, group: string) {
    await this.page
      .getByRole("combobox", { name: "Opens", exact: true })
      .selectOption(target);
    await this.page
      .getByRole("combobox", { name: "In group", exact: true })
      .selectOption({ label: group });
    await this.page.getByRole("button", { name: "Add entry" }).click();
  }

  async openNavEntrySettings(title: string) {
    await this.page
      .getByRole("button", { name: `Settings of ${title}`, exact: true })
      .click();
  }

  async chooseNavRecords(scope: string) {
    await this.page
      .getByTestId("config-nav-details")
      .getByRole("combobox", { name: "Which records", exact: true })
      .selectOption(scope);
  }

  get draftCount(): Locator {
    return this.page.getByTestId("config-draft-count");
  }

  get preview(): Locator {
    return this.page.getByTestId("config-preview");
  }

  get options(): Locator {
    return this.page.getByTestId("config-options");
  }

  async addOption(value: string) {
    await this.page.getByLabel("New option", { exact: true }).fill(value);
    await this.page.getByRole("button", { name: "Add option" }).click();
  }

  async openReview() {
    await this.page.getByTestId("config-review").click();
  }

  get reviewDrawer(): Locator {
    return this.page.getByTestId("config-review-drawer");
  }

  get saveButton(): Locator {
    return this.page.getByTestId("config-save");
  }

  async save() {
    await expect(this.saveButton).toBeEnabled();
    await this.saveButton.click();
  }

  async discardAll() {
    await this.page
      .getByRole("button", { name: "Discard all", exact: true })
      .click();
    await this.page
      .getByRole("button", { name: "Discard", exact: true })
      .click();
  }

  async reload() {
    await this.page.reload();
  }
}
