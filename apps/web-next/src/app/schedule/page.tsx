"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Bay, Schedule, RepairOrder } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { PageHeader } from "@/components/ui/PageHeader";
import { Plus, CalendarDays, Warehouse } from "lucide-react";

export default function SchedulePage() {
  const [bays, setBays] = useState<Bay[]>([]);
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [orders, setOrders] = useState<RepairOrder[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [bayName, setBayName] = useState("");
  const [form, setForm] = useState({ bay_id: "", repair_order_id: "", technician_id: "", start_time: "", end_time: "" });

  const load = useCallback(() => {
    api.get<Bay[]>("/schedule/bays").then(setBays).catch(() => {});
    api.get<Schedule[]>("/schedule").then(setSchedules).catch(() => {});
    api.get<RepairOrder[]>("/repair-orders").then(setOrders).catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const createBay = async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/schedule/bays", { name: bayName });
    setBayName("");
    load();
  };

  const createSchedule = async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/schedule", {
      ...form,
      start_time: new Date(form.start_time).toISOString(),
      end_time: new Date(form.end_time).toISOString(),
    });
    setForm({ bay_id: "", repair_order_id: "", technician_id: "", start_time: "", end_time: "" });
    setShowForm(false);
    load();
  };

  const bayMap = new Map(bays.map((b) => [b.id, b.name]));

  return (
    <div className="space-y-6">
      <PageHeader
        title="Schedule"
        subtitle={`${schedules.length} scheduled jobs`}
        action={
          <button onClick={() => setShowForm(!showForm)} className="gf-btn-primary">
            <Plus className="w-4 h-4" /> Add Schedule
          </button>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <Card className="lg:col-span-2">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr>
                  <th className="gf-th">Bay</th>
                  <th className="gf-th">RO</th>
                  <th className="gf-th">Start</th>
                  <th className="gf-th">End</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {schedules.map((s) => (
                  <tr key={s.id} className="hover:bg-surface-2 transition-colors">
                    <td className="px-5 py-3.5 text-sm font-medium text-ink">{bayMap.get(s.bay_id) || s.bay_id.slice(0, 8)}</td>
                    <td className="px-5 py-3.5 text-sm text-ink-dim nums">{s.repair_order_id.slice(0, 8)}</td>
                    <td className="px-5 py-3.5 text-sm text-ink-dim nums">{new Date(s.start_time).toLocaleString()}</td>
                    <td className="px-5 py-3.5 text-sm text-ink-dim nums">{new Date(s.end_time).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {schedules.length === 0 && (
            <div className="px-5 py-14 text-center">
              <CalendarDays className="w-8 h-8 text-ink-mute mx-auto mb-2" />
              <p className="text-sm text-ink-mute">No schedules yet</p>
            </div>
          )}
        </Card>

        <div className="space-y-6">
          <Card>
            <CardContent className="p-5 space-y-3">
              <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
                <Warehouse className="w-4 h-4 text-amber" />
                Service Bays
              </h2>
              {bays.map((b) => (
                <div key={b.id} className="flex items-center justify-between py-2 border-b border-line last:border-0">
                  <span className="text-sm font-medium text-ink">{b.name}</span>
                  <Badge status={b.active ? "active" : "inactive"}>{b.active ? "Active" : "Inactive"}</Badge>
                </div>
              ))}
              {bays.length === 0 && <p className="text-sm text-ink-mute">No bays yet</p>}
              <form onSubmit={createBay} className="flex gap-2 pt-2">
                <input value={bayName} onChange={(e) => setBayName(e.target.value)} placeholder="Bay name" className="gf-input flex-1" />
                <button type="submit" className="gf-btn-primary px-3 py-2.5">
                  Add
                </button>
              </form>
            </CardContent>
          </Card>

          {showForm && (
            <Card>
              <CardContent className="p-5">
                <h2 className="text-sm font-semibold text-ink mb-4">New Schedule</h2>
                <form onSubmit={createSchedule} className="space-y-4">
                  <select value={form.bay_id} onChange={(e) => setForm({ ...form, bay_id: e.target.value })} required className="gf-input">
                    <option value="">Select Bay</option>
                    {bays.map((b) => (
                      <option key={b.id} value={b.id}>
                        {b.name}
                      </option>
                    ))}
                  </select>
                  <select value={form.repair_order_id} onChange={(e) => setForm({ ...form, repair_order_id: e.target.value })} required className="gf-input">
                    <option value="">Select RO</option>
                    {orders.map((ro) => (
                      <option key={ro.id} value={ro.id}>
                        {ro.description || ro.id.slice(0, 8)}
                      </option>
                    ))}
                  </select>
                  <input type="datetime-local" value={form.start_time} onChange={(e) => setForm({ ...form, start_time: e.target.value })} required className="gf-input" />
                  <input type="datetime-local" value={form.end_time} onChange={(e) => setForm({ ...form, end_time: e.target.value })} required className="gf-input" />
                  <button type="submit" className="gf-btn-primary w-full">
                    Create
                  </button>
                </form>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
