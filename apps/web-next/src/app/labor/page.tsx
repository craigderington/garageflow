"use client";

import { useCallback, useEffect, useState, useMemo } from "react";
import { api, ApiError } from "@/lib/api";
import type { RepairOrder, LaborLog, User as TechnicianUser } from "@/lib/types";
import { Card, CardContent, CardHeader } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { PageHeader } from "@/components/ui/PageHeader";
import {
  Clock,
  Play,
  Square,
  UserCheck,
  Plus,
  DollarSign,
  Award,
  Pencil,
  Trash2,
  X,
  CheckCircle2,
} from "lucide-react";

export default function LaborPage() {
  const [orders, setOrders] = useState<RepairOrder[]>([]);
  const [logs, setLogs] = useState<LaborLog[]>([]);
  const [technicians, setTechnicians] = useState<TechnicianUser[]>([]);
  const [selectedRO, setSelectedRO] = useState("");
  const [selectedTech, setSelectedTech] = useState("");
  const [description, setDescription] = useState("");
  const [clockedIn, setClockedIn] = useState(false);
  const [techError, setTechError] = useState("");
  const [savingTech, setSavingTech] = useState(false);

  // Add / Edit Tech State
  const [showAddTech, setShowAddTech] = useState(false);
  const [editingTech, setEditingTech] = useState<TechnicianUser | null>(null);
  const [techForm, setTechForm] = useState({
    name: "",
    email: "",
    role: "technician" as TechnicianUser["role"],
    specialities: "",
    hourly_rate: 100,
  });

  const load = useCallback(() => {
    api.get<RepairOrder[]>("/repair-orders").then(setOrders).catch(() => {});
    api.get<TechnicianUser[]>("/users").then(setTechnicians).catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const loadLogs = useCallback(async (roId: string) => {
    const data = await api.get<LaborLog[]>(`/labor/ro/${roId}`).catch(() => []);
    setLogs(data);
  }, []);

  const techMap = useMemo(() => new Map(technicians.map((t) => [t.id, t])), [technicians]);

  const clockIn = async () => {
    if (!selectedRO) return;
    await api.post("/labor/clock-in", { repair_order_id: selectedRO, description });
    setClockedIn(true);
    loadLogs(selectedRO);
  };

  const clockOut = async (logId: string, minutes: number) => {
    await api.post(`/labor/clock-out/${logId}`, { minutes, description });
    setClockedIn(false);
    loadLogs(selectedRO);
  };

  const createTech = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!techForm.name.trim() || !techForm.email.trim()) return;
    setSavingTech(true);
    setTechError("");
    try {
      await api.post("/users", { ...techForm, email: techForm.email.trim().toLowerCase(), hourly_rate: Number(techForm.hourly_rate) || 100 });
      setTechForm({ name: "", email: "", role: "technician", specialities: "", hourly_rate: 100 });
      setShowAddTech(false);
      load();
    } catch (error) {
      setTechError(error instanceof ApiError && error.status === 403 ? "Only an owner or administrator can create technicians." : "Technician could not be created. The email may already be in use.");
    } finally {
      setSavingTech(false);
    }
  };

  const startEditTech = (t: TechnicianUser) => {
    setEditingTech(t);
    setTechForm({
      name: t.name,
      email: t.email,
      role: t.role,
      specialities: t.specialities || "",
      hourly_rate: t.hourly_rate || 100,
    });
  };

  const updateTech = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingTech) return;
    await api.put(`/users/${editingTech.id}`, {
      ...techForm,
      hourly_rate: Number(techForm.hourly_rate) || 100,
    });
    setEditingTech(null);
    setTechForm({ name: "", email: "", role: "technician", specialities: "", hourly_rate: 100 });
    load();
  };

  const deleteTech = async (id: string) => {
    if (!confirm("Remove this technician?")) return;
    await api.del(`/users/${id}`);
    load();
  };

  const activeLog = logs.find((l) => !l.clock_out);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Labor & Technician Management"
        subtitle="Track labor hours, assign technicians, configure hourly rates, and manage shop specialities"
        action={
          <button onClick={() => setShowAddTech(!showAddTech)} className="gf-btn-primary flex items-center gap-2">
            <Plus className="w-4 h-4" /> Add Technician
          </button>
        }
      />

      {/* Add / Edit Technician Form */}
      {(showAddTech || editingTech) && (
        <Card className="border-amber/30 bg-surface-2/60">
          <div className="flex items-center justify-between px-5 py-4 border-b border-line">
            <h2 className="text-sm font-bold text-ink flex items-center gap-2">
              <UserCheck className="w-4 h-4 text-amber" />
              {editingTech ? `Edit Technician: ${editingTech.name}` : "Add Technician / Staff Member"}
            </h2>
            <button
              onClick={() => {
                setShowAddTech(false);
                setEditingTech(null);
              }}
              className="text-ink-mute hover:text-ink"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
          <form onSubmit={editingTech ? updateTech : createTech} className="p-5 space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <input
                value={techForm.name}
                onChange={(e) => setTechForm({ ...techForm, name: e.target.value })}
                placeholder="Full Name"
                required
                className="gf-input"
              />
              <input
                value={techForm.email}
                onChange={(e) => setTechForm({ ...techForm, email: e.target.value })}
                placeholder="Email Address"
                type="email"
                required
                className="gf-input"
              />
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <label className="text-xs font-medium text-ink-dim mb-1 block">Role</label>
                <select
                  value={techForm.role}
                  onChange={(e) => setTechForm({ ...techForm, role: e.target.value as TechnicianUser["role"] })}
                  className="gf-input w-full"
                >
                  <option value="technician">Technician</option>
                  <option value="service_writer">Service Writer</option>
                  <option value="admin">Admin</option>
                  <option value="owner">Owner</option>
                </select>
              </div>

              <div>
                <label className="text-xs font-medium text-ink-dim mb-1 block">Hourly Labor Rate ($/hr)</label>
                <input
                  type="number"
                  value={techForm.hourly_rate || ""}
                  onChange={(e) => setTechForm({ ...techForm, hourly_rate: Number(e.target.value) || 0 })}
                  placeholder="e.g. 120"
                  required
                  className="gf-input w-full"
                />
              </div>

              <div>
                <label className="text-xs font-medium text-ink-dim mb-1 block">Specialities</label>
                <input
                  value={techForm.specialities}
                  onChange={(e) => setTechForm({ ...techForm, specialities: e.target.value })}
                  placeholder="e.g. Brakes, Diagnostics, Engine"
                  className="gf-input w-full"
                />
              </div>
            </div>

            <div className="flex justify-end gap-2 pt-2">
              {techError && <p className="text-xs text-rose-400 mr-auto self-center" role="alert">{techError}</p>}
              <button
                type="button"
                onClick={() => {
                  setShowAddTech(false);
                  setEditingTech(null);
                }}
                className="gf-btn-secondary"
              >
                Cancel
              </button>
              <button type="submit" disabled={savingTech} className="gf-btn-primary">
                {savingTech ? "Saving…" : editingTech ? "Save Changes" : "Create Technician"}
              </button>
            </div>
          </form>
        </Card>
      )}

      {/* Technician Roster Cards */}
      <div className="space-y-3">
        <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
          <Award className="w-4 h-4 text-amber" /> Technicians & Staff Roster
        </h2>
        {technicians.length === 0 ? (
          <Card className="p-6 text-center text-sm text-ink-mute">No technicians or staff configured.</Card>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {technicians.map((tech) => (
              <Card key={tech.id} className="p-4 relative group hover:border-amber/40 transition-colors">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-full bg-amber/10 border border-amber/30 flex items-center justify-center text-amber font-bold text-sm">
                      {tech.name ? tech.name.slice(0, 2).toUpperCase() : "TECH"}
                    </div>
                    <div>
                      <h4 className="text-sm font-bold text-ink">{tech.name || tech.email}</h4>
                      <p className="text-xs text-ink-mute">{tech.email}</p>
                    </div>
                  </div>
                  <Badge status={tech.role === "owner" ? "approved" : "active"}>
                    {tech.role.replace(/_/g, " ")}
                  </Badge>
                </div>

                <div className="mt-4 pt-3 border-t border-line flex items-center justify-between text-xs">
                  <span className="text-ink-dim font-medium flex items-center gap-1">
                    <DollarSign className="w-3.5 h-3.5 text-emerald-400" />
                    ${tech.hourly_rate ? tech.hourly_rate.toFixed(2) : "100.00"}/hr
                  </span>
                  <span className="text-ink-mute truncate max-w-[140px]">
                    {tech.specialities || "General Service"}
                  </span>
                </div>

                <div className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1 bg-surface-2 p-1 rounded-md border border-line">
                  <button onClick={() => startEditTech(tech)} className="text-ink-mute hover:text-amber p-1" title="Edit">
                    <Pencil className="w-3.5 h-3.5" />
                  </button>
                  <button onClick={() => deleteTech(tech.id)} className="text-ink-mute hover:text-rose-400 p-1" title="Delete">
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>

      {/* Clock In / Active Session Section */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardContent className="p-5 space-y-4">
            <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
              <Play className="w-4 h-4 text-emerald-400" />
              Clock In Technician
            </h2>

            <div>
              <label className="text-xs font-medium text-ink-dim mb-1 block">Repair Order *</label>
              <select
                value={selectedRO}
                onChange={(e) => {
                  setSelectedRO(e.target.value);
                  loadLogs(e.target.value);
                }}
                className="gf-input w-full"
              >
                <option value="">Select Repair Order</option>
                {orders.map((ro) => (
                  <option key={ro.id} value={ro.id}>
                    {ro.description || `RO-${ro.id.slice(0, 8)}`}
                  </option>
                ))}
              </select>
            </div>

            {selectedTech ? (
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="text-xs font-medium text-ink-dim block">Technician</label>
                  <button type="button" onClick={() => setSelectedTech("")} className="text-[10px] text-amber hover:underline">Change</button>
                </div>
                <select
                  value={selectedTech}
                  onChange={(e) => setSelectedTech(e.target.value)}
                  className="gf-input w-full"
                >
                  <option value="">Select Technician (Optional)</option>
                  {technicians.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.name} (${t.hourly_rate ? t.hourly_rate.toFixed(2) : "100.00"}/hr)
                    </option>
                  ))}
                </select>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setSelectedTech(technicians[0]?.id || "default")}
                className="text-xs text-amber hover:underline inline-flex items-center gap-1 font-medium"
              >
                + Assign Technician Identity & Rate
              </button>
            )}

            <div>
              <label className="text-xs font-medium text-ink-dim mb-1 block">Task Description</label>
              <input
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Task description"
                className="gf-input w-full"
              />
            </div>

            <button
              onClick={clockIn}
              disabled={!selectedRO || clockedIn}
              className="gf-btn-success w-full justify-center"
            >
              <Play className="w-4 h-4" /> Clock In
            </button>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-5 space-y-4">
            <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
              <Clock className="w-4 h-4 text-amber" />
              Active Labor Session
            </h2>
            {activeLog ? (
              <div className="p-4 bg-amber-400/10 border border-amber-400/25 rounded-lg space-y-3">
                <div>
                  <p className="text-sm font-medium text-ink">{activeLog.description || "No description"}</p>
                  <p className="text-xs text-ink-dim mt-1 nums">
                    Started {new Date(activeLog.clock_in).toLocaleTimeString()}
                  </p>
                </div>
                <div>
                  <label className="gf-label">Minutes worked</label>
                  <input type="number" id="clockout-minutes" placeholder="e.g. 45" className="gf-input" />
                </div>
                <button
                  onClick={() => {
                    const el = document.getElementById("clockout-minutes") as HTMLInputElement;
                    clockOut(activeLog.id, Number(el?.value) || 0);
                  }}
                  className="gf-btn-danger w-full justify-center"
                >
                  <Square className="w-4 h-4" /> Clock Out
                </button>
              </div>
            ) : (
              <div className="py-8 text-center text-ink-mute text-xs">
                <CheckCircle2 className="w-8 h-8 text-ink-mute opacity-50 mx-auto mb-2" />
                No active session for selected repair order
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Labor Logs Table */}
      <Card>
        <CardHeader>
          <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
            <Clock className="w-4 h-4 text-amber" /> Logged Labor Summary
          </h2>
        </CardHeader>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr>
                <th className="gf-th">RO</th>
                <th className="gf-th">Technician</th>
                <th className="gf-th">Minutes</th>
                <th className="gf-th">Est. Labor Cost</th>
                <th className="gf-th">Description</th>
                <th className="gf-th">Clock In</th>
                <th className="gf-th">Clock Out</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {logs
                .filter((l) => l.clock_out)
                .map((l) => {
                  const tech = techMap.get(l.mechanic_id);
                  const rate = tech?.hourly_rate || 100;
                  const cost = ((l.minutes || 0) / 60) * rate;

                  return (
                    <tr key={l.id} className="hover:bg-surface-2 transition-colors">
                      <td className="px-5 py-3.5 text-sm text-ink-dim nums font-mono">
                        RO-{l.repair_order_id.slice(0, 8).toUpperCase()}
                      </td>
                      <td className="px-5 py-3.5 text-sm text-ink font-medium">
                        {tech?.name || "Technician"}
                      </td>
                      <td className="px-5 py-3.5 text-sm font-bold text-ink nums">{l.minutes}m</td>
                      <td className="px-5 py-3.5 text-sm font-bold text-amber nums">${cost.toFixed(2)}</td>
                      <td className="px-5 py-3.5 text-sm text-ink-dim">{l.description || "—"}</td>
                      <td className="px-5 py-3.5 text-sm text-ink-dim nums">
                        {new Date(l.clock_in).toLocaleTimeString()}
                      </td>
                      <td className="px-5 py-3.5 text-sm text-ink-dim nums">
                        {l.clock_out ? new Date(l.clock_out).toLocaleTimeString() : "—"}
                      </td>
                    </tr>
                  );
                })}
            </tbody>
          </table>
        </div>
        {logs.filter((l) => l.clock_out).length === 0 && (
          <div className="px-5 py-14 text-center">
            <Clock className="w-8 h-8 text-ink-mute mx-auto mb-2" />
            <p className="text-sm text-ink-mute">No completed labor logs</p>
          </div>
        )}
      </Card>
    </div>
  );
}
