import type { MetadataRoute } from "next";

import { SITE_URL } from "@/lib/site";

// Deliberately just the marketing landing. Every other route is authenticated
// or token-gated, so listing it would either 302 crawlers to /login or expose
// a customer's DVI report. Add public marketing pages here as they are built.
export default function sitemap(): MetadataRoute.Sitemap {
  return [
    {
      url: SITE_URL,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 1,
    },
  ];
}
