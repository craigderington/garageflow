import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

async function makeRO(api: import("@playwright/test").APIRequestContext, description: string) {
  const c = await (await api.post("/customers", { data: { name: `Est ${uniq()}` } })).json();
  const v = await (await api.post("/vehicles", { data: { customer_id: c.id, make: "GMC", model: "Sierra", year: 2020, vin: uniq("VIN") } })).json();
  return (await api.post("/repair-orders", { data: { customer_id: c.id, vehicle_id: v.id, description, mileage: 10000 } })).json();
}

test.describe("estimates", () => {
  test("build an estimate via the form with line items and total", async ({ app, api }) => {
    const desc = `EstBuild ${uniq()}`;
    await makeRO(api, desc);

    await gotoReady(app, "/estimates");
    await app.getByRole("button", { name: /new estimate/i }).click();
    await expect(app.getByRole("combobox").first()).toBeVisible();
    await app.getByRole("combobox").first().selectOption({ label: desc });

    await app.getByPlaceholder("Description").fill("Timing belt");
    const numbers = app.locator('form input[type="number"]');
    await numbers.nth(0).fill("2"); // quantity
    await numbers.nth(1).fill("150"); // unit price
    await app.getByRole("button", { name: /create estimate/i }).click();

    const row = app.getByRole("row").filter({ hasText: desc });
    await expect(row).toBeVisible();
    await expect(row).toContainText("$300.00");
    await expect(row).toContainText(/draft/i);
  });

  test("send a draft estimate moves it to sent", async ({ app, api }) => {
    const desc = `EstSend ${uniq()}`;
    const ro = await makeRO(api, desc);
    await api.post("/estimates", { data: { repair_order_id: ro.id, items: [{ type: "part", description: "Filter", quantity: 1, unit_price: 25 }] } });

    await gotoReady(app, "/estimates");
    const row = app.getByRole("row").filter({ hasText: desc });
    await expect(row).toContainText(/draft/i);
    await row.getByRole("button", { name: /send/i }).click();
    await expect(row).toContainText(/sent/i);
  });

  test("approve a sent estimate and the RO becomes approved", async ({ app, api }) => {
    const desc = `EstApprove ${uniq()}`;
    const ro = await makeRO(api, desc);
    const est = await (
      await api.post("/estimates", { data: { repair_order_id: ro.id, items: [{ type: "labor", description: "Diag", quantity: 1, unit_price: 90 }] } })
    ).json();
    await api.post(`/estimates/${est.id}/send`);

    await gotoReady(app, "/estimates");
    const row = app.getByRole("row").filter({ hasText: desc });
    await expect(row).toContainText(/sent/i);
    await row.getByRole("button", { name: /approve/i }).click();
    await expect(row).toContainText(/approved/i);

    // approving the estimate flips the repair order to approved server-side
    const updatedRO = await (await api.get(`/repair-orders/${ro.id}`)).json();
    expect(updatedRO.status).toBe("approved");
  });

  test("an RO that already has an estimate is not offered in the build dropdown", async ({ app, api }) => {
    const desc = `EstDup ${uniq()}`;
    const ro = await makeRO(api, desc);
    await api.post("/estimates", { data: { repair_order_id: ro.id, items: [{ type: "fee", description: "Shop fee", quantity: 1, unit_price: 10 }] } });

    await gotoReady(app, "/estimates");
    await app.getByRole("button", { name: /new estimate/i }).click();
    const select = app.getByRole("combobox").first();
    await expect(select).toBeVisible();
    await expect(select.getByRole("option", { name: desc })).toHaveCount(0);
  });
});
