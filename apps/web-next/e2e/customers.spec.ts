import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

test.describe("customers", () => {
  test("create a customer via the form and see it in the list", async ({ app }) => {
    const name = `Cust ${uniq()}`;
    const phone = "555-0142";
    const email = `${uniq("c")}@example.com`;

    await gotoReady(app, "/customers");
    await app.getByRole("button", { name: /add customer/i }).click();
    await expect(app.getByPlaceholder("Name")).toBeVisible();
    await app.getByPlaceholder("Name").fill(name);
    await app.getByPlaceholder("Phone").fill(phone);
    await app.getByPlaceholder("Email").fill(email);
    await app.getByPlaceholder("Notes").fill("VIP — handle with care");
    await app.getByRole("button", { name: /^create$/i }).click();

    const row = app.getByRole("row").filter({ hasText: name });
    await expect(row).toBeVisible();
    await expect(row).toContainText(phone);
    await expect(row).toContainText(email);
  });

  test("name is required (native validation blocks empty submit)", async ({ app }) => {
    await gotoReady(app, "/customers");
    await app.getByRole("button", { name: /add customer/i }).click();
    await expect(app.getByPlaceholder("Name")).toBeVisible();
    await app.getByRole("button", { name: /^create$/i }).click();

    const nameInput = app.getByPlaceholder("Name");
    // form did not submit: input still present and reported invalid by the browser
    const valid = await nameInput.evaluate((el: HTMLInputElement) => el.checkValidity());
    expect(valid).toBe(false);
  });

  test("customer detail shows contact info and empty relation sections", async ({ app, api }) => {
    const name = `Detail ${uniq()}`;
    const created = await api.post("/customers", {
      data: { name, phone: "555-0199", email: "detail@example.com", notes: "Some notes here" },
    });
    expect(created.ok()).toBeTruthy();
    const customer = await created.json();

    await app.goto(`/customers/${customer.id}`);
    await expect(app.getByTestId("customer-name")).toHaveText(name);
    await expect(app.getByText("555-0199")).toBeVisible();
    await expect(app.getByText("detail@example.com")).toBeVisible();
    await expect(app.getByText("Some notes here")).toBeVisible();
    await expect(app.getByText("No vehicles on file")).toBeVisible();
    await expect(app.getByText("No repair orders yet")).toBeVisible();
  });

  test("customer detail lists related vehicles and repair orders with working links", async ({ app, api }) => {
    const name = `Linked ${uniq()}`;
    const c = await (await api.post("/customers", { data: { name } })).json();
    const v = await (
      await api.post("/vehicles", {
        data: { customer_id: c.id, make: "Toyota", model: "Tacoma", year: 2021, license_plate: "GF-1234", vin: uniq("VIN") },
      })
    ).json();
    const ro = await (
      await api.post("/repair-orders", {
        data: { customer_id: c.id, vehicle_id: v.id, description: "Brake job", mileage: 42000 },
      })
    ).json();
    expect(ro.id).toBeTruthy();

    await app.goto(`/customers/${c.id}`);
    await expect(app.getByText("2021 Toyota Tacoma")).toBeVisible();
    await expect(app.getByText("GF-1234")).toBeVisible();
    await expect(app.getByText("Brake job")).toBeVisible();

    // vehicle link navigates to the vehicle detail page
    await app.getByRole("link", { name: /Toyota Tacoma/ }).click();
    await expect(app).toHaveURL(new RegExp(`/vehicles/${v.id}`));
  });

  test("edit a customer updates the detail", async ({ app, api }) => {
    const c = await (await api.post("/customers", { data: { name: `Edit ${uniq()}`, phone: "111" } })).json();
    const newName = `Edited ${uniq()}`;

    await gotoReady(app, `/customers/${c.id}`);
    await app.getByRole("button", { name: /^edit$/i }).click();
    await app.getByPlaceholder("Name").fill(newName);
    await app.getByPlaceholder("Phone").fill("555-7777");
    await app.getByRole("button", { name: /save changes/i }).click();

    await expect(app.getByTestId("customer-name")).toHaveText(newName);
    await expect(app.getByText("555-7777")).toBeVisible();
    const fromApi = await (await api.get(`/customers/${c.id}`)).json();
    expect(fromApi.name).toBe(newName);
  });

  test("delete a customer removes it and returns to the list", async ({ app, api }) => {
    const c = await (await api.post("/customers", { data: { name: `Del ${uniq()}` } })).json();

    await gotoReady(app, `/customers/${c.id}`);
    app.on("dialog", (d) => d.accept());
    await app.getByRole("button", { name: /^delete$/i }).click();

    await expect(app).toHaveURL(/\/customers$/);
    const res = await api.get(`/customers/${c.id}`);
    expect(res.status()).toBe(404);
  });

  test("unknown customer id shows a not-found state", async ({ app }) => {
    await app.goto("/customers/00000000-0000-0000-0000-0000000000ff");
    await expect(app.getByText(/customer not found/i)).toBeVisible();
  });

  test("clicking a customer name in the list opens the detail page", async ({ app, api }) => {
    const name = `Clickable ${uniq()}`;
    await api.post("/customers", { data: { name } });

    await app.goto("/customers");
    await app.getByRole("link", { name }).click();
    await expect(app.getByTestId("customer-name")).toHaveText(name);
  });
});
