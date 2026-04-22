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
 * that one check honest. (review-response RR-K6DJL)
 */

test.describe('Origin allowlist (security canary)', () => {
  test('GET with no Origin header is rejected with 403', async ({ serverUrl }) => {
    const req = await playwrightRequest.newContext();
    try {
      const resp = await req.fetch(`${serverUrl}/api/v1/features`, { method: 'GET' });
      expect(resp.status()).toBe(403);
    } finally {
      await req.dispose();
    }
  });

  test('GET with a mismatched Origin header is rejected with 403', async ({ serverUrl }) => {
    const req = await playwrightRequest.newContext({
      extraHTTPHeaders: { Origin: 'http://evil.example' },
    });
    try {
      const resp = await req.fetch(`${serverUrl}/api/v1/features`, { method: 'GET' });
      expect(resp.status()).toBe(403);
    } finally {
      await req.dispose();
    }
  });

  test('GET with the allowlisted Origin succeeds', async ({ serverUrl }) => {
    const req = await playwrightRequest.newContext({
      extraHTTPHeaders: { Origin: serverUrl },
    });
    try {
      const resp = await req.fetch(`${serverUrl}/api/v1/features`, { method: 'GET' });
      expect(resp.ok()).toBeTruthy();
    } finally {
      await req.dispose();
    }
  });
});
