import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

async function makeRO(api: import("@playwright/test").APIRequestContext, description: string) {
  const c = await (await api.post("/customers", { data: { name: `Sch ${uniq()}` } })).json();
  const v = await (await api.post("/vehicles", { data: { customer_id: c.id, make: "Kia", model: "Soul", year: 2018, vin: uniq("VIN") } })).json();
  return (await api.post("/repair-orders", { data: { customer_id: c.id, vehicle_id: v.id, description, mileage: 7000 } })).json();
}

test.describe("schedule", () => {
  test("create a service bay via the form", async ({ app }) => {
    const bayName = `Bay ${uniq()}`;

    await gotoReady(app, "/schedule");
    await app.getByRole("button", { name: /add bay/i }).click();
    await app.getByPlaceholder("Bay name").fill(bayName);
    await app.getByRole("button", { name: /^add$/i }).click();

    const bayRow = app.getByText(bayName, { exact: true });
    await expect(bayRow).toBeVisible();
    // new bays are active by default
    await expect(app.getByText(bayName, { exact: true }).locator("xpath=ancestor::div[1]")).toContainText(/active/i);
  });

  test("schedule a repair order into a bay and time window", async ({ app, api }) => {
    const desc = `SchRO ${uniq()}`;
    const bayName = `SBay ${uniq()}`;
    await makeRO(api, desc);
    await api.post("/schedule/bays", { data: { name: bayName } });

    await gotoReady(app, "/schedule");
    await app.getByRole("button", { name: /add schedule/i }).click();

    // two selects in the schedule form: bay, then RO
    await app.getByRole("combobox").nth(0).selectOption({ label: bayName });
    await app.getByRole("combobox").nth(1).selectOption({ label: desc });
    await app.locator('input[type="datetime-local"]').nth(0).fill("2026-07-01T09:00");
    await app.locator('input[type="datetime-local"]').nth(1).fill("2026-07-01T11:00");
    await app.getByRole("button", { name: /^create$/i }).click();

    const row = app.getByRole("row").filter({ hasText: bayName });
    await expect(row).toBeVisible();
  });

  test("schedule form requires a bay and repair order", async ({ app }) => {
    await gotoReady(app, "/schedule");
    await app.getByRole("button", { name: /add schedule/i }).click();
    await app.getByRole("button", { name: /^create$/i }).click();
    // first select (bay) is required and empty -> invalid
    const valid = await app.getByRole("combobox").nth(0).evaluate((el: HTMLSelectElement) => el.checkValidity());
    expect(valid).toBe(false);
  });
});
