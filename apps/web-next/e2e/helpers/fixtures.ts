import { test as base, type Page, type APIRequestContext } from "@playwright/test";
import { sessionFor, apiAs, SEED_EMAIL, type SeedUser } from "./api";

type Fixtures = {
  /** A page already authenticated as the given seed user (default: owner). */
  app: Page;
  /** Authenticated API context for arranging/asserting data directly. */
  api: APIRequestContext;
  /** Log in as an arbitrary seed user on demand within a test. */
  loginAs: (page: Page, user: SeedUser) => Promise<void>;
};

/** Inject the session cookie into a browser context so the app sees an authed user. */
async function authenticate(page: Page, user: SeedUser) {
  const session = await sessionFor(SEED_EMAIL[user]);
  // Cookies are not port-specific: a host-only `localhost` cookie set by the API
  // on :8081 is sent from the web app on :3000 too (credentials: include).
  await page.context().addCookies([
    {
      name: "session",
      value: session,
      domain: "localhost",
      path: "/",
      httpOnly: true,
      secure: true,
      sameSite: "Lax",
    },
  ]);
}

export const test = base.extend<Fixtures>({
  app: async ({ page }, use) => {
    await authenticate(page, "owner");
    await use(page);
  },
  api: async ({}, use) => {
    const ctx = await apiAs("owner");
    await use(ctx);
    await ctx.dispose();
  },
  loginAs: async ({}, use) => {
    await use(async (page: Page, user: SeedUser) => authenticate(page, user));
  },
});

export { expect } from "@playwright/test";

/**
 * Navigate and wait until the client component has hydrated. These pages fire
 * their data-fetch effects only after hydration, so `networkidle` is a reliable
 * proxy for "React has attached event handlers" — without it, early clicks on
 * JS-driven controls (form toggles, buttons) are silently dropped.
 */
export async function gotoReady(page: Page, path: string) {
  await page.goto(path);
  await page.waitForLoadState("networkidle");
}
