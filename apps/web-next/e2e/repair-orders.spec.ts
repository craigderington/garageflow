import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

async function makeRO(api: import("@playwright/test").APIRequestContext, description: string) {
  const c = await (await api.post("/customers", { data: { name: `RO ${uniq()}` } })).json();
  const v = await (
    await api.post("/vehicles", { data: { customer_id: c.id, make: "Jeep", model: "Wrangler", year: 2021, vin: uniq("VIN") } })
  ).json();
  const ro = await (
    await api.post("/repair-orders", { data: { customer_id: c.id, vehicle_id: v.id, description, mileage: 55000 } })
  ).json();
  return { c, v, ro };
}

test.describe("repair orders", () => {
  test("create and edit an estimate without leaving the repair order", async ({ app, api }) => {
    const { ro } = await makeRO(api, `Inline estimate ${uniq()}`);
    await gotoReady(app, `/repair-orders/${ro.id}`);

    await app.getByPlaceholder("Description").fill("Front brake pads");
    await app.getByLabel("Quantity").fill("2");
    await app.getByLabel("Unit price").fill("75");
    await app.getByRole("button", { name: /create estimate/i }).click();
    await expect(app.getByText("Front brake pads")).toBeVisible();
    await expect(app.getByTestId("estimate-total")).toHaveText("$150.00");

    await app.getByRole("button", { name: /^edit$/i }).click();
    await app.getByPlaceholder("Description").fill("Premium front brake pads");
    await app.getByRole("button", { name: /save estimate/i }).click();
    await expect(app.getByText("Premium front brake pads")).toBeVisible();
  });

  test("add completed labor from the repair orders page", async ({ app, api }) => {
    const { ro } = await makeRO(api, `Inline labor ${uniq()}`);
    await gotoReady(app, "/repair-orders");
    const laborCard = app.locator("div.gf-card").filter({ hasText: "Add Completed Labor" });
    await laborCard.getByRole("combobox").nth(0).selectOption(ro.id);
    await laborCard.getByRole("spinbutton").fill("30");
    await laborCard.getByPlaceholder(/Diagnostics/i).fill("Road test");
    await laborCard.getByRole("button", { name: /add labor/i }).click();
    await expect(laborCard.getByRole("status")).toContainText("Labor added");
  });

  test("detail shows description, status, customer + vehicle links, empty sections", async ({ app, api }) => {
    const desc = `Coolant flush ${uniq()}`;
    const { c, v } = await makeRO(api, desc);

    await gotoReady(app, "/repair-orders");
    await app.getByRole("link", { name: desc }).click();

    await expect(app.getByRole("heading", { name: desc })).toBeVisible();
    await expect(app.getByTestId("ro-status")).toContainText("created");
    await expect(app.getByText("No estimate yet")).toBeVisible();
    await expect(app.getByText("No labor logged yet")).toBeVisible();

    await app.getByRole("link", { name: /Jeep Wrangler/ }).click();
    await expect(app).toHaveURL(new RegExp(`/vehicles/${v.id}`));
    await expect(app.getByText(c.name)).toBeVisible();
  });

  test("status transition persists across reload", async ({ app, api }) => {
    const { ro } = await makeRO(api, `Status ${uniq()}`);

    await gotoReady(app, `/repair-orders/${ro.id}`);
    await expect(app.getByTestId("ro-status")).toContainText("created");

    await app.getByLabel("Change status").selectOption("in_progress");
    await expect(app.getByTestId("ro-status")).toContainText("in progress");

    // confirm it actually persisted server-side
    await gotoReady(app, `/repair-orders/${ro.id}`);
    await expect(app.getByTestId("ro-status")).toContainText("in progress");
    const got = await (await api.get(`/repair-orders/${ro.id}`)).json();
    expect(got.status).toBe("in_progress");
  });

  test("detail renders estimate line items and total", async ({ app, api }) => {
    const { ro } = await makeRO(api, `Estimate RO ${uniq()}`);
    await api.post("/estimates", {
      data: {
        repair_order_id: ro.id,
        items: [
          { type: "part", description: "Rotors", quantity: 2, unit_price: 80 },
          { type: "labor", description: "Install", quantity: 1, unit_price: 120 },
        ],
      },
    });

    await gotoReady(app, `/repair-orders/${ro.id}`);
    await expect(app.getByText("Rotors")).toBeVisible();
    await expect(app.getByText("Install")).toBeVisible();
    await expect(app.getByTestId("estimate-total")).toHaveText("$280.00");
  });

  test("detail renders labor logs", async ({ app, api }) => {
    const { ro } = await makeRO(api, `Labor RO ${uniq()}`);
    await api.post("/labor/clock-in", { data: { repair_order_id: ro.id, description: "Teardown" } });

    await gotoReady(app, `/repair-orders/${ro.id}`);
    await expect(app.getByText("Teardown")).toBeVisible();
    await expect(app.getByText("clocked in")).toBeVisible();
  });

  test("unknown repair order id shows a not-found state", async ({ app }) => {
    await gotoReady(app, "/repair-orders/00000000-0000-0000-0000-0000000000ff");
    await expect(app.getByText(/repair order not found/i)).toBeVisible();
  });
});
