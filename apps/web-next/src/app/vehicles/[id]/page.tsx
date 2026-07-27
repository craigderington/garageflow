"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Vehicle, Customer, RepairOrder } from "@/lib/types";
import { Card, CardContent, CardHeader } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { ArrowLeft, Car, User, ClipboardList, Pencil, Trash2, X } from "lucide-react";

function Spec({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="gf-eyebrow">{label}</p>
      <p className="text-sm text-ink mt-1 nums">{value || "—"}</p>
    </div>
  );
}

export default function VehicleDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [vehicle, setVehicle] = useState<Vehicle | null>(null);
  const [owner, setOwner] = useState<Customer | null>(null);
  const [orders, setOrders] = useState<RepairOrder[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "notfound">("loading");
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({ vin: "", make: "", model: "", year: 0, color: "", license_plate: "" });
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    api
      .get<Vehicle>(`/vehicles/${id}`)
      .then((v) => {
        setVehicle(v);
        setStatus("ready");
        if (v.customer_id) api.get<Customer>(`/customers/${v.customer_id}`).then(setOwner).catch(() => {});
      })
      .catch(() => setStatus("notfound"));
    api.get<RepairOrder[]>("/repair-orders").then((o) => setOrders(o.filter((x) => x.vehicle_id === id))).catch(() => {});
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const startEdit = () => {
    if (!vehicle) return;
    setForm({
      vin: vehicle.vin,
      make: vehicle.make,
      model: vehicle.model,
      year: vehicle.year,
      color: vehicle.color,
      license_plate: vehicle.license_plate,
    });
    setEditing(true);
  };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.patch(`/vehicles/${id}`, form);
      setEditing(false);
      load();
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!confirm("Delete this vehicle?")) return;
    setBusy(true);
    try {
      await api.del(`/vehicles/${id}`);
      router.push("/vehicles");
    } finally {
      setBusy(false);
    }
  };

  if (status === "loading") return <div className="py-20 text-center text-sm text-ink-mute">Loading…</div>;

  if (status === "notfound" || !vehicle) {
    return (
      <div className="space-y-6">
        <Link href="/vehicles" className="inline-flex items-center gap-1.5 text-sm text-ink-dim hover:text-amber transition-colors">
          <ArrowLeft className="w-4 h-4" /> Back to vehicles
        </Link>
        <Card>
          <div className="px-5 py-14 text-center">
            <Car className="w-8 h-8 text-ink-mute mx-auto mb-2" />
            <p className="text-sm text-ink-mute">Vehicle not found</p>
          </div>
        </Card>
      </div>
    );
  }

  const title = `${vehicle.year ? `${vehicle.year} ` : ""}${vehicle.make} ${vehicle.model}`.trim();

  return (
    <div className="space-y-6">
      <Link href="/vehicles" className="inline-flex items-center gap-1.5 text-sm text-ink-dim hover:text-amber transition-colors">
        <ArrowLeft className="w-4 h-4" /> Back to vehicles
      </Link>

      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <span className="h-9 w-1 rounded-full bg-amber shadow-[0_0_12px_-2px_var(--color-amber)]" />
          <div>
            <h1 className="text-xl font-bold tracking-tight text-ink" data-testid="vehicle-title">{title || "Vehicle"}</h1>
            {owner ? (
              <Link href={`/customers/${owner.id}`} className="text-sm text-ink-dim mt-0.5 inline-flex items-center gap-1.5 hover:text-amber transition-colors">
                <User className="w-3.5 h-3.5" /> {owner.name}
              </Link>
            ) : (
              <p className="text-sm text-ink-mute mt-0.5">No owner on file</p>
            )}
          </div>
        </div>
        {!editing && (
          <div className="flex items-center gap-2 shrink-0">
            <button onClick={startEdit} className="gf-btn-secondary text-sm">
              <Pencil className="w-3.5 h-3.5" /> Edit
            </button>
            <button onClick={remove} disabled={busy} className="gf-btn-ghost text-sm text-rose-400 hover:text-rose-300 hover:bg-rose-500/10">
              <Trash2 className="w-3.5 h-3.5" /> Delete
            </button>
          </div>
        )}
      </div>

      {editing && (
        <Card>
          <div className="flex items-center justify-between px-5 py-4 border-b border-line">
            <h2 className="text-sm font-bold text-ink">Edit vehicle</h2>
            <button onClick={() => setEditing(false)} className="text-ink-mute hover:text-ink">
              <X className="w-4 h-4" />
            </button>
          </div>
          <form onSubmit={save} className="p-5 space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <input value={form.make} onChange={(e) => setForm({ ...form, make: e.target.value })} placeholder="Make" required className="gf-input" />
              <input value={form.model} onChange={(e) => setForm({ ...form, model: e.target.value })} placeholder="Model" required className="gf-input" />
              <input type="number" value={form.year || ""} onChange={(e) => setForm({ ...form, year: Number(e.target.value) })} placeholder="Year" className="gf-input" />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <input value={form.license_plate} onChange={(e) => setForm({ ...form, license_plate: e.target.value })} placeholder="License plate" className="gf-input" />
              <input value={form.color} onChange={(e) => setForm({ ...form, color: e.target.value })} placeholder="Color" className="gf-input" />
              <input value={form.vin} onChange={(e) => setForm({ ...form, vin: e.target.value })} placeholder="VIN" className="gf-input" />
            </div>
            <div className="flex gap-2">
              <button type="submit" disabled={busy} className="gf-btn-primary">Save changes</button>
              <button type="button" onClick={() => setEditing(false)} className="gf-btn-ghost">Cancel</button>
            </div>
          </form>
        </Card>
      )}

      <Card>
        <CardHeader>
          <h2 className="gf-eyebrow">Specifications</h2>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-5">
            <Spec label="Year" value={vehicle.year ? String(vehicle.year) : ""} />
            <Spec label="Make" value={vehicle.make} />
            <Spec label="Model" value={vehicle.model} />
            <Spec label="VIN" value={vehicle.vin} />
            <Spec label="License Plate" value={vehicle.license_plate} />
            <Spec label="Color" value={vehicle.color} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <h2 className="gf-eyebrow flex items-center gap-2">
            <ClipboardList className="w-3.5 h-3.5 text-amber" /> Service History
          </h2>
        </CardHeader>
        {orders.length === 0 ? (
          <div className="px-5 py-8 text-center text-sm text-ink-mute">No repair orders for this vehicle</div>
        ) : (
          <ul className="divide-y divide-line">
            {orders.map((o) => (
              <li key={o.id}>
                <Link
                  href={`/repair-orders/${o.id}`}
                  className="flex items-center justify-between gap-3 px-5 py-3.5 hover:bg-surface-2 transition-colors"
                >
                  <span className="text-sm font-medium text-ink truncate">{o.description || "—"}</span>
                  <span className="flex items-center gap-3 shrink-0">
                    <span className="text-xs text-ink-mute nums">{o.mileage ? `${o.mileage.toLocaleString()} mi` : ""}</span>
                    <Badge status={o.status}>{o.status.replace(/_/g, " ")}</Badge>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
