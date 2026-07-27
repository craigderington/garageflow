import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

async function makeRO(api: import("@playwright/test").APIRequestContext, description: string) {
  const c = await (await api.post("/customers", { data: { name: `Lab ${uniq()}` } })).json();
  const v = await (await api.post("/vehicles", { data: { customer_id: c.id, make: "Ram", model: "1500", year: 2019, vin: uniq("VIN") } })).json();
  return (await api.post("/repair-orders", { data: { customer_id: c.id, vehicle_id: v.id, description, mileage: 8000 } })).json();
}

test.describe("labor", () => {
  test("clock in starts an active session, clock out records minutes", async ({ app, api }) => {
    const desc = `LaborRO ${uniq()}`;
    const task = `Replace alternator ${uniq()}`;
    await makeRO(api, desc);

    await gotoReady(app, "/labor");
    await app.getByRole("combobox").selectOption({ label: desc });
    await app.getByPlaceholder("Task description").fill(task);
    await app.getByRole("button", { name: /clock in/i }).click();

    // active session shows the task and offers clock out
    await expect(app.getByText(task)).toBeVisible();
    const clockOut = app.getByRole("button", { name: /clock out/i });
    await expect(clockOut).toBeVisible();

    await app.getByPlaceholder("e.g. 45").fill("45");
    await clockOut.click();

    const row = app.getByRole("row").filter({ hasText: task });
    await expect(row).toBeVisible();
    await expect(row).toContainText("45m");
  });

  test("clock in is disabled until a repair order is selected", async ({ app }) => {
    await gotoReady(app, "/labor");
    await expect(app.getByRole("button", { name: /clock in/i })).toBeDisabled();
  });
});
