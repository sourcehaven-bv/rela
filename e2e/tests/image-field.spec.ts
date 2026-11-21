import { test, expect } from './fixtures';
import { EntityPage, FormPage, ListPage } from '../pages';

// TKT-WG1410D: a file property with `display: {image: true}` is an image. The
// form shows an upload with preview, replace and remove; the detail view shows
// a preview; a list column shows a thumbnail. The server accepts only PNG,
// JPEG, GIF and WebP, judged by content.

// A valid 1x1 PNG, so the browser can decode it.
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64',
);

const decoded = (img: import('@playwright/test').Locator) =>
  img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth > 0);

test.describe('Image field', () => {
  test('uploads on create, previews on the form and detail view, thumbnails in the list', async ({
    appPage,
    api,
  }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('brand');
    await form.fillField('title', 'Acme');
    await form.attachBinaryFile('logo', 'acme.png', PNG, 'image/png');
    await expect(form.imagePreview('logo')).toBeVisible();
    expect(await decoded(form.imagePreview('logo'))).toBe(true);

    const created = await form.submitAndExpectCreateWithUploads('brands', 1);
    const entity = await api.getEntity('brands', created.id);
    expect(entity._attachments?.logo?.[0]?.filename).toBe('acme.png');

    const detail = new EntityPage(appPage);
    await detail.navigateToEntity('brand', created.id);
    const preview = appPage.locator('main img.image-preview-img');
    await expect(preview).toBeVisible();
    expect(await decoded(preview)).toBe(true);

    const list = new ListPage(appPage);
    await list.navigateToList('brand_logos');
    const thumb = appPage.locator('.rl-table-row img.image-cell').first();
    await expect(thumb).toBeVisible();
    expect(await decoded(thumb)).toBe(true);
  });

  test('replaces and removes the image on the edit form', async ({ appPage, api }) => {
    const brand = await api.createEntity('brands', { properties: { title: 'Globex' } });
    const form = new FormPage(appPage);
    await form.navigateToEditForm('brand', brand.id);

    await form.attachBinaryFile('logo', 'one.png', PNG, 'image/png');
    await expect(form.imagePreview('logo')).toBeVisible();
    await form.attachBinaryFile('logo', 'two.png', PNG, 'image/png');
    await expect
      .poll(async () => (await api.getEntity('brands', brand.id))._attachments?.logo?.[0]?.filename)
      .toBe('two.png');

    await appPage.locator('#field-logo .image-remove').click();
    // The widget asks for confirmation through the app's own dialog.
    await appPage.getByRole('button', { name: 'Delete', exact: true }).click();
    await expect
      .poll(async () => (await api.getEntity('brands', brand.id))._attachments?.logo ?? [])
      .toHaveLength(0);
  });

  test('refuses a file that is not an image', async ({ appPage, api, serverUrl }) => {
    const brand = await api.createEntity('brands', { properties: { title: 'Initech' } });
    const url = `${serverUrl}/api/v1/brands/${brand.id}/_attachments/logo`;
    const put = (name: string, mimeType: string, buffer: Buffer) =>
      appPage.request.put(url, {
        headers: { Origin: serverUrl },
        multipart: { file: { name, mimeType, buffer } },
      });

    const svg = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>');
    expect((await put('logo.svg', 'image/svg+xml', svg)).status()).toBe(422);
    expect((await put('logo.png', 'image/png', svg)).status()).toBe(422);
    expect((await put('doc.pdf', 'application/pdf', Buffer.from('%PDF-1.4'))).status()).toBe(422);
    expect((await api.getEntity('brands', brand.id))._attachments?.logo ?? []).toHaveLength(0);

    expect((await put('logo.png', 'image/png', PNG)).status()).toBe(200);
    const served = await appPage.request.get(`${url}/logo.png`);
    expect(served.headers()['content-type']).toBe('image/png');
    expect(served.headers()['x-content-type-options']).toBe('nosniff');
  });

  test('the form refuses a non-image pick before uploading', async ({ appPage, api }) => {
    const brand = await api.createEntity('brands', { properties: { title: 'Hooli' } });
    const form = new FormPage(appPage);
    await form.navigateToEditForm('brand', brand.id);
    await form.attachBinaryFile('logo', 'doc.pdf', Buffer.from('%PDF-1.4'), 'application/pdf');
    expect(await form.fileErrorText('logo')).toContain('PNG, JPEG, GIF or WebP');
    expect((await api.getEntity('brands', brand.id))._attachments?.logo ?? []).toHaveLength(0);
  });
});
