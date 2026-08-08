import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

async function makeCustomer(api: import("@playwright/test").APIRequestContext, name: string) {
  return (await api.post("/customers", { data: { name } })).json();
}

test.describe("vehicles", () => {
  test("archive and restore a vehicle from the list", async ({ app, api }) => {
    const customer = await (await api.post("/customers", { data: { name: `Archive ${uniq()}` } })).json();
    const make = `ArchiveMake${uniq()}`;
    await api.post("/vehicles", { data: { customer_id: customer.id, make, model: "Retired", year: 2015 } });
    await gotoReady(app, "/vehicles");
    await app.getByLabel(`Archive ${make} Retired`).click();
    await expect(app.getByText(make)).toHaveCount(0);
    await app.getByText("Show archived vehicles").click();
    await expect(app.getByText(make)).toBeVisible();
    await app.getByLabel(`Restore ${make} Retired`).click();
  });

  test("create a vehicle via the form and see it in the list", async ({ app, api }) => {
    const custName = `Owner ${uniq()}`;
    await makeCustomer(api, custName);
    const plate = uniq("PL").toUpperCase().slice(0, 8);

    await gotoReady(app, "/vehicles");
    await app.getByRole("button", { name: /add vehicle/i }).click();
    await expect(app.getByRole("combobox")).toBeVisible();
    await app.getByRole("combobox").selectOption({ label: custName });
    await app.getByPlaceholder("VIN").fill(uniq("VIN"));
    await app.getByPlaceholder("License Plate").fill(plate);
    await app.getByPlaceholder("Year").fill("2022");
    await app.getByPlaceholder("Make").fill("Subaru");
    await app.getByPlaceholder("Model").fill("Outback");
    await app.getByPlaceholder("Color").fill("Green");
    await app.getByRole("button", { name: /^create$/i }).click();

    const row = app.getByRole("row").filter({ hasText: plate });
    await expect(row).toBeVisible();
    await expect(row).toContainText("Subaru");
    await expect(row).toContainText(custName);
  });

  test("customer is required to create a vehicle", async ({ app }) => {
    await gotoReady(app, "/vehicles");
    await app.getByRole("button", { name: /add vehicle/i }).click();
    await expect(app.getByRole("combobox")).toBeVisible();
    await app.getByRole("button", { name: /^create$/i }).click();
    const valid = await app.getByRole("combobox").evaluate((el: HTMLSelectElement) => el.checkValidity());
    expect(valid).toBe(false);
  });

  test("vehicle detail shows specs, owner link, and empty history", async ({ app, api }) => {
    const custName = `VOwner ${uniq()}`;
    const c = await makeCustomer(api, custName);
    const v = await (
      await api.post("/vehicles", {
        data: { customer_id: c.id, make: "Honda", model: "Civic", year: 2019, color: "Blue", license_plate: "HND-9", vin: uniq("VIN") },
      })
    ).json();

    await gotoReady(app, `/vehicles/${v.id}`);
    await expect(app.getByTestId("vehicle-title")).toHaveText("2019 Honda Civic");
    await expect(app.getByText("Blue")).toBeVisible();
    await expect(app.getByText("HND-9")).toBeVisible();
    await expect(app.getByText("No repair orders for this vehicle")).toBeVisible();

    // owner link navigates to the customer
    await app.getByRole("link", { name: custName }).click();
    await expect(app).toHaveURL(new RegExp(`/customers/${c.id}`));
  });

  test("vehicle detail shows service history when ROs exist", async ({ app, api }) => {
    const c = await makeCustomer(api, `SvcOwner ${uniq()}`);
    const v = await (
      await api.post("/vehicles", { data: { customer_id: c.id, make: "Ford", model: "F-150", year: 2020, vin: uniq("VIN") } })
    ).json();
    await api.post("/repair-orders", { data: { customer_id: c.id, vehicle_id: v.id, description: "Oil change", mileage: 30000 } });

    await gotoReady(app, `/vehicles/${v.id}`);
    await expect(app.getByText("Oil change")).toBeVisible();
    await expect(app.getByText("30,000 mi")).toBeVisible();
  });

  test("edit a vehicle updates the detail", async ({ app, api }) => {
    const c = await makeCustomer(api, `VEdit ${uniq()}`);
    const v = await (
      await api.post("/vehicles", { data: { customer_id: c.id, make: "Honda", model: "Accord", year: 2017, vin: uniq("VIN") } })
    ).json();

    await gotoReady(app, `/vehicles/${v.id}`);
    await app.getByRole("button", { name: /^edit$/i }).click();
    await app.getByPlaceholder("Model").fill("Pilot");
    await app.getByPlaceholder("Color").fill("Silver");
    await app.getByRole("button", { name: /save changes/i }).click();

    await expect(app.getByTestId("vehicle-title")).toContainText("Pilot");
    await expect(app.getByText("Silver")).toBeVisible();
  });

  test("delete a vehicle removes it and returns to the list", async ({ app, api }) => {
    const c = await makeCustomer(api, `VDel ${uniq()}`);
    const v = await (
      await api.post("/vehicles", { data: { customer_id: c.id, make: "Mazda", model: "CX5", year: 2016, vin: uniq("VIN") } })
    ).json();

    await gotoReady(app, `/vehicles/${v.id}`);
    app.on("dialog", (d) => d.accept());
    await app.getByRole("button", { name: /^delete$/i }).click();

    await expect(app).toHaveURL(/\/vehicles$/);
    const res = await api.get(`/vehicles/${v.id}`);
    expect(res.status()).toBe(404);
  });

  test("unknown vehicle id shows a not-found state", async ({ app }) => {
    await gotoReady(app, "/vehicles/00000000-0000-0000-0000-0000000000ff");
    await expect(app.getByText(/vehicle not found/i)).toBeVisible();
  });
});
