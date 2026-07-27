import { request, type APIRequestContext } from "@playwright/test";

export const API_URL = process.env.API_URL || "http://localhost:8081";

export type SeedUser = "owner" | "writer" | "tech";
export const SEED_EMAIL: Record<SeedUser, string> = {
  owner: "owner@garageflow.app",
  writer: "writer@garageflow.app",
  tech: "tech@garageflow.app",
};
export const SEED_PASSWORD = "password123";

/**
 * Logs in via the dev magic-link flow (which returns the code directly in dev)
 * and returns the raw `session` cookie value. No email mocking needed.
 */
export async function sessionFor(email: string): Promise<string> {
  const ctx = await request.newContext({ baseURL: API_URL });
  try {
    const linkRes = await ctx.post("/auth/magic-link", { data: { email } });
    if (!linkRes.ok()) throw new Error(`magic-link failed: ${linkRes.status()} ${await linkRes.text()}`);
    const { code } = (await linkRes.json()) as { code: string };

    const verifyRes = await ctx.post("/auth/verify", { data: { code } });
    if (!verifyRes.ok()) throw new Error(`verify failed: ${verifyRes.status()} ${await verifyRes.text()}`);

    const cookie = verifyRes
      .headers()
      ["set-cookie"]?.split(/,(?=\s*session=)/) // be tolerant of multiple cookies
      .map((c) => c.trim())
      .find((c) => c.startsWith("session="));
    if (cookie) return cookie.slice("session=".length).split(";")[0];

    const { session } = (await verifyRes.json()) as { session: string };
    return session;
  } finally {
    await ctx.dispose();
  }
}

/** An authenticated API request context for a given seed user (server-side calls in tests). */
export async function apiAs(user: SeedUser = "owner"): Promise<APIRequestContext> {
  const session = await sessionFor(SEED_EMAIL[user]);
  return request.newContext({
    baseURL: API_URL,
    extraHTTPHeaders: { cookie: `session=${session}` },
  });
}

/** Unique suffix so parallel tests never collide on names. */
export function uniq(prefix = ""): string {
  return `${prefix}${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`;
}
