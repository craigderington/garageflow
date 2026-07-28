"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowRight, Loader2, Mail, Wrench } from "lucide-react";

import { useAuth } from "@/hooks/useAuth";
import { ApiError } from "@/lib/api";

// Landing page for the link in the magic-link email. Without this route the
// emailed link 404s, which is what it did before: the API builds
// {APP_URL}/auth/verify?code=... and nothing served that path.
//
// Also accepts a manually typed code, because the same email offers "or enter
// this code" and some mail clients mangle long links.
function VerifyForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { verifyCode } = useAuth();

  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  // A code is single-use, so a re-render must never spend it twice.
  const attempted = useRef(false);

  const submit = async (value: string) => {
    const trimmed = value.trim();
    if (!trimmed) return;
    setError("");
    setLoading(true);
    try {
      await verifyCode(trimmed);
      router.push("/dashboard");
    } catch (err) {
      setError(
        err instanceof ApiError
          ? "That link is invalid or has expired. Magic links last 15 minutes."
          : "Could not sign you in. Please try again.",
      );
      setLoading(false);
    }
  };

  useEffect(() => {
    const fromLink = searchParams.get("code");
    if (fromLink && !attempted.current) {
      attempted.current = true;
      setCode(fromLink);
      submit(fromLink);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  return (
    <div className="gf-card overflow-hidden">
      <div className="hazard h-1.5 w-full opacity-90" />
      <div className="p-6">
        {loading ? (
          <div className="flex items-center gap-3 py-2">
            <Loader2 className="w-4 h-4 animate-spin text-amber" />
            <p className="text-sm text-ink-dim">Signing you in...</p>
          </div>
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              submit(code);
            }}
            className="space-y-4"
          >
            <div>
              <label className="gf-label">Sign-in code</label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-ink-mute" />
                <input
                  type="text"
                  required
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  className="gf-input pl-9 nums"
                  placeholder="Paste the code from your email"
                />
              </div>
            </div>

            {error && (
              <div className="p-3 bg-rose-500/10 border border-rose-500/25 rounded-md">
                <p className="text-sm text-rose-300">{error}</p>
              </div>
            )}

            <button type="submit" className="gf-btn-primary w-full">
              <ArrowRight className="w-4 h-4" />
              Sign In
            </button>
          </form>
        )}

        <div className="mt-4 pt-4 border-t border-line">
          <a
            href="/login"
            className="block w-full text-center text-sm text-ink-dim hover:text-ink transition-colors"
          >
            Back to sign in
          </a>
        </div>
      </div>
    </div>
  );
}

export default function VerifyPage() {
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
          <p className="text-sm text-ink-dim mt-1">Confirming your sign-in link</p>
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
          <VerifyForm />
        </Suspense>
      </div>
    </div>
  );
}
