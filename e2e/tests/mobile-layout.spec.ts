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
   * Safe-area insets moved from the shell onto the pane, because the shell
   * clips its overflow and padding there would not scroll with the content.
   * Pixel 7 reports no inset, so `env()` resolves to its fallback — which is
   * exactly the arithmetic under test: the fallback must leave the hamburger's
   * 60px of headroom rather than collapsing to zero.
   */
  test("the pane reserves headroom for the hamburger and the notch", async ({
    appPage,
  }) => {
    const mobile = new MobileLayoutPage(appPage);
    await mobile.openList("features");

    const padding = await mobile.panePadding();
    // 60px at <=768px, 56px at <=480px; both leave the hamburger its row.
    expect(parseFloat(padding.top)).toBeGreaterThanOrEqual(56);
    // Room for the home indicator below the last card.
    expect(parseFloat(padding.bottom)).toBeGreaterThanOrEqual(24);
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
