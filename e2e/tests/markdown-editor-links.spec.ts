import { test, expect } from './fixtures';
import { FormPage } from '../pages';

/**
 * Links in the markdown body editor.
 *
 * The geometry assertion here is the point of this file. Unit tests mount the
 * editor under happy-dom, which has no layout engine and returns all-zero
 * rects, so a panel positioned against the wrong anchor — or written into the
 * wrong coordinate space — passes every one of them. Both defects happened, and
 * both were caught by eye rather than by CI. This closes that gap.
 */
test.describe('Markdown editor links', () => {
  let form: FormPage;

  test.beforeEach(async ({ appPage }) => {
    form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
  });

  /** A body with a word to link, and the word itself. */
  const BODY = 'read the documentation now';
  const WORD = 'documentation';

  async function linkTheWord(url: string) {
    await form.typeMarkdownBody(BODY);
    await form.selectWordInMarkdownBody(WORD);
    await form.openLinkDialog();
    await form.submitLinkDialog(url);
  }

  test('inserts a link over the selection and stores the normalized URL', async () => {
    // A bare host, to prove the write-side gate normalizes rather than storing
    // what was typed.
    await linkTheWord('example.com/guide');

    const anchor = form.editorLinks.first();
    await expect(anchor).toHaveText(WORD);
    await expect(anchor).toHaveAttribute('href', 'https://example.com/guide');
  });

  test('refuses a javascript: URL without touching the document', async () => {
    await form.typeMarkdownBody(BODY);
    await form.selectWordInMarkdownBody(WORD);
    await form.openLinkDialog();
    await form.submitLinkDialog('javascript:alert(1)');

    // The dialog stays open with a message, and no link is created. The message
    // names the rule; it must not echo the input back into the page.
    await expect(form.linkDialog).toBeVisible();
    await expect(form.linkDialogError).not.toBeEmpty();
    await expect(form.linkDialogError).not.toContainText('javascript:');
    await expect(form.editorLinks).toHaveCount(0);
  });

  test('the link panel sits under the link it describes', async () => {
    // THE regression this file exists for: the panel anchored to the SELECTION
    // rather than to the link, so it appeared wherever the caret was.
    //
    // Reaching the panel by CLICKING the link would not test this — a click
    // puts the caret inside the link, so the two anchors nearly coincide and
    // any tolerance loose enough to be stable also hides the bug (verified: the
    // click version passed against the original defect). The caret is put in a
    // DIFFERENT paragraph first, so a selection-anchored panel lands a line or
    // more away and the assertion has something to catch.
    await form.typeMarkdownBody(`${BODY}\nsecond paragraph here`);
    await form.selectWordInMarkdownBody(WORD);
    await form.openLinkDialog();
    await form.submitLinkDialog('https://example.com/guide');

    // Caret into the second paragraph, then back into the link via the
    // keyboard, so the panel is driven by caret position rather than a click.
    await form.placeCaretInMarkdownWord('second');
    await form.placeCaretInMarkdownWord(WORD);

    await expect(form.linkPanel).toBeVisible();
    await expect(form.linkPanel).toContainText('https://example.com/guide');

    const { link, panel } = await form.linkAndPanelBoxes();

    // Directly below the link's own box. The offset is 6px by construction, so
    // a 24px bound is generous for rendering variance yet far tighter than the
    // line height a mis-anchored panel would be off by.
    expect(panel.y).toBeGreaterThanOrEqual(link.y);
    expect(panel.y - (link.y + link.height)).toBeLessThan(24);

    // Left-aligned with the link (placement is bottom-start), not with some
    // other point on the line.
    expect(Math.abs(panel.x - link.x)).toBeLessThan(24);
  });

  test('removes a link from the toolbar while keeping the text', async () => {
    await linkTheWord('https://example.com/guide');
    await form.editorLinks.first().click();

    // Reachable without the floating panel, which is what keeps unlink
    // available to a keyboard user.
    await form.markdownToolbarButton('Remove link').click();

    await expect(form.editorLinks).toHaveCount(0);
    await expect(form.proseMirror).toContainText(WORD);
  });

  test('offers divider, undo and redo, with history disabled at the stack ends', async () => {
    const undo = form.markdownToolbarButton('Undo');
    const redo = form.markdownToolbarButton('Redo');

    // aria-disabled, never the native attribute: these flip on every
    // transaction, and a natively disabled control drops focus to <body>
    // mid-interaction.
    await expect(undo).toHaveAttribute('aria-disabled', 'true');
    await expect(redo).toHaveAttribute('aria-disabled', 'true');
    await expect(undo).not.toHaveAttribute('disabled', /.*/);

    await form.typeMarkdownBody('before');
    await form.markdownToolbarButton('Divider').click();

    await expect(form.editorDividers).toHaveCount(1);
    await expect(undo).toHaveAttribute('aria-disabled', 'false');
  });
});
