import { test, expect } from "./helpers/fixtures";
import { API_URL, uniq } from "./helpers/api";

const demoEmail = () => `demo-${uniq()}@example.com`;

test.describe("demo tenant", () => {
  test("email capture lands on a populated dashboard", async ({ page }) => {
    await page.goto("/login");
    await page.getByTestId("demo-email").fill(demoEmail());
    await page.getByTestId("demo-submit").click();

    await expect(page).toHaveURL(/\/dashboard/);
    // The pitch is a shop with work in it; an empty demo demos nothing. Scope
    // to the orders table, not just any link on the page — the sidebar nav
    // renders its own links first in DOM order and would satisfy a bare
    // getByRole("link") query even with zero repair orders.
    await page.goto("/repair-orders");
    await expect(page.getByRole("table").getByRole("link").first()).toBeVisible();
  });

  test("the same email resumes one shop rather than accumulating them", async ({ request }) => {
    const email = demoEmail();

    const first = await request.post(`${API_URL}/demo`, { data: { email } });
    expect(first.status()).toBe(200);
    const meAfterFirst = await request.get(`${API_URL}/auth/me`);
    expect(meAfterFirst.status()).toBe(200);
    const { shop_id: shopIdAfterFirst } = await meAfterFirst.json();
    expect(shopIdAfterFirst).toBeTruthy();

    const second = await request.post(`${API_URL}/demo`, { data: { email } });
    expect(second.status()).toBe(200);
    const meAfterSecond = await request.get(`${API_URL}/auth/me`);
    expect(meAfterSecond.status()).toBe(200);
    const { shop_id: shopIdAfterSecond } = await meAfterSecond.json();

    // Proof, not just a 200: the second call must resolve to the exact same
    // shop as the first, not a second shop for the same prospect.
    expect(shopIdAfterSecond).toBe(shopIdAfterFirst);
  });

  test("provisioning never returns a token in the body", async ({ request }) => {
    const res = await request.post(`${API_URL}/demo`, { data: { email: demoEmail() } });

    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).not.toMatch(/token/i);
    expect(body).not.toMatch(/password/i);
  });

  test("an invalid address is rejected", async ({ request }) => {
    const res = await request.post(`${API_URL}/demo`, { data: { email: "nope" } });
    expect(res.status()).toBe(400);
  });

  test("a bad resume token explains itself", async ({ page }) => {
    await page.goto("/demo/resume?token=not-a-real-token");
    await expect(page.getByText(/expired/i)).toBeVisible();
  });

  // The guard that stops a prospect texting a stranger.
  test("a demo shop sends nothing outbound but still shows link and QR", async ({ page }) => {
    // Provision via page.request (not the standalone `request` fixture) so the
    // session cookie lands in this test's browser context — the plain `request`
    // fixture is a separate APIRequestContext with its own cookie jar and would
    // leave `page` unauthenticated.
    await page.request.post(`${API_URL}/demo`, { data: { email: demoEmail() } });

    // The seed (internal/demo/seed.go) always attaches its one inspection to
    // the first repair order ("Brake inspection..."); the second ("Oil
    // change...") is seeded without one. Pick that one explicitly rather than
    // "the first RO in the table" — table order is created_at DESC and not
    // guaranteed to disagree with seed order, so a positional pick would be
    // one query-planner tiebreak away from silently exercising the wrong RO.
    const orders = await (await page.request.get(`${API_URL}/repair-orders`)).json();
    const ro = orders.find((o: { description: string }) => o.description.includes("Oil change"));
    expect(ro).toBeTruthy();

    await page.goto(`/repair-orders/${ro.id}`);
    await page.getByTestId("start-inspection").click();
    await expect(page).toHaveURL(/\/inspections\//);

    await page.getByTestId("send-inspection").click();
    await expect(page.getByTestId("customer-link")).toBeVisible();

    await page.getByTestId("toggle-qr").click();
    await expect(page.getByTestId("qr-image")).toHaveAttribute("src", /^data:image\/png;base64,/);
  });
});
