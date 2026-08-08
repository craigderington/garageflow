import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

test.describe("dashboard", () => {
  test("renders the four KPI stat cards", async ({ app }) => {
    await gotoReady(app, "/dashboard");
    for (const label of ["In Service", "Awaiting Service", "Completed Today", "Low Stock Items"]) {
      await expect(app.getByText(label, { exact: true }).first()).toBeVisible();
    }
  });

  test("a newly created repair order shows under Awaiting Service", async ({ app, api }) => {
    const desc = `Dash await ${uniq()}`;
    const c = await (await api.post("/customers", { data: { name: `D ${uniq()}` } })).json();
    await api.post("/repair-orders", { data: { customer_id: c.id, description: desc, mileage: 100 } });

    await gotoReady(app, "/dashboard");
    await expect(app.getByText(desc).first()).toBeVisible();
  });

  test("dashboard repair-order records open the selected order", async ({ app, api }) => {
    const desc = `Dash link ${uniq()}`;
    const customer = await (await api.post("/customers", { data: { name: `Link ${uniq()}` } })).json();
    const ro = await (await api.post("/repair-orders", { data: { customer_id: customer.id, description: desc } })).json();
    await gotoReady(app, "/dashboard");
    await app.getByText(desc).first().click();
    await expect(app).toHaveURL(new RegExp(`/repair-orders/${ro.id}`));
  });

  test("an in-progress repair order shows under Vehicles In Service", async ({ app, api }) => {
    const desc = `Dash inservice ${uniq()}`;
    const c = await (await api.post("/customers", { data: { name: `D ${uniq()}` } })).json();
    const ro = await (await api.post("/repair-orders", { data: { customer_id: c.id, description: desc } })).json();
    await api.patch(`/repair-orders/${ro.id}`, { data: { status: "in_progress" } });

    await gotoReady(app, "/dashboard");
    await expect(app.getByText(desc).first()).toBeVisible();
  });

  test("a low-stock part appears in Low Stock Alerts", async ({ app, api }) => {
    const name = `Dash low ${uniq()}`;
    await api.post("/inventory", { data: { name, sku: uniq("SKU"), stock_level: 1, min_stock: 5, unit_price: 3 } });

    await gotoReady(app, "/dashboard");
    await expect(app.getByText(name).first()).toBeVisible();
  });

  test("View all inventory link navigates to inventory", async ({ app }) => {
    await gotoReady(app, "/dashboard");
    await app.getByRole("link", { name: /view all inventory/i }).click();
    await expect(app).toHaveURL(/\/inventory/);
  });
});
