import { test, expect } from "./helpers/fixtures";
import { API_URL, uniq } from "./helpers/api";

// End-to-end proof of the realtime path, which no app code exercises yet:
//
//   POST /repair-orders -> Bus.Publish -> Redis pub/sub -> Hub -> SSE frame
//
// Driven through EventSource in the browser rather than an APIRequestContext,
// because Playwright's request API buffers a response and a stream never ends.
test.describe("realtime events", () => {
  test("a repair order created over the API arrives on the SSE stream", async ({ app, api }) => {
    await app.goto("/dashboard");

    // Buffer every frame rather than resolving on the first one. The whole
    // suite runs in parallel against one shop, so other specs are publishing to
    // this same stream — the frame we want is not necessarily the first to
    // arrive. Waiting on "open" before publishing matters too: Hub.Publish is
    // non-blocking, so an event sent before the client is subscribed is dropped.
    await app.evaluate((apiUrl) => {
      const w = window as unknown as Record<string, unknown>;
      const es = new EventSource(`${apiUrl}/events`, { withCredentials: true });
      const frames: string[] = [];
      w.__es = es;
      w.__frames = frames;
      w.__open = new Promise<void>((resolve) => es.addEventListener("open", () => resolve()));
      es.addEventListener("update", (e) => frames.push((e as MessageEvent).data));
    }, API_URL);

    await app.evaluate(() => (window as unknown as { __open: Promise<void> }).__open);

    const description = `Realtime ${uniq()}`;
    const c = await (await api.post("/customers", { data: { name: `RT ${uniq()}` } })).json();
    const v = await (
      await api.post("/vehicles", {
        data: { customer_id: c.id, make: "Subaru", model: "Outback", year: 2020, vin: uniq("VIN") },
      })
    ).json();
    const ro = await (
      await api.post("/repair-orders", {
        data: { customer_id: c.id, vehicle_id: v.id, description, mileage: 42000 },
      })
    ).json();

    const findFrame = () =>
      app.evaluate((id) => {
        const frames = (window as unknown as { __frames: string[] }).__frames;
        return (
          frames
            .map((f) => {
              try {
                return JSON.parse(f);
              } catch {
                return null;
              }
            })
            .find((e) => e?.data?.repair_order_id === id) ?? null
        );
      }, ro.id);

    await expect.poll(findFrame, { timeout: 15_000 }).not.toBeNull();

    const envelope = await findFrame();
    await app.evaluate(() => (window as unknown as { __es: EventSource }).__es.close());

    expect(envelope.type).toBe("repair_order:created");
    expect(envelope.data.repair_order_id).toBe(ro.id);
    expect(envelope.data.shop_id).toBeTruthy();
  });

  // The stream is the one route deliberately outside the auth group's timeout,
  // so it is also the easiest place to accidentally drop authentication.
  test("the stream rejects an unauthenticated client", async ({ request }) => {
    const res = await request.get(`${API_URL}/events`);
    expect(res.status()).toBe(401);
  });
});
