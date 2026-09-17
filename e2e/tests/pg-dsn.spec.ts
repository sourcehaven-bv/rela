import { test, expect } from "@playwright/test";
import { POSTGRES_E2E_ENABLED, pgDsnForSchema } from "./fixtures";

/**
 * The postgres e2e run pins each worker to its own schema through the DSN it
 * hands rela-server. Getting that DSN's ENCODING wrong does not fail one spec:
 * the server never starts, so the whole job dies during startup with an error
 * naming a configuration parameter rather than the harness that built it.
 *
 * That is what happened. The DSN used libpq's `options=-c search_path=...` set
 * through URLSearchParams, which writes a space as "+". A URI query reads "+"
 * as a literal plus, so the server saw the parameter name "+search_path" and
 * refused every connection with SQLSTATE 42704. It stayed hidden because pgx
 * decoded "+" back to a space until v5.11.0, which stopped doing so to match
 * libpq.
 *
 * These assertions need no browser and no database, but they do need the admin
 * DSN to build from, so they carry the same gate as every other postgres spec.
 * Keeping them here rather than inside a browser spec means a wrong encoding
 * fails in milliseconds with a readable message instead of as 300 timed-out
 * specs.
 */
test.describe("postgres e2e DSN", () => {
  test.skip(
    !POSTGRES_E2E_ENABLED,
    "requires a postgres backend (set RELA_E2E_DATABASE_URL)",
  );

  test('pins the schema without encoding a space as "+"', () => {
    const dsn = pgDsnForSchema("relae2e_1_2");

    // The literal the server chokes on. Asserting its absence states the bug
    // directly, so a regression names itself in the failure output.
    expect(dsn).not.toContain("+search_path");
    expect(dsn).not.toContain("+");

    expect(new URL(dsn).searchParams.get("search_path")).toBe(
      "relae2e_1_2,public",
    );
  });

  test("keeps the rest of the admin DSN intact", () => {
    // The DSN is rewritten, not rebuilt, so anything dropped here becomes a
    // connection failure rather than a visible error in this harness.
    const admin = new URL(process.env.RELA_E2E_DATABASE_URL ?? "");
    const pinned = new URL(pgDsnForSchema("relae2e_3_4"));

    expect(pinned.protocol).toBe(admin.protocol);
    expect(pinned.host).toBe(admin.host);
    expect(pinned.pathname).toBe(admin.pathname);
    expect(pinned.username).toBe(admin.username);
  });
});
