import { test, expect } from './fixtures';
import { request as playwrightRequest } from '@playwright/test';

/**
 * Canary for the backend's Origin allowlist middleware.
 *
 * Every other spec in this suite hits the API through the `api` fixture,
 * which injects a matching `Origin` header so the security middleware lets
 * requests through. That leaves the middleware itself un-exercised — a
 * regression that widens or bypasses the allowlist would pass the rest of
 * the suite silently. These tests deliberately skip the fixture to keep
 * that one check honest. (review-response RR-K6DJL, RR-3AERY)
 *
 * The CSRF concern is specifically about cross-site *writes*. A GET-only
 * canary would miss the primary attack surface, so we exercise GET, POST,
 * PATCH, and DELETE.
 */

async function fetchWith(
  origin: string | null,
  method: string,
  url: string,
  body?: Record<string, unknown>,
) {
  const ctx = await playwrightRequest.newContext(
    origin === null ? {} : { extraHTTPHeaders: { Origin: origin } },
  );
  try {
    const opts: Record<string, unknown> = { method };
    if (body !== undefined) opts.data = body;
    return await ctx.fetch(url, opts);
  } finally {
    await ctx.dispose();
  }
}

test.describe('Origin allowlist (security canary)', () => {
  test('GET with no Origin header is rejected with 403', async ({ serverUrl }) => {
    const resp = await fetchWith(null, 'GET', `${serverUrl}/api/v1/features`);
    expect(resp.status()).toBe(403);
  });

  test('GET with a mismatched Origin header is rejected with 403', async ({ serverUrl }) => {
    const resp = await fetchWith('http://evil.example', 'GET', `${serverUrl}/api/v1/features`);
    expect(resp.status()).toBe(403);
  });

  test('GET with the allowlisted Origin succeeds', async ({ serverUrl }) => {
    const resp = await fetchWith(serverUrl, 'GET', `${serverUrl}/api/v1/features`);
    expect(resp.ok()).toBeTruthy();
  });

  test('POST with no Origin header is rejected with 403', async ({ serverUrl }) => {
    const resp = await fetchWith(null, 'POST', `${serverUrl}/api/v1/features`, {
      properties: { title: 'CSRF POST canary', status: 'draft', priority: 'low' },
    });
    expect(resp.status()).toBe(403);
  });

  test('POST with a mismatched Origin header is rejected with 403', async ({ serverUrl }) => {
    const resp = await fetchWith('http://evil.example', 'POST', `${serverUrl}/api/v1/features`, {
      properties: { title: 'CSRF POST canary', status: 'draft', priority: 'low' },
    });
    expect(resp.status()).toBe(403);
  });

  test('PATCH with no Origin header is rejected with 403', async ({ serverUrl }) => {
    const resp = await fetchWith(null, 'PATCH', `${serverUrl}/api/v1/features/FEAT-001`, {
      properties: { title: 'CSRF PATCH canary' },
    });
    expect(resp.status()).toBe(403);
  });

  test('DELETE with no Origin header is rejected with 403', async ({ serverUrl }) => {
    const resp = await fetchWith(null, 'DELETE', `${serverUrl}/api/v1/features/FEAT-001`);
    expect(resp.status()).toBe(403);
  });
});
