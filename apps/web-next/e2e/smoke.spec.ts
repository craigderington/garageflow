import { test, expect } from "./helpers/fixtures";
import { SEED_EMAIL, SEED_PASSWORD } from "./helpers/api";

test.describe("smoke", () => {
  test("unauthenticated visitor is sent to login", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByRole("button", { name: /sign in/i })).toBeVisible();
  });

  test("password login lands on the dashboard", async ({ page }) => {
    await page.goto("/login");
    await page.getByTestId("signin-email").fill(SEED_EMAIL.owner);
    await page.getByPlaceholder("Enter your password").fill(SEED_PASSWORD);
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/dashboard/);
  });

  test("authed app shell renders the nav", async ({ app }) => {
    await app.goto("/dashboard");
    const nav = app.getByRole("navigation").first();
    await expect(nav.getByRole("link", { name: "Repair Orders" })).toBeVisible();
    await expect(nav.getByRole("link", { name: "Customers" })).toBeVisible();
    await expect(nav.getByRole("link", { name: "Inventory", exact: true })).toBeVisible();
  });
});
