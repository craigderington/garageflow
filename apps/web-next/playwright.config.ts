import { defineConfig, devices } from "@playwright/test";

/**
 * GarageFlow E2E config.
 *
 * Targets a running stack. Defaults to the local dev servers (web :3000, api :8081);
 * override with BASE_URL / API_URL to point at the dockerized stack instead.
 *
 *   BASE_URL=http://localhost:3000 API_URL=http://localhost:8081 npx playwright test
 */
const BASE_URL = process.env.BASE_URL || "http://localhost:3000";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  timeout: 30_000,
  expect: { timeout: 7_500 },
  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    actionTimeout: 10_000,
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
});
