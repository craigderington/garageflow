"use client";

import { useState } from "react";
import { ArrowRight, Loader2, Mail } from "lucide-react";

import { api, ApiError } from "@/lib/api";

// Self-contained email capture for the throwaway demo tenant. On success the
// API has already set the session cookie, so we hard-navigate to /dashboard
// (not router.push) so AuthProvider remounts and re-reads /auth/me — a soft
// navigation would land on the dashboard still believing we're logged out.
export function DemoCapture() {
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await api.post("/demo", { email });
      window.location.href = "/dashboard";
    } catch (err) {
      if (err instanceof ApiError && err.status === 429) {
        setError("Too many demo requests just now. Try again in a minute.");
      } else {
        setError(err instanceof ApiError ? err.message : "Could not start your demo shop.");
      }
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      <div>
        <label className="gf-label">Work email</label>
        <div className="relative">
          <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-ink-mute" />
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="gf-input pl-9"
            placeholder="you@shop.com"
            data-testid="demo-email"
          />
        </div>
      </div>

      {error && (
        <div className="p-3 bg-rose-500/10 border border-rose-500/25 rounded-md">
          <p className="text-sm text-rose-300">{error}</p>
        </div>
      )}

      <button type="submit" disabled={loading} data-testid="demo-submit" className="gf-btn-primary w-full">
        {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <ArrowRight className="w-4 h-4" />}
        {loading ? "Setting up your shop..." : "Try the demo"}
      </button>
    </form>
  );
}
