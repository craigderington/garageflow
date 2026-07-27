import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M8AAAMBAQDJ/pLvAAAAAElFTkSuQmCC",
  "base64"
);

async function makeRO(api: import("@playwright/test").APIRequestContext, desc: string) {
  const c = await (await api.post("/customers", { data: { name: `DVI ${uniq()}`, phone: "555-0100", email: "dvi@example.com" } })).json();
  const v = await (await api.post("/vehicles", { data: { customer_id: c.id, make: "Toyota", model: "Camry", year: 2019, vin: uniq("VIN") } })).json();
  const ro = await (await api.post("/repair-orders", { data: { customer_id: c.id, vehicle_id: v.id, description: desc } })).json();
  return { c, v, ro };
}

test.describe("digital vehicle inspection", () => {
  test("API: create from template, flag item, photo, send, public report, approve → estimate", async ({ api }) => {
    const { ro } = await makeRO(api, `InspAPI ${uniq()}`);

    const insp = await (await api.post(`/repair-orders/${ro.id}/inspection`)).json();
    expect(insp.items.length).toBeGreaterThanOrEqual(10);
    const item = insp.items[0];

    expect((await api.patch(`/inspections/${insp.id}/items/${item.id}`, { data: { condition: "urgent", price: 250, note: "metal-on-metal" } })).status()).toBe(204);

    const up = await api.post(`/inspections/${insp.id}/items/${item.id}/photos`, {
      multipart: { file: { name: "pad.png", mimeType: "image/png", buffer: PNG } },
    });
    expect(up.status()).toBe(201);

    const send = await (await api.post(`/inspections/${insp.id}/send`)).json();
    const token = send.public_token;
    expect(token).toBeTruthy();

    // public report (no auth needed, but api ctx is fine)
    const report = await (await api.get(`/public/inspections/${token}`)).json();
    expect(report.shop_name).toBeTruthy();
    expect(report.vehicle).toContain("Camry");
    const flagged = report.inspection.items.find((i: { id: string }) => i.id === item.id);
    expect(flagged.photos.length).toBe(1);

    // public photo streams
    const photo = flagged.photos[0];
    const content = await api.get(`/public/inspections/${token}/photos/${photo.id}`);
    expect(content.ok()).toBeTruthy();
    expect(content.headers()["content-type"]).toContain("image/png");

    // approve → estimate created + RO approved
    const approve = await (await api.post(`/public/inspections/${token}/approve`, { data: { item_ids: [item.id] } })).json();
    expect(approve.estimate_created).toBe(true);
    expect(approve.total).toBe(250);

    const est = await (await api.get(`/estimates/ro/${ro.id}`)).json();
    expect(est.estimate.status).toBe("approved");
    expect(est.estimate.total).toBe(250);
    const updatedRO = await (await api.get(`/repair-orders/${ro.id}`)).json();
    expect(updatedRO.status).toBe("approved");
  });

  test("tech UI: start inspection from the RO, flag an item, send", async ({ app, api }) => {
    const { ro } = await makeRO(api, `InspUI ${uniq()}`);

    await gotoReady(app, `/repair-orders/${ro.id}`);
    await app.getByTestId("start-inspection").click();
    await expect(app).toHaveURL(/\/inspections\//);

    // mark "Front brake pads" as Now (urgent)
    await app.getByRole("button", { name: "Front brake pads: Now" }).click();
    // price input appears for flagged items
    const price = app.getByPlaceholder("Est. price");
    await expect(price).toBeVisible();
    await price.fill("320");
    await price.blur();

    await app.getByTestId("send-inspection").click();
    await expect(app.getByText(/customer link/i)).toBeVisible();
  });

  test("customer UI: open public report and approve work", async ({ page, api }) => {
    const { ro } = await makeRO(api, `InspCust ${uniq()}`);
    const insp = await (await api.post(`/repair-orders/${ro.id}/inspection`)).json();
    const item = insp.items[0];
    await api.patch(`/inspections/${insp.id}/items/${item.id}`, { data: { condition: "urgent", price: 180, note: "worn" } });
    const send = await (await api.post(`/inspections/${insp.id}/send`)).json();

    // customer is unauthenticated
    await page.goto(`/inspect/${send.public_token}`);
    await expect(page.getByRole("heading", { name: /your vehicle inspection/i })).toBeVisible();
    await expect(page.getByTestId(`finding-${item.id}`)).toContainText("$180.00");

    await page.getByTestId(`approve-${item.id}`).check();
    await page.getByTestId("approve-submit").click();
    await expect(page.getByTestId("approved-banner")).toBeVisible();

    const est = await (await api.get(`/estimates/ro/${ro.id}`)).json();
    expect(est.estimate.status).toBe("approved");
  });
});
