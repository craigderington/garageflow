"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import type { RepairOrder, RepairOrderStatus, Customer, Vehicle, Estimate, EstimateItem, LaborLog } from "@/lib/types";
import { Card, CardContent, CardHeader } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { EstimateEditor } from "@/components/EstimateEditor";
import { ArrowLeft, User, Car, FileText, Wrench, ClipboardList, ImagePlus, Trash2, Loader2, FileText as FileIcon, ClipboardCheck, ArrowRight, Pencil } from "lucide-react";

function InspectionSection({ roId }: { roId: string }) {
  const router = useRouter();
  const [insp, setInsp] = useState<{ id: string; status: string; items: { condition: string }[] } | null>(null);
  const [loading, setLoading] = useState(true);
  const [starting, setStarting] = useState(false);

  useEffect(() => {
    api
      .get<{ id: string; status: string; items: { condition: string }[] }>(`/repair-orders/${roId}/inspection`)
      .then(setInsp)
      .catch(() => setInsp(null))
      .finally(() => setLoading(false));
  }, [roId]);

  const start = async () => {
    setStarting(true);
    try {
      const created = await api.post<{ id: string }>(`/repair-orders/${roId}/inspection`);
      router.push(`/inspections/${created.id}`);
    } finally {
      setStarting(false);
    }
  };

  if (loading) return null;
  const flagged = insp?.items.filter((i) => i.condition === "attention" || i.condition === "urgent").length ?? 0;

  return (
    <Card>
      <CardHeader className="flex items-center justify-between">
        <h2 className="gf-eyebrow flex items-center gap-2">
          <ClipboardCheck className="w-3.5 h-3.5 text-amber" /> Digital Inspection
        </h2>
        {insp && <Badge status={insp.status === "reviewed" ? "approved" : insp.status}>{insp.status.replace(/_/g, " ")}</Badge>}
      </CardHeader>
      <CardContent className="flex items-center justify-between gap-4">
        {insp ? (
          <>
            <p className="text-sm text-ink-dim">
              {flagged} item{flagged === 1 ? "" : "s"} flagged for the customer.
            </p>
            <Link href={`/inspections/${insp.id}`} className="gf-btn-secondary text-sm" data-testid="open-inspection">
              Open inspection <ArrowRight className="w-3.5 h-3.5" />
            </Link>
          </>
        ) : (
          <>
            <p className="text-sm text-ink-mute">No inspection yet. Walk the car and send the customer a photo report.</p>
            <button onClick={start} disabled={starting} className="gf-btn-primary text-sm" data-testid="start-inspection">
              {starting ? <Loader2 className="w-4 h-4 animate-spin" /> : <ClipboardCheck className="w-4 h-4" />}
              Start inspection
            </button>
          </>
        )}
      </CardContent>
    </Card>
  );
}

type Photo = {
  id: string;
  repair_order_id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
};

function PhotosCard({ roId }: { roId: string }) {
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [urls, setUrls] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const urlsRef = useRef<Record<string, string>>({});

  const load = useCallback(async () => {
    const list = await api.get<Photo[]>(`/repair-orders/${roId}/photos`).catch(() => [] as Photo[]);
    setPhotos(list);
    const next: Record<string, string> = {};
    await Promise.all(
      list
        .filter((p) => p.content_type.startsWith("image/"))
        .map(async (p) => {
          try {
            const b = await api.blob(`/repair-orders/${roId}/photos/${p.id}`);
            next[p.id] = URL.createObjectURL(b);
          } catch {
            /* ignore */
          }
        })
    );
    Object.values(urlsRef.current).forEach((u) => URL.revokeObjectURL(u));
    urlsRef.current = next;
    setUrls(next);
  }, [roId]);

  useEffect(() => {
    load();
    return () => Object.values(urlsRef.current).forEach((u) => URL.revokeObjectURL(u));
  }, [load]);

  const onPick = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    setBusy(true);
    try {
      for (const f of Array.from(files)) {
        const form = new FormData();
        form.append("file", f);
        await api.upload(`/repair-orders/${roId}/photos`, form);
      }
      await load();
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  };

  const remove = async (id: string) => {
    await api.del(`/repair-orders/${roId}/photos/${id}`);
    load();
  };

  return (
    <Card>
      <CardHeader className="flex items-center justify-between">
        <h2 className="gf-eyebrow flex items-center gap-2">
          <ImagePlus className="w-3.5 h-3.5 text-amber" /> Photos
        </h2>
        <button onClick={() => fileRef.current?.click()} disabled={busy} className="gf-btn-secondary text-xs px-2.5 py-1.5">
          {busy ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <ImagePlus className="w-3.5 h-3.5" />}
          {busy ? "Uploading…" : "Add photos"}
        </button>
        <input
          ref={fileRef}
          type="file"
          accept="image/*,application/pdf"
          multiple
          onChange={onPick}
          data-testid="photo-input"
          className="hidden"
        />
      </CardHeader>
      {photos.length === 0 ? (
        <div className="px-5 py-8 text-center text-sm text-ink-mute">No photos yet — add intake or damage shots.</div>
      ) : (
        <div className="p-5 grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          {photos.map((p) => (
            <div key={p.id} className="group relative aspect-square rounded-lg overflow-hidden border border-line bg-base">
              {p.content_type.startsWith("image/") && urls[p.id] ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={urls[p.id]} alt={p.filename} className="w-full h-full object-cover" />
              ) : (
                <div className="w-full h-full flex flex-col items-center justify-center gap-2 text-ink-mute p-2">
                  <FileIcon className="w-7 h-7" />
                  <span className="text-[10px] text-center truncate w-full">{p.filename}</span>
                </div>
              )}
              <button
                onClick={() => remove(p.id)}
                aria-label={`Delete ${p.filename}`}
                className="absolute top-1.5 right-1.5 p-1.5 rounded-md bg-black/60 text-ink-dim opacity-0 group-hover:opacity-100 hover:text-rose-400 transition-opacity"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

const STATUSES: RepairOrderStatus[] = [
  "created",
  "diagnosed",
  "estimate_sent",
  "approved",
  "in_progress",
  "completed",
  "invoiced",
  "closed",
];

const money = (n: number) => n.toLocaleString("en-US", { style: "currency", currency: "USD" });

export default function RepairOrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [ro, setRo] = useState<RepairOrder | null>(null);
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [vehicle, setVehicle] = useState<Vehicle | null>(null);
  const [estimate, setEstimate] = useState<{ estimate: Estimate; items: EstimateItem[] } | null>(null);
  const [labor, setLabor] = useState<LaborLog[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "notfound">("loading");
  const [saving, setSaving] = useState(false);
  const [editingEstimate, setEditingEstimate] = useState(false);

  const load = useCallback(() => {
    api
      .get<RepairOrder>(`/repair-orders/${id}`)
      .then((o) => {
        setRo(o);
        setStatus("ready");
        if (o.customer_id) api.get<Customer>(`/customers/${o.customer_id}`).then(setCustomer).catch(() => {});
        if (o.vehicle_id) api.get<Vehicle>(`/vehicles/${o.vehicle_id}`).then(setVehicle).catch(() => {});
      })
      .catch(() => setStatus("notfound"));
    api.get<{ estimate: Estimate; items: EstimateItem[] }>(`/estimates/ro/${id}`).then(setEstimate).catch(() => setEstimate(null));
    api.get<LaborLog[]>(`/labor/ro/${id}`).then(setLabor).catch(() => {});
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const changeStatus = async (next: string) => {
    setSaving(true);
    try {
      await api.patch(`/repair-orders/${id}`, { status: next });
      load();
    } finally {
      setSaving(false);
    }
  };

  if (status === "loading") return <div className="py-20 text-center text-sm text-ink-mute">Loading…</div>;

  if (status === "notfound" || !ro) {
    return (
      <div className="space-y-6">
        <Link href="/repair-orders" className="inline-flex items-center gap-1.5 text-sm text-ink-dim hover:text-amber transition-colors">
          <ArrowLeft className="w-4 h-4" /> Back to repair orders
        </Link>
        <Card>
          <div className="px-5 py-14 text-center">
            <ClipboardList className="w-8 h-8 text-ink-mute mx-auto mb-2" />
            <p className="text-sm text-ink-mute">Repair order not found</p>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <Link href="/repair-orders" className="inline-flex items-center gap-1.5 text-sm text-ink-dim hover:text-amber transition-colors">
        <ArrowLeft className="w-4 h-4" /> Back to repair orders
      </Link>

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <span className="h-9 w-1 rounded-full bg-amber shadow-[0_0_12px_-2px_var(--color-amber)]" />
          <div>
            <h1 className="text-xl font-bold tracking-tight text-ink">{ro.description || "Repair Order"}</h1>
            <div className="flex items-center gap-2 mt-1">
              <Badge status={ro.status} data-testid="ro-status">{ro.status.replace(/_/g, " ")}</Badge>
              {ro.mileage > 0 && <span className="text-xs text-ink-mute nums">{ro.mileage.toLocaleString()} mi</span>}
            </div>
          </div>
        </div>

        {/* status transition */}
        <div className="flex items-center gap-2">
          <label className="gf-label !mb-0">Status</label>
          <select
            aria-label="Change status"
            value={ro.status}
            disabled={saving}
            onChange={(e) => changeStatus(e.target.value)}
            className="gf-input !w-auto"
          >
            {STATUSES.map((s) => (
              <option key={s} value={s}>
                {s.replace(/_/g, " ")}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Card>
          <CardContent className="flex items-center gap-3">
            <User className="w-4 h-4 text-ink-mute" />
            {customer ? (
              <Link href={`/customers/${customer.id}`} className="text-sm font-medium text-ink hover:text-amber transition-colors">
                {customer.name}
              </Link>
            ) : (
              <span className="text-sm text-ink-mute">No customer</span>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-3">
            <Car className="w-4 h-4 text-ink-mute" />
            {vehicle ? (
              <Link href={`/vehicles/${vehicle.id}`} className="text-sm font-medium text-ink hover:text-amber transition-colors">
                {`${vehicle.year ? `${vehicle.year} ` : ""}${vehicle.make} ${vehicle.model}`.trim() || "Vehicle"}
              </Link>
            ) : (
              <span className="text-sm text-ink-mute">No vehicle</span>
            )}
          </CardContent>
        </Card>
      </div>

      {/* digital inspection */}
      <InspectionSection roId={id} />

      {/* estimate */}
      <Card>
        <CardHeader className="flex items-center justify-between">
          <h2 className="gf-eyebrow flex items-center gap-2">
            <FileText className="w-3.5 h-3.5 text-amber" /> Estimate
          </h2>
          <div className="flex items-center gap-2">{estimate && <Badge status={estimate.estimate.status}>{estimate.estimate.status}</Badge>}{estimate && estimate.estimate.status !== "paid" && <button onClick={() => setEditingEstimate(!editingEstimate)} className="gf-btn-secondary text-xs"><Pencil className="w-3.5 h-3.5" /> {editingEstimate ? "Close Editor" : "Edit"}</button>}</div>
        </CardHeader>
        {!estimate ? (
          <CardContent className="p-5"><p className="text-sm text-ink-mute mb-4">No estimate yet. Build it here while reviewing the repair order.</p><EstimateEditor repairOrderId={id} onSaved={load} /></CardContent>
        ) : editingEstimate ? (
          <CardContent className="p-5"><EstimateEditor repairOrderId={id} existing={estimate} onSaved={() => { setEditingEstimate(false); load(); }} /></CardContent>
        ) : (
          <>
            <table className="w-full">
              <thead>
                <tr>
                  <th className="gf-th">Item</th>
                  <th className="gf-th">Type</th>
                  <th className="gf-th text-right">Qty</th>
                  <th className="gf-th text-right">Unit</th>
                  <th className="gf-th text-right">Total</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {estimate.items.map((it) => (
                  <tr key={it.id}>
                    <td className="px-5 py-3 text-sm text-ink">{it.description}</td>
                    <td className="px-5 py-3 text-sm text-ink-dim capitalize">{it.type}</td>
                    <td className="px-5 py-3 text-sm text-ink-dim nums text-right">{it.quantity}</td>
                    <td className="px-5 py-3 text-sm text-ink-dim nums text-right">{money(it.unit_price)}</td>
                    <td className="px-5 py-3 text-sm text-ink nums text-right">{money(it.total)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div className="flex justify-end px-5 py-3.5 border-t border-line">
              <span className="text-sm text-ink-dim mr-3">Total</span>
              <span className="text-sm font-bold text-amber nums" data-testid="estimate-total">{money(estimate.estimate.total)}</span>
            </div>
          </>
        )}
      </Card>

      {/* labor */}
      <Card>
        <CardHeader>
          <h2 className="gf-eyebrow flex items-center gap-2">
            <Wrench className="w-3.5 h-3.5 text-amber" /> Labor
          </h2>
        </CardHeader>
        {labor.length === 0 ? (
          <div className="px-5 py-8 text-center text-sm text-ink-mute">No labor logged yet</div>
        ) : (
          <ul className="divide-y divide-line">
            {labor.map((l) => (
              <li key={l.id} className="flex items-center justify-between px-5 py-3.5">
                <div>
                  <p className="text-sm text-ink">{l.description || "Labor"}</p>
                  <p className="text-xs text-ink-mute nums">{new Date(l.clock_in).toLocaleString()}</p>
                </div>
                <span className="text-sm nums text-ink-dim">
                  {l.clock_out ? `${l.minutes} min` : <span className="text-emerald-400">clocked in</span>}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>

      {/* photos */}
      <PhotosCard roId={id} />
    </div>
  );
}
