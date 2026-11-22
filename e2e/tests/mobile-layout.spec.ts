import { test, expect, SEED } from "./fixtures";
import { withDataEntryBlock } from "./next-action-fixture";
import { MobileLayoutPage, NextActionPage } from "../pages";

/**
 * The mobile app shell (RlAppShell).
 *
 * Runs on a phone viewport, which nothing else in the suite does. It exists
 * because moving to the shell changed the SCROLL ROOT: rela used to scroll the
 * document beside a fixed sidebar, and the shell is `100dvh` with the main
 * pane scrolling internally. Everything that sticks, bleeds to the viewport
 * edge, or reserves a safe-area inset was written against the old model, and
 * none of it is observable from Desktop Chrome or from jsdom.
 *
 * These assert user-visible outcomes — the bar stays at the top, the drawer
 * covers the content, the content clears the notch — rather than the CSS that
 * currently produces them, so a later change to the shell's internals does not
 * have to rewrite the file.
 */
test.describe("Mobile app shell", () => {
  test("the drawer is closed until the hamburger opens it", async ({
    appPage,
  }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    await mobile.expectDrawerOffScreen();

    await mobile.openDrawer();
    await mobile.expectDrawerOnScreen();
    // The scrim is what tells the user the content behind is inert.
    await expect(mobile.scrim).toBeVisible();
  });

  test("Escape closes the drawer", async ({ appPage }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    await mobile.openDrawer();
    await mobile.expectDrawerOnScreen();

    await appPage.keyboard.press("Escape");
    await mobile.expectDrawerOffScreen();
  });

  test("tapping the scrim closes the drawer", async ({ appPage }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    await mobile.openDrawer();
    await mobile.expectDrawerOnScreen();

    // To the RIGHT of the open drawer: the scrim spans the whole viewport, so
    // a click near the left edge lands on the sidebar covering it.
    await mobile.tapScrimBesideDrawer();
    await mobile.expectDrawerOffScreen();
  });

  /*
   * THE SCROLL-ROOT ASSERTION. The shell hides the document's overflow, so a
   * test that scrolls the window measures nothing at all. Scrolling the pane
   * and seeing its scrollTop move is what proves the pane is the container
   * every sticky rule now resolves against.
   */
  test("the main pane scrolls, not the document", async ({ appPage }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    // The fixture list is short enough to fit a phone screen, so give the pane
    // something to scroll. Without this the assertion below is vacuous: a pane
    // with no overflow reports scrollTop 0 whether or not it is the container.
    await mobile.growPaneContent();
    expect(await mobile.paneScrollTop()).toBe(0);

    await mobile.scrollPane(300);
    expect(await mobile.paneScrollTop()).toBeGreaterThan(0);

    // The window stayed put: the shell, not the document, owns the overflow.
    const windowScroll = await appPage.evaluate(() => window.scrollY);
    expect(windowScroll).toBe(0);
  });

  /*
   * The bar has to stay at the top of the VIEWPORT while the pane scrolls
   * under it. This is the assertion that fails if the sticky bars were left
   * resolving against a scroll root that no longer scrolls.
   */
  test("the mobile topbar stays put while the pane scrolls", async ({
    appPage,
  }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    test.skip(
      !(await mobile.hasTopbar()),
      "this list renders no mobile topbar",
    );

    const before = await mobile.topOf(mobile.topbar);
    await mobile.scrollPane(400);
    const after = await mobile.topOf(mobile.topbar);

    // Sub-pixel tolerance only: a bar that came unstuck moves by the full
    // scroll distance, not by half a pixel.
    expect(Math.abs(after - before)).toBeLessThan(2);
  });

  /*
   * The negative-margin bleed in mobile-bars.css is written against the pane's
   * horizontal padding. If the two ever disagree the bars stop reaching the
   * screen edge, which looks like a rendering bug rather than a CSS contract
   * being broken, so the contract is asserted directly.
   */
  test("the page padding contract the bleeding bars depend on", async ({
    appPage,
  }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    /*
     * The bars hardcode a negative margin per breakpoint (-16px at <=768px,
     * -12px at <=480px). Whichever applies, the bleed only reaches the screen
     * edge if the pane's horizontal padding is the same number, so the two are
     * compared against each other rather than against a literal.
     */
    const ppx = await mobile.pagePaddingX();
    expect(["16px", "12px"]).toContain(ppx);

    const padding = await mobile.panePadding();
    expect(padding.left).toBe(ppx);
    expect(padding.right).toBe(ppx);
  });

  /*
   * A list puts its title and the drawer toggle in the shell's header, so
   * App.vue renders no hamburger row. The pane must not keep the 60px it
   * reserves for that row: it showed as an empty band under the header.
   */
  test("a list leaves no empty band under its header", async ({ appPage }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    const padding = await mobile.panePadding();
    expect(parseFloat(padding.top)).toBeLessThanOrEqual(20);
    // Room for the home indicator below the last card.
    expect(parseFloat(padding.bottom)).toBeGreaterThanOrEqual(24);
  });

  /*
   * The list's table starts right under the header band. The pane's top
   * padding and the table's own top padding used to add up to an empty band
   * between the toolbar and the rows (BUG-6WQ1U8S).
   */
  test("a list's table starts right under the header", async ({ appPage }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    const gap = await mobile.headerToTableGap();
    expect(gap).toBeLessThanOrEqual(2);
    // Not pulled up under the header band either.
    expect(gap).toBeGreaterThanOrEqual(-1);
  });

  /*
   * An entity page names itself in the bar beside the menu toggle, as a list
   * and a dashboard do. Its heading is below the bar, so the bar names the
   * entity type.
   */
  test("an entity page has a title in the top bar", async ({ appPage }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.navigateTo(`/entity/feature/${SEED.features.authentication}`);
    await expect(mobile.menuBarTitle()).toHaveText(/feature/i);
  });

  /*
   * The badges of a list line up down the column on a phone. A kept cell
   * used to size to its value, so a shorter or empty value moved the badge
   * after it into another column.
   */
  test("a list keeps its columns in place on a phone", async ({ appPage }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    const rows = await mobile.keptCellLefts();
    expect(rows.length).toBeGreaterThan(1);
    expect(rows[0].length).toBeGreaterThan(0);
    for (const row of rows) expect(row).toEqual(rows[0]);
  });

  /*
   * A view section table stacks each row on a phone, as a list does: the
   * first column heads the row and every other column is a labelled line.
   * It used to scroll sideways, with its last columns off screen
   * (BUG-6WQ1U8S). TASK-001 implements FEAT-001 in the seed, so the task
   * view's "Linked features" table has a row.
   */
  test("a view section table stacks its rows instead of scrolling", async ({ appPage }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openEntityTable("task", SEED.tasks.writeUnitTests, "Linked features");

    expect(await mobile.sectionTableScrolls("Linked features")).toBe(false);
    // Each value carries its own column label.
    const labels = mobile.sectionCellLabels("Linked features");
    // The heading cell shows no label; the status line does.
    await expect(labels.nth(0)).toBeHidden();
    await expect(labels.nth(1)).toBeVisible();
    await expect(labels.nth(1)).toHaveText(/status/i);
  });

  /*
   * A long value without spaces, such as an ID, wraps inside its properties
   * row. It used to run past the screen and scroll the pane sideways. The
   * task view shows the assignee in a display section ("Task") and in an
   * inline-edit section ("Narrow fields"), so both renderings are covered.
   */
  test("a long unbroken property value wraps instead of scrolling", async ({ appPage, api }) => {
    const longId = "org_01J9ZQ7K3M5N8P2R4T6V8X0Y1A3C5E7G9H2K4M6P8R0T2V4X6Z8B";
    await api.updateEntity("tasks", SEED.tasks.writeUnitTests, { assignee: longId });
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openEntitySection("task", SEED.tasks.writeUnitTests, "Task");

    for (const heading of ["Task", "Narrow fields"]) {
      const value = mobile.sectionValue(heading, longId);
      await expect(value).toBeVisible();
      // Wrapped onto more lines, not clipped to one.
      expect(await mobile.lineCount(value)).toBeGreaterThan(1);
    }
    expect(await mobile.sectionValueOverflows("Task")).toBe(false);
    expect(await mobile.sectionValueOverflows("Narrow fields")).toBe(false);
    expect(await mobile.paneScrollsSideways()).toBe(false);
  });

  // The header's tools hold the list's search, sort and filter. A phone
  // needs them as much as a desktop does.
  test("a list keeps its search box on a phone", async ({ appPage }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    await expect(mobile.listSearch).toBeVisible();
    await expect(mobile.listSearch).toBeInViewport();
  });

  // A closed drawer sits just off the left edge. Its shadow fell onto the
  // content as a grey strip.
  test("the closed drawer casts no shadow onto the content", async ({
    appPage,
  }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    await mobile.expectDrawerOffScreen();
    expect(await mobile.drawerShadow()).toBe("none");

    await mobile.openDrawer();
    await mobile.expectDrawerOnScreen();
    expect(await mobile.drawerShadow()).not.toBe("none");
  });

  test("the hamburger clears the notch and stays reachable while scrolling", async ({
    appPage,
  }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    await expect(mobile.hamburger).toBeVisible();
    const before = await mobile.topOf(mobile.hamburger);
    expect(before).toBeGreaterThanOrEqual(0);

    await mobile.scrollPane(400);

    // Still on screen: it is sticky within the pane, so it does not scroll
    // away and strand the user with no way back to the navigation.
    await expect(mobile.hamburger).toBeInViewport();
  });

  /*
   * On a page that renders its header inline, the hamburger floats over the
   * content as it scrolls. It needs its own opaque surface, or the text
   * under it shows through the icon and both become unreadable.
   */
  test("the hamburger keeps its own surface over scrolled content", async ({
    appPage,
  }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.navigateTo(`/entity/feature/${SEED.features.authentication}`);
    await expect(mobile.hamburger).toBeVisible();

    await mobile.growPaneContent();
    await mobile.scrollPane(400);

    await expect(mobile.hamburger).toBeInViewport();
    expect(await mobile.backgroundAlpha(mobile.hamburger)).toBe(1);
  });

  /**
   * A shortcut hint is meaningless where there is no keyboard, so the shell
   * hides every one below the breakpoint.
   *
   * Only a real browser at a real width can see this: the rule is a media
   * query and RlKbd sets `display: inline-flex` in its own scoped style, so
   * the hiding rule has to beat it. happy-dom applies neither stylesheet and
   * would report the hint visible whether or not the rule survived — which is
   * exactly how this could regress silently during the RlKbd migration.
   */
  test("keyboard shortcut hints are hidden on a touch device", async ({
    appPage,
  }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    // The hints may be in the DOM; what matters is that none is displayed.
    for (const hint of await mobile.shortcutHints.all()) {
      await expect(hint).toBeHidden();
    }
  });
});

/**
 * The next-action card is the first thing on a page, right under the
 * hamburger's row. The two must not touch: a tap meant for the menu would
 * land on the card, and the card's border would run into the button. Pages
 * with a sticky topbar are not covered here: there the button sits over the
 * topbar on purpose, and the card comes below the topbar.
 */
const withBanner = withDataEntryBlock(`next_action_bands:
  - id: focus
    prominence: banner
next_actions:
  about-this-feature:
    band: focus
    context: feature
    suggest: "This one: {title}"
`);

withBanner.describe("Mobile app shell with a banner", () => {
  withBanner(
    "the hamburger keeps clear of the next-action card",
    async ({ appPage }) => {
      const mobile = new MobileLayoutPage(appPage);
      const na = new NextActionPage(appPage);
      await mobile.navigateTo(
        `/entity/feature/${SEED.features.authentication}`,
      );
      await na.expectShowing(/^This one: /);
      await expect(mobile.hamburger).toBeVisible();

      const button = await mobile.hamburger.boundingBox();
      const card = await na.card.boundingBox();
      if (!button || !card) throw new Error("hamburger or card has no box");
      // About 8px of air between the button and the card. Boxes are
      // fractional on a phone, so allow half a pixel of rounding.
      expect(card.y - (button.y + button.height)).toBeGreaterThanOrEqual(7.5);
    },
  );
});
