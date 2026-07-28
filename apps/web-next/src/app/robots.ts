import type { MetadataRoute } from "next";

import { SITE_URL } from "@/lib/site";

// Only the marketing landing at / is meant to be indexed. Everything else is
// either behind auth or reached through an unguessable token, and both must
// stay out of search results:
//
//   /inspect/[token]  DVI reports sent to customers. The token is the only
//                     thing protecting them, so an indexed URL is a leak of
//                     one customer's vehicle photos and findings.
//   /portal           customer-facing estimates and service history.
//
// Disallow is a crawler courtesy, not access control — the noindex headers in
// those routes' own metadata are what actually keep them out of an index.
export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        disallow: [
          "/api/",
          "/customers",
          "/dashboard",
          "/estimates",
          "/inspect",
          "/inspections",
          "/inventory",
          "/labor",
          "/login",
          "/portal",
          "/repair-orders",
          "/schedule",
          "/vehicles",
        ],
      },
    ],
    sitemap: `${SITE_URL}/sitemap.xml`,
  };
}
