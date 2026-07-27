"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { api, ApiError } from "@/lib/api";
import type { Customer, Vehicle, RepairOrder } from "@/lib/types";
import { Card, CardContent, CardHeader } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { ArrowLeft, Mail, Phone, Car, ClipboardList, User, Pencil, Trash2, X } from "lucide-react";

export default function CustomerDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const router = useRouter();
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [orders, setOrders] = useState<RepairOrder[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "notfound">("loading");
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({ name: "", phone: "", email: "", notes: "" });
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    api
      .get<Customer>(`/customers/${id}`)
      .then((c) => {
        setCustomer(c);
        setStatus("ready");
      })
      .catch((err) => {
        setStatus(err instanceof ApiError && err.status === 404 ? "notfound" : "notfound");
      });
    api.get<Vehicle[]>("/vehicles").then((v) => setVehicles(v.filter((x) => x.customer_id === id))).catch(() => {});
    api.get<RepairOrder[]>("/repair-orders").then((o) => setOrders(o.filter((x) => x.customer_id === id))).catch(() => {});
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const startEdit = () => {
    if (!customer) return;
    setForm({ name: customer.name, phone: customer.phone, email: customer.email, notes: customer.notes });
    setEditing(true);
  };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.patch(`/customers/${id}`, form);
      setEditing(false);
      load();
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!confirm("Delete this customer? This also removes their vehicles and repair orders.")) return;
    setBusy(true);
    try {
      await api.del(`/customers/${id}`);
      router.push("/customers");
    } finally {
      setBusy(false);
    }
  };

  if (status === "loading") {
    return <div className="py-20 text-center text-sm text-ink-mute">Loading…</div>;
  }

  if (status === "notfound" || !customer) {
    return (
      <div className="space-y-6">
        <Link href="/customers" className="inline-flex items-center gap-1.5 text-sm text-ink-dim hover:text-amber transition-colors">
          <ArrowLeft className="w-4 h-4" /> Back to customers
        </Link>
        <Card>
          <div className="px-5 py-14 text-center">
            <User className="w-8 h-8 text-ink-mute mx-auto mb-2" />
            <p className="text-sm text-ink-mute">Customer not found</p>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <Link href="/customers" className="inline-flex items-center gap-1.5 text-sm text-ink-dim hover:text-amber transition-colors">
        <ArrowLeft className="w-4 h-4" /> Back to customers
      </Link>

      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <span className="h-9 w-1 rounded-full bg-amber shadow-[0_0_12px_-2px_var(--color-amber)]" />
          <div>
            <h1 className="text-xl font-bold tracking-tight text-ink" data-testid="customer-name">{customer.name}</h1>
            <p className="text-sm text-ink-dim mt-0.5">
              {vehicles.length} vehicle{vehicles.length === 1 ? "" : "s"} · {orders.length} repair order
              {orders.length === 1 ? "" : "s"}
            </p>
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
            <h2 className="text-sm font-bold text-ink">Edit customer</h2>
            <button onClick={() => setEditing(false)} className="text-ink-mute hover:text-ink">
              <X className="w-4 h-4" />
            </button>
          </div>
          <form onSubmit={save} className="p-5 space-y-4">
            <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="Name" required className="gf-input" />
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <input value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} placeholder="Phone" className="gf-input" />
              <input value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} placeholder="Email" className="gf-input" />
            </div>
            <textarea value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} placeholder="Notes" rows={2} className="gf-input" />
            <div className="flex gap-2">
              <button type="submit" disabled={busy} className="gf-btn-primary">Save changes</button>
              <button type="button" onClick={() => setEditing(false)} className="gf-btn-ghost">Cancel</button>
            </div>
          </form>
        </Card>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* contact */}
        <Card className="lg:col-span-1">
          <CardHeader>
            <h2 className="gf-eyebrow">Contact</h2>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex items-center gap-2.5 text-ink-dim">
              <Phone className="w-4 h-4 text-ink-mute" />
              <span className="nums">{customer.phone || "—"}</span>
            </div>
            <div className="flex items-center gap-2.5 text-ink-dim">
              <Mail className="w-4 h-4 text-ink-mute" />
              <span>{customer.email || "—"}</span>
            </div>
            {customer.notes && (
              <div className="pt-3 border-t border-line text-ink-mute leading-relaxed">{customer.notes}</div>
            )}
          </CardContent>
        </Card>

        <div className="lg:col-span-2 space-y-6">
          {/* vehicles */}
          <Card>
            <CardHeader className="flex items-center justify-between">
              <h2 className="gf-eyebrow flex items-center gap-2">
                <Car className="w-3.5 h-3.5 text-amber" /> Vehicles
              </h2>
            </CardHeader>
            {vehicles.length === 0 ? (
              <div className="px-5 py-8 text-center text-sm text-ink-mute">No vehicles on file</div>
            ) : (
              <ul className="divide-y divide-line">
                {vehicles.map((v) => (
                  <li key={v.id}>
                    <Link
                      href={`/vehicles/${v.id}`}
                      className="flex items-center justify-between px-5 py-3.5 hover:bg-surface-2 transition-colors"
                    >
                      <span className="text-sm font-medium text-ink">
                        {v.year ? `${v.year} ` : ""}
                        {v.make} {v.model}
                      </span>
                      <span className="text-xs text-ink-mute nums">{v.license_plate || v.vin || "—"}</span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Card>

          {/* repair orders */}
          <Card>
            <CardHeader>
              <h2 className="gf-eyebrow flex items-center gap-2">
                <ClipboardList className="w-3.5 h-3.5 text-amber" /> Repair Orders
              </h2>
            </CardHeader>
            {orders.length === 0 ? (
              <div className="px-5 py-8 text-center text-sm text-ink-mute">No repair orders yet</div>
            ) : (
              <ul className="divide-y divide-line">
                {orders.map((o) => (
                  <li key={o.id}>
                    <Link
                      href={`/repair-orders/${o.id}`}
                      className="flex items-center justify-between gap-3 px-5 py-3.5 hover:bg-surface-2 transition-colors"
                    >
                      <span className="text-sm font-medium text-ink truncate">{o.description || "—"}</span>
                      <Badge status={o.status}>{o.status.replace(/_/g, " ")}</Badge>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}
