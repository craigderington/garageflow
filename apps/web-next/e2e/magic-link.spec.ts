import { test, expect } from "./helpers/fixtures";
import { API_URL, SEED_EMAIL } from "./helpers/api";

// The emailed link is {APP_URL}/auth/verify?code=... Before this route existed
// it 404'd, so every magic-link sign-in in production dead-ended. These walk
// the same path the email does.
test.describe("magic link", () => {
  async function requestCode(request: import("@playwright/test").APIRequestContext) {
    const res = await request.post(`${API_URL}/auth/magic-link`, {
      data: { email: SEED_EMAIL.owner },
    });
    expect(res.ok()).toBeTruthy();
    const { code } = (await res.json()) as { code?: string };
    // Only present with AUTH_DEV_CODES, which the E2E stack sets.
    expect(code, "AUTH_DEV_CODES must be on for the E2E stack").toBeTruthy();
    return code as string;
  }

  test("following the emailed link signs you in", async ({ page, request }) => {
    const code = await requestCode(request);

    await page.goto(`/auth/verify?code=${encodeURIComponent(code)}`);

    await expect(page).toHaveURL(/\/dashboard/);
  });

  test("pasting the code by hand signs you in", async ({ page, request }) => {
    const code = await requestCode(request);

    await page.goto("/auth/verify");
    await page.getByPlaceholder("Paste the code from your email").fill(code);
    await page.getByRole("button", { name: /sign in/i }).click();

    await expect(page).toHaveURL(/\/dashboard/);
  });

  test("a bad code explains itself instead of hanging", async ({ page }) => {
    await page.goto("/auth/verify?code=not-a-real-code");

    await expect(page.getByText(/invalid or has expired/i)).toBeVisible();
    await expect(page).toHaveURL(/\/auth\/verify/);
  });

  // Regression: the page used to interpolate a `code` that production never
  // returns, rendering the literal text "undefined (dev mode)" to real users.
  test("the sent confirmation never shows undefined", async ({ page }) => {
    await page.goto("/login");
    // The page opens in password mode; the toggle and the submit button both
    // say "send magic link", so match the submit button exactly.
    await page.getByRole("button", { name: "Send magic link instead" }).click();
    await page.getByPlaceholder("you@shop.com").fill(SEED_EMAIL.owner);
    await page.getByRole("button", { name: "Send Magic Link", exact: true }).click();

    const confirmation = page.getByText(/magic link sent|check your email/i);
    await expect(confirmation).toBeVisible();
    await expect(confirmation).not.toContainText("undefined");
  });

  // The seeded credentials used to be printed here in plain text.
  test("the login page publishes no credentials", async ({ page }) => {
    await page.goto("/login");

    await expect(page.locator("body")).not.toContainText("password123");
    await expect(page.locator("body")).not.toContainText("owner@garageflow.app");
  });
});
