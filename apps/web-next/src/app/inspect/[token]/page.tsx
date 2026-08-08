"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Check, AlertTriangle, CircleAlert, Wrench, ShieldCheck } from "lucide-react";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "/api";

type Photo = { id: string; filename: string; content_type: string };
type Item = {
  id: string;
  section: string;
  label: string;
  condition: "pending" | "good" | "attention" | "urgent";
  note: string;
  price: number;
  approved: boolean;
  photos: Photo[];
};
type Report = {
  inspection: { id: string; status: string; items: Item[] };
  shop_name: string;
  vehicle: string;
  customer: string;
};

const money = (n: number) => n.toLocaleString("en-US", { style: "currency", currency: "USD" });

const COND: Record<string, { label: string; ring: string; text: string; Icon: typeof Check }> = {
  good: { label: "Looks good", ring: "ring-emerald-400/30 bg-emerald-400/5", text: "text-emerald-300", Icon: Check },
  attention: { label: "Needs attention soon", ring: "ring-amber-400/30 bg-amber-400/5", text: "text-amber-300", Icon: AlertTriangle },
  urgent: { label: "Recommended now", ring: "ring-rose-400/30 bg-rose-400/5", text: "text-rose-300", Icon: CircleAlert },
};

export default function PublicInspection() {
  const { token } = useParams<{ token: string }>();
  const [report, setReport] = useState<Report | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "notfound">("loading");
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/public/inspections/${token}`);
      if (!res.ok) throw new Error("nf");
      const data: Report = await res.json();
      setReport(data);
      setStatus("ready");
      setDone(data.inspection.status === "reviewed");
      const pre: Record<string, boolean> = {};
      data.inspection.items.forEach((i) => {
        if (i.approved) pre[i.id] = true;
      });
      setSelected(pre);
    } catch {
      setStatus("notfound");
    }
  }, [token]);

  useEffect(() => {
    load();
  }, [load]);

  if (status === "loading") return <div className="min-h-screen bg-base flex items-center justify-center text-ink-mute">Loading…</div>;
  if (status === "notfound" || !report)
    return (
      <div className="min-h-screen bg-base flex items-center justify-center p-6 text-center">
        <p className="text-ink-mute">This inspection link is no longer available.</p>
      </div>
    );

  const items = report.inspection.items.filter((i) => i.condition !== "pending");
  const recommended = items.filter((i) => i.condition === "attention" || i.condition === "urgent");
  const sections = Array.from(new Set(items.map((i) => i.section)));
  const total = recommended.filter((i) => selected[i.id]).reduce((s, i) => s + i.price, 0);

  const submit = async () => {
    setSubmitting(true);
    try {
      const ids = recommended.filter((i) => selected[i.id]).map((i) => i.id);
      const res = await fetch(`${API_BASE}/public/inspections/${token}/approve`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ item_ids: ids }),
      });
      if (res.ok) setDone(true);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen bg-base text-ink">
      <div className="hazard h-1.5 w-full" />
      <div className="mx-auto max-w-md px-4 py-8">
        {/* header */}
        <div className="flex items-center gap-2.5 mb-6">
          <span className="w-8 h-8 rounded-lg bg-amber flex items-center justify-center">
            <Wrench className="w-4 h-4 text-black" strokeWidth={2.5} />
          </span>
          <span className="text-sm font-bold">{report.shop_name || "GarageFlow"}</span>
        </div>

        <h1 className="text-2xl font-bold tracking-tight">Your vehicle inspection</h1>
        <p className="text-sm text-ink-dim mt-1">
          {report.vehicle?.trim() || "Your vehicle"}
          {report.customer ? ` · ${report.customer}` : ""}
        </p>

        {done && (
          <div className="mt-5 gf-card p-4 flex items-center gap-3 border-emerald-400/30" data-testid="approved-banner">
            <ShieldCheck className="w-5 h-5 text-emerald-400 shrink-0" />
            <p className="text-sm text-ink-dim">Thanks! Your shop has been notified of the work you approved.</p>
          </div>
        )}

        {/* findings */}
        <div className="mt-6 space-y-6">
          {sections.map((section) => (
            <div key={section}>
              <p className="gf-eyebrow mb-2">{section}</p>
              <div className="space-y-2.5">
                {items
                  .filter((i) => i.section === section)
                  .map((i) => {
                    const c = COND[i.condition];
                    const Icon = c.Icon;
                    const approvable = i.condition === "attention" || i.condition === "urgent";
                    return (
                      <div key={i.id} className={`rounded-lg ring-1 ${c.ring} p-3.5`} data-testid={`finding-${i.id}`}>
                        <div className="flex items-start gap-2.5">
                          <Icon className={`w-4 h-4 mt-0.5 shrink-0 ${c.text}`} />
                          <div className="min-w-0 flex-1">
                            <div className="flex items-center justify-between gap-2">
                              <p className="text-sm font-semibold text-ink">{i.label}</p>
                              {approvable && i.price > 0 && <span className="text-sm font-bold text-amber nums shrink-0">{money(i.price)}</span>}
                            </div>
                            <p className={`text-xs ${c.text} mt-0.5`}>{c.label}</p>
                            {i.note && <p className="text-xs text-ink-dim mt-1">{i.note}</p>}
                            {i.photos.length > 0 && (
                              <div className="flex flex-wrap gap-2 mt-2.5">
                                {i.photos.map((p) => (
                                  // eslint-disable-next-line @next/next/no-img-element
                                  <img
                                    key={p.id}
                                    src={`${API_BASE}/public/inspections/${token}/photos/${p.id}`}
                                    alt={p.filename}
                                    className="w-20 h-20 rounded-md object-cover border border-line"
                                  />
                                ))}
                              </div>
                            )}
                            {approvable && !done && (
                              <label className="mt-2.5 flex items-center gap-2 text-sm text-ink-dim cursor-pointer">
                                <input
                                  type="checkbox"
                                  checked={!!selected[i.id]}
                                  onChange={(e) => setSelected((s) => ({ ...s, [i.id]: e.target.checked }))}
                                  data-testid={`approve-${i.id}`}
                                  className="w-4 h-4 accent-amber"
                                />
                                Approve this work
                              </label>
                            )}
                          </div>
                        </div>
                      </div>
                    );
                  })}
              </div>
            </div>
          ))}
        </div>

        {!done && recommended.length > 0 && (
          <div className="sticky bottom-0 mt-8 -mx-4 px-4 py-4 bg-base/90 backdrop-blur border-t border-line">
            <button onClick={submit} disabled={submitting} className="gf-btn-primary w-full" data-testid="approve-submit">
              {submitting ? "Submitting…" : total > 0 ? `Approve selected work · ${money(total)}` : "Approve selected work"}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
