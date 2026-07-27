"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Vehicle, Customer } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Plus, Car } from "lucide-react";

export default function VehiclesPage() {
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ customer_id: "", vin: "", make: "", model: "", year: 0, color: "", license_plate: "" });

  const load = useCallback(() => {
    api.get<Vehicle[]>("/vehicles").then(setVehicles).catch(() => {});
    api.get<Customer[]>("/customers").then(setCustomers).catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/vehicles", form);
    setForm({ customer_id: "", vin: "", make: "", model: "", year: 0, color: "", license_plate: "" });
    setShowForm(false);
    load();
  };

  const customerMap = new Map(customers.map((c) => [c.id, c.name]));

  return (
    <div className="space-y-6">
      <PageHeader
        title="Vehicles"
        subtitle={`${vehicles.length} registered vehicles`}
        action={
          <button onClick={() => setShowForm(!showForm)} className="gf-btn-primary">
            <Plus className="w-4 h-4" /> Add Vehicle
          </button>
        }
      />

      {showForm && (
        <Card>
          <CardContent className="p-5">
            <form onSubmit={create} className="space-y-4">
              <select value={form.customer_id} onChange={(e) => setForm({ ...form, customer_id: e.target.value })} required className="gf-input">
                <option value="">Select Customer</option>
                {customers.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <input value={form.vin} onChange={(e) => setForm({ ...form, vin: e.target.value })} placeholder="VIN" className="gf-input" />
                <input value={form.license_plate} onChange={(e) => setForm({ ...form, license_plate: e.target.value })} placeholder="License Plate" className="gf-input" />
                <input type="number" value={form.year || ""} onChange={(e) => setForm({ ...form, year: Number(e.target.value) })} placeholder="Year" className="gf-input" />
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <input value={form.make} onChange={(e) => setForm({ ...form, make: e.target.value })} placeholder="Make" className="gf-input" />
                <input value={form.model} onChange={(e) => setForm({ ...form, model: e.target.value })} placeholder="Model" className="gf-input" />
                <input value={form.color} onChange={(e) => setForm({ ...form, color: e.target.value })} placeholder="Color" className="gf-input" />
              </div>
              <div className="flex gap-2">
                <button type="submit" className="gf-btn-primary">
                  Create
                </button>
                <button type="button" onClick={() => setShowForm(false)} className="gf-btn-ghost">
                  Cancel
                </button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr>
                <th className="gf-th">Owner</th>
                <th className="gf-th">Make</th>
                <th className="gf-th">Model</th>
                <th className="gf-th">Year</th>
                <th className="gf-th">VIN</th>
                <th className="gf-th">Plate</th>
                <th className="gf-th">Color</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {vehicles.map((v) => (
                <tr key={v.id} className="hover:bg-surface-2 transition-colors">
                  <td className="px-5 py-3.5 text-sm font-medium text-ink">{customerMap.get(v.customer_id) || "—"}</td>
                  <td className="px-5 py-3.5 text-sm font-medium">
                    <Link href={`/vehicles/${v.id}`} className="text-ink hover:text-amber transition-colors">
                      {v.make || "View"}
                    </Link>
                  </td>
                  <td className="px-5 py-3.5 text-sm text-ink-dim">{v.model || "—"}</td>
                  <td className="px-5 py-3.5 text-sm text-ink-dim nums">{v.year || "—"}</td>
                  <td className="px-5 py-3.5 text-sm text-ink-dim nums">{v.vin || "—"}</td>
                  <td className="px-5 py-3.5 text-sm text-ink-dim nums">{v.license_plate || "—"}</td>
                  <td className="px-5 py-3.5 text-sm text-ink-dim">{v.color || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {vehicles.length === 0 && (
          <div className="px-5 py-14 text-center">
            <Car className="w-8 h-8 text-ink-mute mx-auto mb-2" />
            <p className="text-sm text-ink-mute">No vehicles registered</p>
          </div>
        )}
      </Card>
    </div>
  );
}
