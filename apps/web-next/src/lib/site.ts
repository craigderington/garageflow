// Canonical public origin, used for sitemap/robots URLs and metadataBase.
// Override with NEXT_PUBLIC_SITE_URL at build time for previews; the default is
// production, so a missing env var yields correct output rather than localhost
// leaking into a deployed sitemap.
export const SITE_URL =
  process.env.NEXT_PUBLIC_SITE_URL?.replace(/\/$/, "") ??
  "https://garageflow.studio";
