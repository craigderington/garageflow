"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import { Card, CardHeader } from "@/components/ui/Card";
import { ArrowLeft, Camera, Send, Check, AlertTriangle, CircleAlert, Loader2, Copy, Trash2 } from "lucide-react";

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
type Inspection = {
  id: string;
  repair_order_id: string;
  status: string;
  public_token: string;
  items: Item[];
};

const CONDS: { key: Item["condition"]; label: string; cls: string; activeCls: string; Icon: typeof Check }[] = [
  { key: "good", label: "Good", cls: "text-emerald-300 border-emerald-400/25", activeCls: "bg-emerald-500 text-black border-emerald-500", Icon: Check },
  { key: "attention", label: "Soon", cls: "text-amber-300 border-amber-400/25", activeCls: "bg-amber text-black border-amber", Icon: AlertTriangle },
  { key: "urgent", label: "Now", cls: "text-rose-300 border-rose-400/25", activeCls: "bg-rose-500 text-white border-rose-500", Icon: CircleAlert },
];

export default function InspectionEditor() {
  const { id } = useParams<{ id: string }>();
  const [insp, setInsp] = useState<Inspection | null>(null);
  const [urls, setUrls] = useState<Record<string, string>>({});
  const urlsRef = useRef<Record<string, string>>({});
  const [sending, setSending] = useState(false);
  const [publicUrl, setPublicUrl] = useState("");

  const load = useCallback(async () => {
    const data = await api.get<Inspection>(`/inspections/${id}`).catch(() => null);
    if (!data) return;
    setInsp(data);
    if (data.public_token) setPublicUrl(`${window.location.origin}/inspect/${data.public_token}`);
    const next: Record<string, string> = {};
    await Promise.all(
      data.items.flatMap((it) =>
        it.photos
          .filter((p) => p.content_type.startsWith("image/"))
          .map(async (p) => {
            try {
              next[p.id] = URL.createObjectURL(await api.blob(`/inspections/${id}/photos/${p.id}`));
            } catch {}
          })
      )
    );
    Object.values(urlsRef.current).forEach((u) => URL.revokeObjectURL(u));
    urlsRef.current = next;
    setUrls(next);
  }, [id]);

  useEffect(() => {
    load();
    return () => Object.values(urlsRef.current).forEach((u) => URL.revokeObjectURL(u));
  }, [load]);

  const patchItem = async (itemId: string, body: Partial<Pick<Item, "condition" | "note" | "price">>) => {
    setInsp((cur) => (cur ? { ...cur, items: cur.items.map((it) => (it.id === itemId ? { ...it, ...body } : it)) } : cur));
    await api.patch(`/inspections/${id}/items/${itemId}`, body).catch(() => {});
  };

  const uploadPhoto = async (itemId: string, files: FileList | null) => {
    if (!files?.length) return;
    for (const f of Array.from(files)) {
      const form = new FormData();
      form.append("file", f);
      await api.upload(`/inspections/${id}/items/${itemId}/photos`, form);
    }
    load();
  };

  const deletePhoto = async (photoId: string) => {
    await api.del(`/inspections/${id}/photos/${photoId}`);
    load();
  };

  const send = async () => {
    setSending(true);
    try {
      const res = await api.post<{ url: string }>(`/inspections/${id}/send`);
      setPublicUrl(res.url);
      load();
    } finally {
      setSending(false);
    }
  };

  if (!insp) return <div className="py-20 text-center text-sm text-ink-mute">Loading…</div>;

  const sections = Array.from(new Set(insp.items.map((i) => i.section)));
  const flagged = insp.items.filter((i) => i.condition === "attention" || i.condition === "urgent").length;

  return (
    <div className="space-y-6 max-w-3xl">
      <Link href={`/repair-orders/${insp.repair_order_id}`} className="inline-flex items-center gap-1.5 text-sm text-ink-dim hover:text-amber transition-colors">
        <ArrowLeft className="w-4 h-4" /> Back to repair order
      </Link>

      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex items-center gap-3">
          <span className="h-9 w-1 rounded-full bg-amber shadow-[0_0_12px_-2px_var(--color-amber)]" />
          <div>
            <h1 className="text-xl font-bold tracking-tight text-ink">Vehicle Inspection</h1>
            <p className="text-sm text-ink-dim mt-0.5" data-testid="insp-summary">
              {flagged} item{flagged === 1 ? "" : "s"} flagged · status {insp.status.replace(/_/g, " ")}
            </p>
          </div>
        </div>
        <button onClick={send} disabled={sending} className="gf-btn-primary" data-testid="send-inspection">
          {sending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
          {insp.status === "in_progress" ? "Send to customer" : "Resend"}
        </button>
      </div>

      {publicUrl && (
        <div className="gf-card p-4 flex items-center gap-3 border-amber/30">
          <Check className="w-4 h-4 text-emerald-400 shrink-0" />
          <span className="text-sm text-ink-dim flex-1 truncate">
            Customer link: <span className="text-ink nums">{publicUrl}</span>
          </span>
          <button onClick={() => navigator.clipboard?.writeText(publicUrl)} className="gf-btn-ghost text-xs px-2.5 py-1.5">
            <Copy className="w-3.5 h-3.5" /> Copy
          </button>
        </div>
      )}

      {sections.map((section) => (
        <Card key={section}>
          <CardHeader>
            <h2 className="gf-eyebrow">{section}</h2>
          </CardHeader>
          <ul className="divide-y divide-line">
            {insp.items
              .filter((it) => it.section === section)
              .map((it) => {
                const flaggedItem = it.condition === "attention" || it.condition === "urgent";
                return (
                  <li key={it.id} className="p-4 space-y-3" data-testid={`item-${it.id}`}>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-sm font-medium text-ink">{it.label}</span>
                      <div className="flex gap-1.5 shrink-0">
                        {CONDS.map(({ key, label, cls, activeCls, Icon }) => (
                          <button
                            key={key}
                            onClick={() => patchItem(it.id, { condition: it.condition === key ? "pending" : key })}
                            aria-label={`${it.label}: ${label}`}
                            aria-pressed={it.condition === key}
                            className={`inline-flex items-center gap-1 px-3 py-2 rounded-md border text-xs font-semibold transition-colors ${
                              it.condition === key ? activeCls : `bg-base ${cls} hover:bg-surface-2`
                            }`}
                          >
                            <Icon className="w-3.5 h-3.5" /> {label}
                          </button>
                        ))}
                      </div>
                    </div>

                    {flaggedItem && (
                      <div className="space-y-3 pl-1">
                        <div className="flex flex-col sm:flex-row gap-3">
                          <input
                            defaultValue={it.note}
                            onBlur={(e) => e.target.value !== it.note && patchItem(it.id, { note: e.target.value })}
                            placeholder="What you found (e.g. 2mm left, metal-on-metal)"
                            className="gf-input flex-1"
                            data-testid={`note-${it.id}`}
                          />
                          <div className="relative sm:w-40">
                            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-mute text-sm">$</span>
                            <input
                              type="number"
                              defaultValue={it.price || ""}
                              onBlur={(e) => Number(e.target.value) !== it.price && patchItem(it.id, { price: Number(e.target.value) })}
                              placeholder="Est. price"
                              className="gf-input pl-7"
                              data-testid={`price-${it.id}`}
                            />
                          </div>
                        </div>

                        <div className="flex flex-wrap items-center gap-2">
                          {it.photos.map((p) => (
                            <div key={p.id} className="relative w-16 h-16 rounded-md overflow-hidden border border-line group">
                              {urls[p.id] ? (
                                // eslint-disable-next-line @next/next/no-img-element
                                <img src={urls[p.id]} alt={p.filename} className="w-full h-full object-cover" />
                              ) : (
                                <div className="w-full h-full bg-surface-2" />
                              )}
                              <button
                                onClick={() => deletePhoto(p.id)}
                                aria-label={`Delete ${p.filename}`}
                                className="absolute top-0.5 right-0.5 p-1 rounded bg-black/60 text-ink-dim opacity-0 group-hover:opacity-100 hover:text-rose-400"
                              >
                                <Trash2 className="w-3 h-3" />
                              </button>
                            </div>
                          ))}
                          <label className="w-16 h-16 rounded-md border border-dashed border-line-strong flex items-center justify-center text-ink-mute hover:text-amber hover:border-amber/50 cursor-pointer transition-colors">
                            <Camera className="w-5 h-5" />
                            <input
                              type="file"
                              accept="image/*"
                              multiple
                              className="hidden"
                              data-testid={`photo-${it.id}`}
                              onChange={(e) => uploadPhoto(it.id, e.target.files)}
                            />
                          </label>
                        </div>
                      </div>
                    )}
                  </li>
                );
              })}
          </ul>
        </Card>
      ))}
    </div>
  );
}
