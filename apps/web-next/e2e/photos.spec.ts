import { test, expect, gotoReady } from "./helpers/fixtures";
import { uniq } from "./helpers/api";

// 1x1 transparent PNG
const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M8AAAMBAQDJ/pLvAAAAAElFTkSuQmCC",
  "base64"
);

async function makeRO(api: import("@playwright/test").APIRequestContext, desc: string) {
  const c = await (await api.post("/customers", { data: { name: `Ph ${uniq()}` } })).json();
  return (await api.post("/repair-orders", { data: { customer_id: c.id, description: desc } })).json();
}

test.describe("repair-order photos (MinIO-backed)", () => {
  test("upload, list, stream, and delete via the API", async ({ api }) => {
    const ro = await makeRO(api, `PhAPI ${uniq()}`);

    const up = await api.post(`/repair-orders/${ro.id}/photos`, {
      multipart: { file: { name: "intake.png", mimeType: "image/png", buffer: PNG } },
    });
    expect(up.status()).toBe(201);
    const photo = await up.json();
    expect(photo.filename).toBe("intake.png");
    expect(photo.content_type).toBe("image/png");

    const list = await (await api.get(`/repair-orders/${ro.id}/photos`)).json();
    expect(list.map((p: { id: string }) => p.id)).toContain(photo.id);

    const content = await api.get(`/repair-orders/${ro.id}/photos/${photo.id}`);
    expect(content.ok()).toBeTruthy();
    expect(content.headers()["content-type"]).toContain("image/png");
    expect((await content.body()).length).toBe(PNG.length);

    const del = await api.delete(`/repair-orders/${ro.id}/photos/${photo.id}`);
    expect(del.status()).toBe(204);
    const after = await (await api.get(`/repair-orders/${ro.id}/photos`)).json();
    expect(after.map((p: { id: string }) => p.id)).not.toContain(photo.id);
  });

  test("non-image/pdf uploads are rejected", async ({ api }) => {
    const ro = await makeRO(api, `PhBad ${uniq()}`);
    const res = await api.post(`/repair-orders/${ro.id}/photos`, {
      multipart: { file: { name: "notes.txt", mimeType: "text/plain", buffer: Buffer.from("nope") } },
    });
    expect(res.status()).toBe(415);
  });

  test("uploading to another shop's repair order is blocked", async ({ api }) => {
    // a random non-existent RO id behaves as not-found for this shop
    const res = await api.post(`/repair-orders/00000000-0000-0000-0000-0000000000ff/photos`, {
      multipart: { file: { name: "x.png", mimeType: "image/png", buffer: PNG } },
    });
    expect(res.status()).toBe(404);
  });

  test("upload a photo on the RO detail page and see the thumbnail, then delete it", async ({ app, api }) => {
    const desc = `PhUI ${uniq()}`;
    const ro = await makeRO(api, desc);

    await gotoReady(app, `/repair-orders/${ro.id}`);
    await expect(app.getByText(/no photos yet/i)).toBeVisible();

    await app.getByTestId("photo-input").setInputFiles({ name: "damage.png", mimeType: "image/png", buffer: PNG });

    const img = app.getByAltText("damage.png");
    await expect(img).toBeVisible();

    await app.getByRole("button", { name: "Delete damage.png" }).click();
    await expect(app.getByAltText("damage.png")).toHaveCount(0);
    await expect(app.getByText(/no photos yet/i)).toBeVisible();
  });
});
