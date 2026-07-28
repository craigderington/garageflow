import { test, expect } from "./helpers/fixtures";
import { API_URL } from "./helpers/api";

test.describe("health", () => {
  // Vigil polls this. It must stay unauthenticated, or monitoring silently
  // starts alerting on 401s instead of on real outages.
  test("healthz reports every dependency without a session", async ({ request }) => {
    const res = await request.get(`${API_URL}/healthz`);

    expect(res.status()).toBe(200);
    expect(res.headers()["cache-control"]).toContain("no-store");

    const body = await res.json();
    expect(body.status).toBe("ok");
    expect(body.checks).toMatchObject({ postgres: "ok", redis: "ok" });
  });
});
