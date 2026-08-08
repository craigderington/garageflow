"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { api, ApiError } from "@/lib/api";
import type { Customer, Vehicle, RepairOrder } from "@/lib/types";
import { Card, CardContent, CardHeader } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import {
  ArrowLeft,
  Mail,
  Phone,
  Car,
  ClipboardList,
  User,
  Pencil,
  Trash2,
  X,
  Plus,
  ChevronRight,
} from "lucide-react";

export default function CustomerDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const router = useRouter();
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [orders, setOrders] = useState<RepairOrder[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "notfound">("loading");
  const [busy, setBusy] = useState(false);

  // Edit Customer State
  const [editing, setEditing] = useState(false);
  const [customerForm, setCustomerForm] = useState({ name: "", phone: "", email: "", notes: "" });

  // Add Vehicle State
  const [showAddVehicle, setShowAddVehicle] = useState(false);
  const [vehicleForm, setVehicleForm] = useState({
    make: "",
    model: "",
    year: new Date().getFullYear(),
    vin: "",
    license_plate: "",
    color: "",
  });

  // Edit Vehicle State
  const [editingVehicle, setEditingVehicle] = useState<Vehicle | null>(null);
  const [editVehicleForm, setEditVehicleForm] = useState({
    make: "",
    model: "",
    year: 0,
    vin: "",
    license_plate: "",
    color: "",
  });

  // Create Repair Order State
  const [showAddRO, setShowAddRO] = useState(false);
  const [roForm, setRoForm] = useState({ vehicle_id: "", description: "", mileage: 0 });

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
    api
      .get<Vehicle[]>("/vehicles")
      .then((v) => setVehicles(v.filter((x) => x.customer_id === id)))
      .catch(() => {});
    api
      .get<RepairOrder[]>("/repair-orders")
      .then((o) => setOrders(o.filter((x) => x.customer_id === id)))
      .catch(() => {});
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const startEdit = () => {
    if (!customer) return;
    setCustomerForm({ name: customer.name, phone: customer.phone, email: customer.email, notes: customer.notes });
    setEditing(true);
  };

  const saveCustomer = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.patch(`/customers/${id}`, customerForm);
      setEditing(false);
      load();
    } finally {
      setBusy(false);
    }
  };

  const removeCustomer = async () => {
    if (!confirm("Delete this customer? This also removes their vehicles and repair orders.")) return;
    setBusy(true);
    try {
      await api.del(`/customers/${id}`);
      router.push("/customers");
    } finally {
      setBusy(false);
    }
  };

  const createVehicle = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const created = await api.post<Vehicle>("/vehicles", {
        customer_id: id,
        ...vehicleForm,
        year: Number(vehicleForm.year) || 0,
      });
      setVehicles((current) => [created, ...current.filter((vehicle) => vehicle.id !== created.id)]);
      setShowAddVehicle(false);
      setVehicleForm({
        make: "",
        model: "",
        year: new Date().getFullYear(),
        vin: "",
        license_plate: "",
        color: "",
      });
      load();
    } finally {
      setBusy(false);
    }
  };

  const startEditVehicle = (v: Vehicle) => {
    setEditingVehicle(v);
    setEditVehicleForm({
      make: v.make,
      model: v.model,
      year: v.year,
      vin: v.vin,
      license_plate: v.license_plate,
      color: v.color,
    });
  };

  const saveVehicle = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingVehicle) return;
    setBusy(true);
    try {
      await api.patch(`/vehicles/${editingVehicle.id}`, {
        ...editVehicleForm,
        year: Number(editVehicleForm.year) || 0,
      });
      setEditingVehicle(null);
      load();
    } finally {
      setBusy(false);
    }
  };

  const deleteVehicle = async (vehicleId: string) => {
    if (!confirm("Delete this vehicle?")) return;
    await api.del(`/vehicles/${vehicleId}`);
    load();
  };

  const createRO = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const created = await api.post<RepairOrder>("/repair-orders", {
        customer_id: id,
        vehicle_id: roForm.vehicle_id,
        description: roForm.description,
        mileage: Number(roForm.mileage) || 0,
      });
      setShowAddRO(false);
      setRoForm({ vehicle_id: "", description: "", mileage: 0 });
      router.push(`/repair-orders/${created.id}`);
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
        <Link
          href="/customers"
          className="inline-flex items-center gap-1.5 text-sm text-ink-dim hover:text-amber transition-colors"
        >
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
      <Link
        href="/customers"
        className="inline-flex items-center gap-1.5 text-sm text-ink-dim hover:text-amber transition-colors"
      >
        <ArrowLeft className="w-4 h-4" /> Back to customers
      </Link>

      {/* Header Banner */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 bg-surface p-6 rounded-xl border border-line">
        <div className="flex items-center gap-4">
          <div className="w-14 h-14 rounded-full bg-amber/10 border border-amber/30 flex items-center justify-center text-amber text-xl font-bold">
            {customer.name.slice(0, 2).toUpperCase()}
          </div>
          <div>
            <h1
              className="text-2xl font-bold tracking-tight text-ink flex items-center gap-2"
              data-testid="customer-name"
            >
              {customer.name}
            </h1>
            <p className="text-sm text-ink-dim mt-0.5">
              {vehicles.length} vehicle{vehicles.length === 1 ? "" : "s"} · {orders.length} repair order
              {orders.length === 1 ? "" : "s"}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <button onClick={() => setShowAddVehicle(true)} className="gf-btn-secondary text-sm">
            <Car className="w-4 h-4 text-amber" /> Add Vehicle
          </button>
          <button onClick={() => setShowAddRO(true)} className="gf-btn-primary text-sm">
            <Plus className="w-4 h-4" /> New Repair Order
          </button>
          {!editing && (
            <>
              <button onClick={startEdit} className="gf-btn-secondary text-sm">
                <Pencil className="w-3.5 h-3.5" /> Edit
              </button>
              <button
                onClick={removeCustomer}
                disabled={busy}
                className="gf-btn-ghost text-sm text-rose-400 hover:text-rose-300 hover:bg-rose-500/10"
              >
                <Trash2 className="w-3.5 h-3.5" /> Delete
              </button>
            </>
          )}
        </div>
      </div>

      {/* Edit Customer Form */}
      {editing && (
        <Card className="border-amber/30">
          <div className="flex items-center justify-between px-5 py-4 border-b border-line">
            <h2 className="text-sm font-bold text-ink">Edit Customer Information</h2>
            <button onClick={() => setEditing(false)} className="text-ink-mute hover:text-ink">
              <X className="w-4 h-4" />
            </button>
          </div>
          <form onSubmit={saveCustomer} className="p-5 space-y-4">
            <input
              value={customerForm.name}
              onChange={(e) => setCustomerForm({ ...customerForm, name: e.target.value })}
              placeholder="Name"
              required
              className="gf-input"
            />
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <input
                value={customerForm.phone}
                onChange={(e) => setCustomerForm({ ...customerForm, phone: e.target.value })}
                placeholder="Phone"
                className="gf-input"
              />
              <input
                value={customerForm.email}
                onChange={(e) => setCustomerForm({ ...customerForm, email: e.target.value })}
                placeholder="Email"
                className="gf-input"
              />
            </div>
            <textarea
              value={customerForm.notes}
              onChange={(e) => setCustomerForm({ ...customerForm, notes: e.target.value })}
              placeholder="Notes"
              rows={2}
              className="gf-input"
            />
            <div className="flex gap-2 justify-end">
              <button type="button" onClick={() => setEditing(false)} className="gf-btn-secondary">
                Cancel
              </button>
              <button type="submit" disabled={busy} className="gf-btn-primary">
                Save Changes
              </button>
            </div>
          </form>
        </Card>
      )}

      {/* Add Vehicle Modal/Form */}
      {showAddVehicle && (
        <Card className="border-amber/30 bg-surface-2/60">
          <div className="flex items-center justify-between px-5 py-4 border-b border-line">
            <h2 className="text-sm font-bold text-ink flex items-center gap-2">
              <Car className="w-4 h-4 text-amber" /> Add Vehicle for {customer.name}
            </h2>
            <button onClick={() => setShowAddVehicle(false)} className="text-ink-mute hover:text-ink">
              <X className="w-4 h-4" />
            </button>
          </div>
          <form onSubmit={createVehicle} className="p-5 space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <input
                value={vehicleForm.year}
                onChange={(e) => setVehicleForm({ ...vehicleForm, year: Number(e.target.value) || 0 })}
                placeholder="Year (e.g. 2022)"
                type="number"
                required
                className="gf-input"
              />
              <input
                value={vehicleForm.make}
                onChange={(e) => setVehicleForm({ ...vehicleForm, make: e.target.value })}
                placeholder="Make (e.g. Ford)"
                required
                className="gf-input"
              />
              <input
                value={vehicleForm.model}
                onChange={(e) => setVehicleForm({ ...vehicleForm, model: e.target.value })}
                placeholder="Model (e.g. F-150)"
                required
                className="gf-input"
              />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <input
                value={vehicleForm.license_plate}
                onChange={(e) => setVehicleForm({ ...vehicleForm, license_plate: e.target.value })}
                placeholder="License Plate"
                className="gf-input"
              />
              <input
                value={vehicleForm.vin}
                onChange={(e) => setVehicleForm({ ...vehicleForm, vin: e.target.value })}
                placeholder="VIN"
                className="gf-input"
              />
              <input
                value={vehicleForm.color}
                onChange={(e) => setVehicleForm({ ...vehicleForm, color: e.target.value })}
                placeholder="Color"
                className="gf-input"
              />
            </div>
            <div className="flex gap-2 justify-end pt-2">
              <button type="button" onClick={() => setShowAddVehicle(false)} className="gf-btn-secondary">
                Cancel
              </button>
              <button type="submit" disabled={busy} className="gf-btn-primary">
                Add Vehicle
              </button>
            </div>
          </form>
        </Card>
      )}

      {/* New Repair Order Modal/Form */}
      {showAddRO && (
        <Card className="border-amber/30 bg-surface-2/60">
          <div className="flex items-center justify-between px-5 py-4 border-b border-line">
            <h2 className="text-sm font-bold text-ink flex items-center gap-2">
              <ClipboardList className="w-4 h-4 text-amber" /> Create Repair Order for {customer.name}
            </h2>
            <button onClick={() => setShowAddRO(false)} className="text-ink-mute hover:text-ink">
              <X className="w-4 h-4" />
            </button>
          </div>
          <form onSubmit={createRO} className="p-5 space-y-4">
            <div>
              <label className="text-xs font-medium text-ink-dim mb-1 block">Vehicle</label>
              <select
                value={roForm.vehicle_id}
                onChange={(e) => setRoForm({ ...roForm, vehicle_id: e.target.value })}
                className="gf-input w-full"
              >
                <option value="">Select Customer Vehicle (Optional)</option>
                {vehicles.map((v) => (
                  <option key={v.id} value={v.id}>
                    {v.year} {v.make} {v.model} {v.license_plate ? `(${v.license_plate})` : ""}
                  </option>
                ))}
              </select>
            </div>

            <textarea
              value={roForm.description}
              onChange={(e) => setRoForm({ ...roForm, description: e.target.value })}
              placeholder="Repair Order Description / Customer Complaints"
              rows={3}
              required
              className="gf-input w-full"
            />

            <input
              type="number"
              value={roForm.mileage || ""}
              onChange={(e) => setRoForm({ ...roForm, mileage: Number(e.target.value) || 0 })}
              placeholder="Current Mileage"
              className="gf-input w-full"
            />

            <div className="flex gap-2 justify-end pt-2">
              <button type="button" onClick={() => setShowAddRO(false)} className="gf-btn-secondary">
                Cancel
              </button>
              <button type="submit" disabled={busy} className="gf-btn-primary">
                Create Repair Order
              </button>
            </div>
          </form>
        </Card>
      )}

      {/* Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Contact Sidebar */}
        <Card className="lg:col-span-1">
          <CardHeader>
            <h2 className="gf-eyebrow">Contact Details</h2>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div className="flex items-center gap-3 text-ink">
              <div className="w-8 h-8 rounded-lg bg-surface-2 flex items-center justify-center text-amber">
                <Phone className="w-4 h-4" />
              </div>
              <div>
                <p className="text-xs text-ink-mute">Phone Number</p>
                <p className="font-semibold nums">{customer.phone || "Not provided"}</p>
              </div>
            </div>
            <div className="flex items-center gap-3 text-ink">
              {customer.email ? <a href={`mailto:${customer.email}`} aria-label={`Email ${customer.name}`} title={`Email ${customer.email}`} className="w-8 h-8 rounded-lg bg-surface-2 flex items-center justify-center text-amber hover:bg-amber/10 hover:text-amber-bright transition-colors">
                <Mail className="w-4 h-4" />
              </a> : <div className="w-8 h-8 rounded-lg bg-surface-2 flex items-center justify-center text-ink-mute"><Mail className="w-4 h-4" /></div>}
              <div>
                <p className="text-xs text-ink-mute">Email Address</p>
                {customer.email ? <a href={`mailto:${customer.email}`} className="font-semibold text-amber hover:underline">{customer.email}</a> : <p className="font-semibold">Not provided</p>}
              </div>
            </div>
            {customer.notes && (
              <div className="pt-3 border-t border-line text-xs text-ink-dim leading-relaxed">
                <p className="font-semibold text-ink mb-1">Customer Notes:</p>
                <p className="whitespace-pre-line">{customer.notes}</p>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Vehicles & ROs Main Tabs */}
        <div className="lg:col-span-2 space-y-6">
          {/* Vehicles List */}
          <Card>
            <CardHeader className="flex items-center justify-between">
              <h2 className="gf-eyebrow flex items-center gap-2">
                <Car className="w-3.5 h-3.5 text-amber" /> Vehicles ({vehicles.length})
              </h2>
              <button
                onClick={() => setShowAddVehicle(true)}
                className="text-xs font-semibold text-amber hover:underline flex items-center gap-1"
              >
                + Add Vehicle
              </button>
            </CardHeader>

            {editingVehicle && (
              <div className="p-4 bg-surface-2 border-b border-line">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-xs font-bold text-ink">Edit Vehicle</h3>
                  <button onClick={() => setEditingVehicle(null)} className="text-ink-mute">
                    <X className="w-3.5 h-3.5" />
                  </button>
                </div>
                <form onSubmit={saveVehicle} className="space-y-3">
                  <div className="grid grid-cols-3 gap-2">
                    <input
                      value={editVehicleForm.year}
                      onChange={(e) => setEditVehicleForm({ ...editVehicleForm, year: Number(e.target.value) || 0 })}
                      placeholder="Year"
                      type="number"
                      className="gf-input text-xs"
                    />
                    <input
                      value={editVehicleForm.make}
                      onChange={(e) => setEditVehicleForm({ ...editVehicleForm, make: e.target.value })}
                      placeholder="Make"
                      className="gf-input text-xs"
                    />
                    <input
                      value={editVehicleForm.model}
                      onChange={(e) => setEditVehicleForm({ ...editVehicleForm, model: e.target.value })}
                      placeholder="Model"
                      className="gf-input text-xs"
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <input
                      value={editVehicleForm.license_plate}
                      onChange={(e) => setEditVehicleForm({ ...editVehicleForm, license_plate: e.target.value })}
                      placeholder="License Plate"
                      className="gf-input text-xs"
                    />
                    <input
                      value={editVehicleForm.vin}
                      onChange={(e) => setEditVehicleForm({ ...editVehicleForm, vin: e.target.value })}
                      placeholder="VIN"
                      className="gf-input text-xs"
                    />
                  </div>
                  <div className="flex justify-end gap-2">
                    <button
                      type="button"
                      onClick={() => setEditingVehicle(null)}
                      className="gf-btn-secondary px-3 py-1 text-xs"
                    >
                      Cancel
                    </button>
                    <button type="submit" className="gf-btn-primary px-3 py-1 text-xs">
                      Save Vehicle
                    </button>
                  </div>
                </form>
              </div>
            )}

            {vehicles.length === 0 ? (
              <div className="px-5 py-8 text-center text-sm text-ink-mute">No vehicles on file</div>
            ) : (
              <ul className="divide-y divide-line">
                {vehicles.map((v) => (
                  <li key={v.id} className="flex items-center justify-between px-5 py-3.5 hover:bg-surface-2 transition-colors">
                    <Link href={`/vehicles/${v.id}`} className="flex-1 min-w-0 group">
                      <p className="text-sm font-semibold text-ink group-hover:text-amber transition-colors">
                        {v.year ? `${v.year} ` : ""}
                        {v.make} {v.model}
                      </p>
                      <p className="text-xs text-ink-mute nums mt-0.5">
                        Plate: {v.license_plate || "—"} · VIN: {v.vin || "—"}
                      </p>
                    </Link>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => startEditVehicle(v)}
                        className="text-xs text-ink-mute hover:text-amber p-1"
                        title="Edit vehicle"
                      >
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => deleteVehicle(v.id)}
                        className="text-xs text-ink-mute hover:text-rose-400 p-1"
                        title="Delete vehicle"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                      <Link href={`/vehicles/${v.id}`} className="text-ink-mute hover:text-ink">
                        <ChevronRight className="w-4 h-4" />
                      </Link>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Card>

          {/* Repair Orders List */}
          <Card>
            <CardHeader className="flex items-center justify-between">
              <h2 className="gf-eyebrow flex items-center gap-2">
                <ClipboardList className="w-3.5 h-3.5 text-amber" /> Repair Orders ({orders.length})
              </h2>
              <button
                onClick={() => setShowAddRO(true)}
                className="text-xs font-semibold text-amber hover:underline flex items-center gap-1"
              >
                + New RO
              </button>
            </CardHeader>
            {orders.length === 0 ? (
              <div className="px-5 py-8 text-center text-sm text-ink-mute">No repair orders yet</div>
            ) : (
              <ul className="divide-y divide-line">
                {orders.map((o) => (
                  <li key={o.id}>
                    <Link
                      href={`/repair-orders/${o.id}`}
                      className="flex items-center justify-between gap-3 px-5 py-3.5 hover:bg-surface-2 transition-colors group"
                    >
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-mono font-bold text-amber">
                            RO-{o.id.slice(0, 8).toUpperCase()}
                          </span>
                          <Badge status={o.status}>{o.status.replace(/_/g, " ")}</Badge>
                        </div>
                        <p className="text-sm font-medium text-ink truncate mt-1 group-hover:text-amber transition-colors">
                          {o.description || "General Repair Work"}
                        </p>
                      </div>
                      <ChevronRight className="w-4 h-4 text-ink-mute group-hover:text-ink shrink-0" />
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
