import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

async function approvedEstimate(api: import("@playwright/test").APIRequestContext, desc: string) {
  const c = await (await api.post("/customers", { data: { name: `Pay ${uniq()}` } })).json();
  const v = await (await api.post("/vehicles", { data: { customer_id: c.id, make: "Audi", model: "A4", year: 2020, vin: uniq("VIN") } })).json();
  const ro = await (await api.post("/repair-orders", { data: { customer_id: c.id, vehicle_id: v.id, description: desc } })).json();
  const est = await (
    await api.post("/estimates", { data: { repair_order_id: ro.id, items: [{ type: "part", description: "Pads", quantity: 1, unit_price: 200 }] } })
  ).json();
  await api.post(`/estimates/${est.id}/send`);
  await api.post(`/estimates/${est.id}/approve`);
  return { ro, est };
}

test.describe("payments (Stripe, dev-mode settlement)", () => {
  test("collect payment on an approved estimate marks it paid", async ({ app, api }) => {
    const desc = `PayUI ${uniq()}`;
    await approvedEstimate(api, desc);

    await gotoReady(app, "/estimates");
    const row = app.getByRole("row").filter({ hasText: desc });
    await expect(row).toContainText(/approved/i);
    await row.getByRole("button", { name: /collect payment/i }).click();
    await expect(row).toContainText(/paid/i);
  });

  test("pay endpoint returns a checkout session and settles in dev mode", async ({ api }) => {
    const { est } = await approvedEstimate(api, `PayAPI ${uniq()}`);
    const res = await api.post(`/estimates/${est.id}/pay`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.mode).toBe("dev");
    expect(body.url).toContain("/estimates?paid=");

    // estimate is now paid
    const check = await (await api.get(`/estimates/ro/${est.repair_order_id}`)).json();
    expect(check.estimate.status).toBe("paid");
  });

  test("paying an already-paid estimate is rejected", async ({ api }) => {
    const { est } = await approvedEstimate(api, `PayDup ${uniq()}`);
    await api.post(`/estimates/${est.id}/pay`);
    const again = await api.post(`/estimates/${est.id}/pay`);
    expect(again.status()).toBe(409);
  });

  test("stripe webhook marks the estimate paid by metadata", async ({ api }) => {
    const { est } = await approvedEstimate(api, `PayHook ${uniq()}`);
    const event = {
      type: "checkout.session.completed",
      data: { object: { metadata: { estimate_id: est.id } } },
    };
    const res = await api.post("/webhooks/stripe", { data: event });
    expect(res.ok()).toBeTruthy();

    const check = await (await api.get(`/estimates/ro/${est.repair_order_id}`)).json();
    expect(check.estimate.status).toBe("paid");
  });
});
