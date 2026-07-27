import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

test.describe("inventory", () => {
  test("create a part via the form and see it in the list", async ({ app }) => {
    const name = `Part ${uniq()}`;
    const sku = uniq("SKU").toUpperCase();

    await gotoReady(app, "/inventory");
    await app.getByRole("button", { name: /add part/i }).click();
    await expect(app.getByPlaceholder("Part name")).toBeVisible();
    await app.getByPlaceholder("Part name").fill(name);
    await app.getByPlaceholder("SKU").fill(sku);
    await app.getByPlaceholder("Stock level").fill("40");
    await app.getByPlaceholder("Unit price").fill("19.99");
    await app.getByRole("button", { name: /^create$/i }).click();

    const row = app.getByRole("row").filter({ hasText: name });
    await expect(row).toBeVisible();
    await expect(row).toContainText(sku);
    await expect(row).toContainText("40");
    await expect(row).toContainText("$19.99");
  });

  test("part name is required", async ({ app }) => {
    await gotoReady(app, "/inventory");
    await app.getByRole("button", { name: /add part/i }).click();
    await expect(app.getByPlaceholder("Part name")).toBeVisible();
    await app.getByRole("button", { name: /^create$/i }).click();
    const valid = await app.getByPlaceholder("Part name").evaluate((el: HTMLInputElement) => el.checkValidity());
    expect(valid).toBe(false);
  });

  test("a part at or below min stock shows the low-stock indicator", async ({ app, api }) => {
    const name = `Low ${uniq()}`;
    // default min_stock is 5; stock 2 -> low
    await api.post("/inventory", { data: { name, sku: uniq("SKU"), stock_level: 2, min_stock: 5, unit_price: 5 } });

    await gotoReady(app, "/inventory");
    const row = app.getByRole("row").filter({ hasText: name });
    await expect(row).toBeVisible();
    await expect(row.getByTestId("low-stock")).toBeVisible();
  });

  test("delete a part removes it from the list", async ({ app, api }) => {
    const name = `Del ${uniq()}`;
    await api.post("/inventory", { data: { name, sku: uniq("SKU"), stock_level: 5, min_stock: 2, unit_price: 1 } });

    await gotoReady(app, "/inventory");
    const row = app.getByRole("row").filter({ hasText: name });
    await expect(row).toBeVisible();
    app.on("dialog", (d) => d.accept());
    await row.getByRole("button", { name: `Delete ${name}` }).click();
    await expect(row).toHaveCount(0);
  });

  test("restocking increases stock level", async ({ app, api }) => {
    const name = `Restock ${uniq()}`;
    const part = await (
      await api.post("/inventory", { data: { name, sku: uniq("SKU"), stock_level: 3, min_stock: 5, unit_price: 8 } })
    ).json();

    await gotoReady(app, "/inventory");
    await app.getByRole("combobox").selectOption({ label: `${name} (3 in stock)` });
    await app.getByPlaceholder("Quantity to add").fill("10");
    await app.getByRole("button", { name: /^restock$/i }).click();

    const row = app.getByRole("row").filter({ hasText: name });
    await expect(row).toContainText("13");

    // verify GET /inventory/{id} reflects the new stock
    const got = await (await api.get(`/inventory/${part.id}`)).json();
    expect(got.stock_level).toBe(13);
  });
});
