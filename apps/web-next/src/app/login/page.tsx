"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/useAuth";
import { api, ApiError } from "@/lib/api";
import { Wrench, Mail, Lock, ArrowRight, Loader2 } from "lucide-react";

export default function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [mode, setMode] = useState<"magic" | "password">("password");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const { login } = useAuth();
  const router = useRouter();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setMessage("");
    setLoading(true);
    try {
      if (mode === "magic") {
        const res = await api.post<{ code: string }>("/auth/magic-link", { email });
        setMessage(`Magic link sent! Use code: ${res.code} (dev mode)`);
      } else {
        await login(email, password);
        router.push("/dashboard");
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to sign in");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-base flex items-center justify-center p-4 relative overflow-hidden">
      {/* ambient amber glow */}
      <div className="pointer-events-none absolute -top-40 left-1/2 -translate-x-1/2 w-[600px] h-[600px] rounded-full bg-amber/10 blur-[120px]" />

      <div className="w-full max-w-sm relative">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-amber shadow-[0_0_30px_-6px_var(--color-amber)] mb-4">
            <Wrench className="w-7 h-7 text-black" strokeWidth={2.5} />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-ink">
            Garage<span className="text-amber">Flow</span>
          </h1>
          <p className="text-sm text-ink-dim mt-1">Sign in to your shop</p>
        </div>

        <div className="gf-card overflow-hidden">
          {/* hazard top strip */}
          <div className="hazard h-1.5 w-full opacity-90" />
          <div className="p-6">
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="gf-label">Email</label>
                <div className="relative">
                  <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-ink-mute" />
                  <input
                    type="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="gf-input pl-9"
                    placeholder="you@shop.com"
                  />
                </div>
              </div>

              {mode === "password" && (
                <div>
                  <label className="gf-label">Password</label>
                  <div className="relative">
                    <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-ink-mute" />
                    <input
                      type="password"
                      required
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      className="gf-input pl-9"
                      placeholder="Enter your password"
                    />
                  </div>
                </div>
              )}

              {error && (
                <div className="p-3 bg-rose-500/10 border border-rose-500/25 rounded-md">
                  <p className="text-sm text-rose-300">{error}</p>
                </div>
              )}
              {message && (
                <div className="p-3 bg-emerald-400/10 border border-emerald-400/25 rounded-md">
                  <p className="text-sm text-emerald-300">{message}</p>
                </div>
              )}

              <button type="submit" disabled={loading} className="gf-btn-primary w-full">
                {loading ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <ArrowRight className="w-4 h-4" />
                )}
                {loading ? "Signing in..." : mode === "magic" ? "Send Magic Link" : "Sign In"}
              </button>
            </form>

            <div className="mt-4 pt-4 border-t border-line">
              <button
                onClick={() => setMode(mode === "magic" ? "password" : "magic")}
                className="w-full text-center text-sm text-ink-dim hover:text-ink transition-colors"
              >
                {mode === "magic" ? "Sign in with password instead" : "Send magic link instead"}
              </button>
            </div>
          </div>
        </div>

        <p className="text-center text-xs text-ink-mute mt-6">
          Demo: <span className="nums text-ink-dim">owner@garageflow.app</span> /{" "}
          <span className="nums text-ink-dim">password123</span>
        </p>
      </div>
    </div>
  );
}
