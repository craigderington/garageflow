"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { RepairOrder, Customer, Vehicle } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { PageHeader } from "@/components/ui/PageHeader";
import { StatCard } from "@/components/ui/StatCard";
import { InlineLabor } from "@/components/InlineLabor";
import { Plus, ClipboardList, Car, X, Clock, Wrench, CheckCircle2 } from "lucide-react";

export default function RepairOrdersPage() {
  const [orders, setOrders] = useState<RepairOrder[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [description, setDescription] = useState("");
  const [customerId, setCustomerId] = useState("");
  const [vehicleId, setVehicleId] = useState("");
  const [mileage, setMileage] = useState(0);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);

  // add-vehicle modal state
  const [showAddVehicle, setShowAddVehicle] = useState(false);
  const [newVehicle, setNewVehicle] = useState({ make: "", model: "", year: 0, license_plate: "", vin: "" });

  const load = useCallback(() => {
    api.get<RepairOrder[]>("/repair-orders").then(setOrders).catch(() => {});
    api.get<Customer[]>("/customers").then(setCustomers).catch(() => {});
    api.get<Vehicle[]>("/vehicles").then(setVehicles).catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/repair-orders", { customer_id: customerId, vehicle_id: vehicleId, description, mileage });
    setShowForm(false);
    setDescription("");
    load();
  };

  const addVehicle = async (e: React.FormEvent) => {
    e.preventDefault();
    const v = await api.post<Vehicle>("/vehicles", { ...newVehicle, customer_id: customerId });
    setVehicles((prev) => [...prev, v]);
    setVehicleId(v.id);
    setShowAddVehicle(false);
    setNewVehicle({ make: "", model: "", year: 0, license_plate: "", vin: "" });
  };

  const customerMap = new Map(customers.map((c) => [c.id, c.name]));
  const awaiting = orders.filter((ro) => ["created", "diagnosed", "estimate_sent"].includes(ro.status)).length;
  const active = orders.filter((ro) => ["approved", "in_progress"].includes(ro.status)).length;
  const completed = orders.filter((ro) => ["completed", "invoiced", "closed"].includes(ro.status)).length;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Repair Orders"
        subtitle={`${orders.length} total orders`}
        action={
          <button onClick={() => setShowForm(!showForm)} className="gf-btn-primary">
            <Plus className="w-4 h-4" /> New RO
          </button>
        }
      />

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Total Repair Orders" value={orders.length} icon={<ClipboardList className="w-5 h-5" />} color="blue" />
        <StatCard label="Awaiting Service" value={awaiting} icon={<Clock className="w-5 h-5" />} color="amber" />
        <StatCard label="In Service" value={active} icon={<Wrench className="w-5 h-5" />} color="blue" />
        <StatCard label="Completed / Closed" value={completed} icon={<CheckCircle2 className="w-5 h-5" />} color="emerald" />
      </div>

      {showForm && (
        <Card>
          <CardContent className="p-5">
            <form onSubmit={create} className="space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <select
                  value={customerId}
                  onChange={(e) => {
                    setCustomerId(e.target.value);
                    setVehicleId("");
                  }}
                  className="gf-input"
                >
                  <option value="">Select Customer</option>
                  {customers.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
                <div className="flex gap-2">
                  <select value={vehicleId} onChange={(e) => setVehicleId(e.target.value)} className="gf-input flex-1">
                    <option value="">Select Vehicle</option>
                    {vehicles
                      .filter((v) => !customerId || v.customer_id === customerId)
                      .map((v) => (
                        <option key={v.id} value={v.id}>
                          {v.make} {v.model} ({v.license_plate})
                        </option>
                      ))}
                  </select>
                  {customerId && (
                    <button
                      type="button"
                      onClick={() => setShowAddVehicle(true)}
                      className="gf-btn px-3 py-2.5 border border-dashed border-amber/40 text-amber hover:bg-amber/10 whitespace-nowrap"
                    >
                      <Car className="w-3.5 h-3.5" /> Add
                    </button>
                  )}
                </div>
              </div>
              <input
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Description of work needed"
                className="gf-input"
              />
              <input
                type="number"
                value={mileage || ""}
                onChange={(e) => setMileage(Number(e.target.value))}
                placeholder="Mileage"
                className="gf-input"
              />
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

      {/* Add Vehicle Modal */}
      {showAddVehicle && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="gf-card w-full max-w-md">
            <div className="flex items-center justify-between px-5 py-4 border-b border-line">
              <h2 className="text-sm font-bold text-ink flex items-center gap-2">
                <Car className="w-4 h-4 text-amber" />
                Add Vehicle for {customers.find((c) => c.id === customerId)?.name}
              </h2>
              <button onClick={() => setShowAddVehicle(false)} className="text-ink-mute hover:text-ink transition-colors">
                <X className="w-4 h-4" />
              </button>
            </div>
            <form onSubmit={addVehicle} className="p-5 space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <input
                  value={newVehicle.make}
                  onChange={(e) => setNewVehicle({ ...newVehicle, make: e.target.value })}
                  placeholder="Make"
                  required
                  className="gf-input"
                />
                <input
                  value={newVehicle.model}
                  onChange={(e) => setNewVehicle({ ...newVehicle, model: e.target.value })}
                  placeholder="Model"
                  required
                  className="gf-input"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <input
                  type="number"
                  value={newVehicle.year || ""}
                  onChange={(e) => setNewVehicle({ ...newVehicle, year: Number(e.target.value) })}
                  placeholder="Year"
                  className="gf-input"
                />
                <input
                  value={newVehicle.license_plate}
                  onChange={(e) => setNewVehicle({ ...newVehicle, license_plate: e.target.value })}
                  placeholder="License plate"
                  className="gf-input"
                />
              </div>
              <input
                value={newVehicle.vin}
                onChange={(e) => setNewVehicle({ ...newVehicle, vin: e.target.value })}
                placeholder="VIN"
                className="gf-input"
              />
              <div className="flex gap-2 pt-1">
                <button type="submit" className="gf-btn-primary flex-1">
                  Add Vehicle
                </button>
                <button type="button" onClick={() => setShowAddVehicle(false)} className="gf-btn-ghost">
                  Cancel
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr>
                <th className="gf-th">Status</th>
                <th className="gf-th">Description</th>
                <th className="gf-th">Customer</th>
                <th className="gf-th">Mileage</th>
                <th className="gf-th">Updated</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {orders.map((ro) => (
                <tr key={ro.id} className="hover:bg-surface-2 transition-colors">
                  <td className="px-5 py-3.5">
                    <Badge status={ro.status}>{ro.status.replace(/_/g, " ")}</Badge>
                  </td>
                  <td className="px-5 py-3.5 text-sm font-medium">
                    <Link href={`/repair-orders/${ro.id}`} className="text-ink hover:text-amber transition-colors">
                      {ro.description || "View order"}
                    </Link>
                  </td>
                  <td className="px-5 py-3.5 text-sm">
                    {ro.customer_id ? (
                      <Link
                        href={`/customers/${ro.customer_id}`}
                        className="font-bold text-amber hover:underline hover:text-amber-bright transition-colors"
                      >
                        {customerMap.get(ro.customer_id) || "View Customer"}
                      </Link>
                    ) : (
                      <span className="text-ink-mute">—</span>
                    )}
                  </td>
                  <td className="px-5 py-3.5 text-sm text-ink-dim nums">{ro.mileage.toLocaleString()}</td>
                  <td className="px-5 py-3.5 text-sm text-ink-mute nums">{new Date(ro.updated_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {orders.length === 0 && (
          <div className="px-5 py-14 text-center">
            <ClipboardList className="w-8 h-8 text-ink-mute mx-auto mb-2" />
            <p className="text-sm text-ink-mute">No repair orders yet</p>
          </div>
        )}
      </Card>

      <InlineLabor orders={orders.filter((ro) => !["closed", "invoiced"].includes(ro.status))} />
    </div>
  );
}
