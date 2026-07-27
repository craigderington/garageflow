"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { RepairOrder, LaborLog } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Clock, Play, Square } from "lucide-react";

export default function LaborPage() {
  const [orders, setOrders] = useState<RepairOrder[]>([]);
  const [logs, setLogs] = useState<LaborLog[]>([]);
  const [selectedRO, setSelectedRO] = useState("");
  const [description, setDescription] = useState("");
  const [clockedIn, setClockedIn] = useState(false);

  const load = useCallback(() => {
    api.get<RepairOrder[]>("/repair-orders").then(setOrders).catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const loadLogs = useCallback(async (roId: string) => {
    const data = await api.get<LaborLog[]>(`/labor/ro/${roId}`).catch(() => []);
    setLogs(data);
  }, []);

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

  const activeLog = logs.find((l) => !l.clock_out);

  return (
    <div className="space-y-6">
      <PageHeader title="Labor Tracking" subtitle="Track technician time on repair orders" />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardContent className="p-5 space-y-4">
            <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
              <Play className="w-4 h-4 text-emerald-400" />
              Clock In
            </h2>
            <select
              value={selectedRO}
              onChange={(e) => {
                setSelectedRO(e.target.value);
                loadLogs(e.target.value);
              }}
              className="gf-input"
            >
              <option value="">Select Repair Order</option>
              {orders.map((ro) => (
                <option key={ro.id} value={ro.id}>
                  {ro.description || ro.id.slice(0, 8)}
                </option>
              ))}
            </select>
            <input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Task description" className="gf-input" />
            <button onClick={clockIn} disabled={!selectedRO || clockedIn} className="gf-btn-success w-full">
              <Play className="w-4 h-4" /> Clock In
            </button>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-5 space-y-4">
            <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
              <Clock className="w-4 h-4 text-amber" />
              Active Session
            </h2>
            {activeLog ? (
              <div className="p-4 bg-amber-400/10 border border-amber-400/25 rounded-lg space-y-3">
                <div>
                  <p className="text-sm font-medium text-ink">{activeLog.description || "No description"}</p>
                  <p className="text-xs text-ink-dim mt-1 nums">Started {new Date(activeLog.clock_in).toLocaleTimeString()}</p>
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
                  className="gf-btn-danger w-full"
                >
                  <Square className="w-4 h-4" /> Clock Out
                </button>
              </div>
            ) : (
              <p className="text-sm text-ink-mute py-4 text-center">No active session</p>
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr>
                <th className="gf-th">RO</th>
                <th className="gf-th">Minutes</th>
                <th className="gf-th">Description</th>
                <th className="gf-th">Clock In</th>
                <th className="gf-th">Clock Out</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {logs
                .filter((l) => l.clock_out)
                .map((l) => (
                  <tr key={l.id} className="hover:bg-surface-2 transition-colors">
                    <td className="px-5 py-3.5 text-sm text-ink-dim nums">{l.repair_order_id.slice(0, 8)}</td>
                    <td className="px-5 py-3.5 text-sm font-bold text-ink nums">{l.minutes}m</td>
                    <td className="px-5 py-3.5 text-sm text-ink-dim">{l.description || "—"}</td>
                    <td className="px-5 py-3.5 text-sm text-ink-dim nums">{new Date(l.clock_in).toLocaleTimeString()}</td>
                    <td className="px-5 py-3.5 text-sm text-ink-dim nums">{l.clock_out ? new Date(l.clock_out).toLocaleTimeString() : "—"}</td>
                  </tr>
                ))}
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
