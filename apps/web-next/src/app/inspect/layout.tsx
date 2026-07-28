import type { Metadata } from "next";

// DVI reports are reached by an unguessable token in the URL, which is the only
// thing protecting them. If one is ever indexed, a customer's vehicle photos
// and findings become public — and search engines learn URLs from referrers and
// toolbars, not just from crawling, so robots.txt alone is not enough.
//
// This lives in a layout because the page itself is a client component and
// cannot export metadata.
export const metadata: Metadata = {
  robots: {
    index: false,
    follow: false,
    nocache: true,
    googleBot: { index: false, follow: false },
  },
};

export default function InspectLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return <>{children}</>;
}
