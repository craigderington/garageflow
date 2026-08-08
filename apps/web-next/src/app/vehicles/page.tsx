"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Vehicle, Customer } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { StatCard } from "@/components/ui/StatCard";
import { Plus, Car, Pencil, Archive, ArchiveRestore, Users, CalendarDays } from "lucide-react";

export default function VehiclesPage() {
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [showArchived, setShowArchived] = useState(false);
  const [editing, setEditing] = useState<Vehicle | null>(null);
  const [form, setForm] = useState({ customer_id: "", vin: "", make: "", model: "", year: 0, color: "", license_plate: "" });

  const load = useCallback(() => {
    api.get<Vehicle[]>("/vehicles?include_archived=true").then(setVehicles).catch(() => {});
    api.get<Customer[]>("/customers").then(setCustomers).catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    const created = await api.post<Vehicle>("/vehicles", form);
    setVehicles((current) => [created, ...current.filter((vehicle) => vehicle.id !== created.id)]);
    setForm({ customer_id: "", vin: "", make: "", model: "", year: 0, color: "", license_plate: "" });
    setShowForm(false);
    load();
  };

  const customerMap = new Map(customers.map((c) => [c.id, c.name]));
  const active = vehicles.filter((v) => !v.archived);
  const displayedVehicles = showArchived ? vehicles : active;
  const currentYear = new Date().getFullYear();
  const recent = active.filter((v) => v.year >= currentYear - 5).length;
  const owners = new Set(active.map((v) => v.customer_id)).size;

  const saveEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editing) return;
    await api.patch(`/vehicles/${editing.id}`, editing);
    setEditing(null);
    load();
  };

  const archiveVehicle = async (vehicle: Vehicle) => {
    await api.post(`/vehicles/${vehicle.id}/archive`, { archived: !vehicle.archived });
    load();
  };

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

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Active Vehicles" value={active.length} icon={<Car className="w-5 h-5" />} color="blue" />
        <StatCard label="Vehicle Owners" value={owners} icon={<Users className="w-5 h-5" />} color="amber" />
        <StatCard label="Last 5 Model Years" value={recent} icon={<CalendarDays className="w-5 h-5" />} color="emerald" />
        <StatCard label="Archived" value={vehicles.filter((v) => v.archived).length} icon={<Archive className="w-5 h-5" />} color="rose" />
      </div>

      <label className="inline-flex items-center gap-2 text-sm text-ink-dim cursor-pointer">
        <input type="checkbox" checked={showArchived} onChange={(e) => setShowArchived(e.target.checked)} /> Show archived vehicles
      </label>

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

      {editing && (
        <Card className="border-amber/30">
          <CardContent className="p-5">
            <form onSubmit={saveEdit} className="space-y-4">
              <h2 className="text-sm font-bold text-ink">Edit {editing.year} {editing.make} {editing.model}</h2>
              <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
                <input value={editing.make} onChange={(e) => setEditing({ ...editing, make: e.target.value })} placeholder="Make" className="gf-input" />
                <input value={editing.model} onChange={(e) => setEditing({ ...editing, model: e.target.value })} placeholder="Model" className="gf-input" />
                <input type="number" value={editing.year || ""} onChange={(e) => setEditing({ ...editing, year: Number(e.target.value) })} placeholder="Year" className="gf-input" />
                <input value={editing.color} onChange={(e) => setEditing({ ...editing, color: e.target.value })} placeholder="Color" className="gf-input" />
                <input value={editing.vin} onChange={(e) => setEditing({ ...editing, vin: e.target.value })} placeholder="VIN" className="gf-input sm:col-span-2" />
                <input value={editing.license_plate} onChange={(e) => setEditing({ ...editing, license_plate: e.target.value })} placeholder="Plate" className="gf-input" />
                <select value={editing.customer_id} onChange={(e) => setEditing({ ...editing, customer_id: e.target.value })} className="gf-input">
                  {customers.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
                </select>
              </div>
              <div className="flex gap-2"><button className="gf-btn-primary">Save Vehicle</button><button type="button" onClick={() => setEditing(null)} className="gf-btn-ghost">Cancel</button></div>
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
                <th className="gf-th">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {displayedVehicles.map((v) => (
                <tr key={v.id} className={`hover:bg-surface-2 transition-colors ${v.archived ? "opacity-55" : ""}`}>
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
                  <td className="px-5 py-3.5"><div className="flex gap-1">
                    <button onClick={() => setEditing(v)} aria-label={`Edit ${v.make} ${v.model}`} className="p-1.5 text-ink-mute hover:text-amber"><Pencil className="w-4 h-4" /></button>
                    <button onClick={() => archiveVehicle(v)} aria-label={`${v.archived ? "Restore" : "Archive"} ${v.make} ${v.model}`} className="p-1.5 text-ink-mute hover:text-rose-400">{v.archived ? <ArchiveRestore className="w-4 h-4" /> : <Archive className="w-4 h-4" />}</button>
                  </div></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {displayedVehicles.length === 0 && (
          <div className="px-5 py-14 text-center">
            <Car className="w-8 h-8 text-ink-mute mx-auto mb-2" />
            <p className="text-sm text-ink-mute">No vehicles registered</p>
          </div>
        )}
      </Card>
    </div>
  );
}
