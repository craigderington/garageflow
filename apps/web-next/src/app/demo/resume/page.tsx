"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Loader2, Wrench } from "lucide-react";

import { DemoCapture } from "@/components/DemoCapture";
import { api } from "@/lib/api";

// Landing page for the "come back to your demo shop" link emailed after a
// prospect captures the demo. Unlike /auth/verify, there is no useAuth
// method for this exchange (it isn't a real login), so we call the API
// directly and — like DemoCapture — hard-navigate on success. AuthProvider
// only reads the session cookie on mount, so a router.push here would land
// on /dashboard while the context still believes we're logged out.
function ResumeForm() {
  const searchParams = useSearchParams();

  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  // A resume token is single-use, so a re-render must never spend it twice.
  const attempted = useRef(false);

  useEffect(() => {
    const token = searchParams.get("token");
    if (!token) {
      setLoading(false);
      setError(
        "That demo link has expired. Demo shops last 14 days — enter your email to start a fresh one.",
      );
      return;
    }
    if (attempted.current) return;
    attempted.current = true;

    api
      .post("/demo/resume", { token })
      .then(() => {
        window.location.href = "/dashboard";
      })
      .catch(() => {
        setError(
          "That demo link has expired. Demo shops last 14 days — enter your email to start a fresh one.",
        );
        setLoading(false);
      });
  }, [searchParams]);

  if (loading) {
    return (
      <div className="gf-card overflow-hidden">
        <div className="hazard h-1.5 w-full opacity-90" />
        <div className="p-6 flex items-center gap-3">
          <Loader2 className="w-4 h-4 animate-spin text-amber" />
          <p className="text-sm text-ink-dim">Reopening your demo shop...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="gf-card overflow-hidden">
      <div className="hazard h-1.5 w-full opacity-90" />
      <div className="p-6">
        <div className="p-3 bg-rose-500/10 border border-rose-500/25 rounded-md mb-4">
          <p className="text-sm text-rose-300">{error}</p>
        </div>
        <DemoCapture />
      </div>
    </div>
  );
}

export default function DemoResumePage() {
  return (
    <div className="min-h-screen bg-base flex items-center justify-center p-4 relative overflow-hidden">
      <div className="pointer-events-none absolute -top-40 left-1/2 -translate-x-1/2 w-[600px] h-[600px] rounded-full bg-amber/10 blur-[120px]" />

      <div className="w-full max-w-sm relative">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-amber shadow-[0_0_30px_-6px_var(--color-amber)] mb-4">
            <Wrench className="w-7 h-7 text-black" strokeWidth={2.5} />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-ink">
            Garage<span className="text-amber">Flow</span>
          </h1>
          <p className="text-sm text-ink-dim mt-1">Reopening your demo shop</p>
        </div>

        {/* useSearchParams forces client rendering up to the nearest boundary,
            so the shell above can still be prerendered. */}
        <Suspense
          fallback={
            <div className="gf-card p-6 flex items-center gap-3">
              <Loader2 className="w-4 h-4 animate-spin text-amber" />
              <p className="text-sm text-ink-dim">Loading...</p>
            </div>
          }
        >
          <ResumeForm />
        </Suspense>
      </div>
    </div>
  );
}
