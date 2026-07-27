import { test, expect, gotoReady } from "./helpers/fixtures";

test.describe("marketing site", () => {
  test("anonymous visitor sees the landing hero and CTAs", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toContainText(/Run the bay/i);
    await expect(page.getByRole("link", { name: /start free/i }).first()).toBeVisible();
    await expect(page.getByRole("link", { name: /^sign in$/i })).toBeVisible();
  });

  test("landing shows the core sections", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /one platform for the whole floor/i })).toBeVisible();
    await expect(page.getByRole("heading", { name: /the repair, start to finish/i })).toBeVisible();
    await expect(page.getByRole("heading", { name: /priced per shop/i })).toBeVisible();
    // feature modules render
    await expect(page.getByRole("heading", { name: "Repair Orders" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Customer Portal" })).toBeVisible();
  });

  test("Start free goes to login", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: /start free/i }).first().click();
    await expect(page).toHaveURL(/\/login/);
  });

  test("in-page anchors are present", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("link", { name: "Features" })).toHaveAttribute("href", "#features");
    await expect(page.getByRole("link", { name: "Pricing" })).toHaveAttribute("href", "#pricing");
  });

  test("an authenticated user is redirected from the landing to the dashboard", async ({ app }) => {
    await gotoReady(app, "/");
    await expect(app).toHaveURL(/\/dashboard/);
  });
});
