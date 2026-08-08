"use client";

import { useEffect, useState } from "react";
import { Clock, Plus } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import type { RepairOrder, User } from "@/lib/types";
import { Card, CardContent, CardHeader } from "@/components/ui/Card";

export function InlineLabor({ orders, initialRepairOrderId = "" }: { orders: RepairOrder[]; initialRepairOrderId?: string }) {
  const [technicians, setTechnicians] = useState<User[]>([]);
  const [repairOrderId, setRepairOrderId] = useState(initialRepairOrderId);
  const [mechanicId, setMechanicId] = useState("");
  const [minutes, setMinutes] = useState(0);
  const [description, setDescription] = useState("");
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api.get<User[]>("/users").then((users) => {
      setTechnicians(users);
      setMechanicId((current) => current || users.find((u) => u.role === "technician")?.id || users[0]?.id || "");
    }).catch(() => setMessage("Could not load technicians."));
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setMessage("");
    try {
      await api.post("/labor/manual", { repair_order_id: repairOrderId, mechanic_id: mechanicId, minutes, description });
      setMinutes(0);
      setDescription("");
      setMessage("Labor added to the repair order.");
    } catch (error) {
      setMessage(error instanceof ApiError ? "Labor could not be added. Check your access and selections." : "Labor could not be added.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
          <Clock className="w-4 h-4 text-amber" /> Add Completed Labor
        </h2>
      </CardHeader>
      <CardContent className="p-5">
        <form onSubmit={submit} className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-3 items-end">
          <div className="lg:col-span-2">
            <label className="gf-label">Repair order</label>
            <select value={repairOrderId} onChange={(e) => setRepairOrderId(e.target.value)} required className="gf-input">
              <option value="">Select repair order</option>
              {orders.map((ro) => <option key={ro.id} value={ro.id}>{ro.description || `RO-${ro.id.slice(0, 8)}`}</option>)}
            </select>
          </div>
          <div>
            <label className="gf-label">Technician</label>
            <select value={mechanicId} onChange={(e) => setMechanicId(e.target.value)} required className="gf-input">
              <option value="">Select technician</option>
              {technicians.map((tech) => <option key={tech.id} value={tech.id}>{tech.name || tech.email}</option>)}
            </select>
          </div>
          <div>
            <label className="gf-label">Minutes</label>
            <input type="number" min="1" value={minutes || ""} onChange={(e) => setMinutes(Number(e.target.value))} required className="gf-input" placeholder="45" />
          </div>
          <button disabled={saving || !repairOrderId || !mechanicId || minutes <= 0} className="gf-btn-primary justify-center">
            <Plus className="w-4 h-4" /> {saving ? "Adding…" : "Add Labor"}
          </button>
          <div className="md:col-span-2 lg:col-span-5">
            <label className="gf-label">Work performed</label>
            <input value={description} onChange={(e) => setDescription(e.target.value)} className="gf-input" placeholder="Diagnostics, brake service, inspection…" />
          </div>
        </form>
        {message && <p className="text-xs text-ink-dim mt-3" role="status">{message}</p>}
      </CardContent>
    </Card>
  );
}
