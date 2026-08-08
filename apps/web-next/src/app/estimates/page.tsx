"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { RepairOrder, Estimate, EstimateItem } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { PageHeader } from "@/components/ui/PageHeader";
import { EstimateEditor } from "@/components/EstimateEditor";
import { Plus, FileText, Send, CheckCircle, CreditCard, Pencil } from "lucide-react";

export default function EstimatesPage() {
  const [orders, setOrders] = useState<RepairOrder[]>([]);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [estimates, setEstimates] = useState<Map<string, any>>(new Map());
  const [showForm, setShowForm] = useState(false);
  const [selectedRO, setSelectedRO] = useState("");
  const [items, setItems] = useState([{ type: "part", description: "", quantity: 1, unit_price: 0 }]);
  const [editing, setEditing] = useState<{ roId: string; data: { estimate: Estimate; items: EstimateItem[] } } | null>(null);

  const load = useCallback(async () => {
    const ros = await api.get<RepairOrder[]>("/repair-orders").catch(() => []);
    setOrders(ros);
    const estMap = new Map();
    for (const ro of ros) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const est = await api.get<any>(`/estimates/ro/${ro.id}`).catch(() => null);
      if (est) estMap.set(ro.id, est);
    }
    setEstimates(estMap);
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const addItem = () => setItems([...items, { type: "part", description: "", quantity: 1, unit_price: 0 }]);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/estimates", { repair_order_id: selectedRO, items });
    setShowForm(false);
    setItems([{ type: "part", description: "", quantity: 1, unit_price: 0 }]);
    load();
  };

  const send = async (id: string) => {
    await api.post(`/estimates/${id}/send`);
    load();
  };
  const approve = async (id: string) => {
    await api.post(`/estimates/${id}/approve`);
    load();
  };
  const pay = async (id: string) => {
    const res = await api.post<{ url: string; mode: string }>(`/estimates/${id}/pay`);
    if (res.mode === "stripe" && res.url) {
      window.location.href = res.url; // hosted Stripe Checkout
    } else {
      load(); // dev mode settles server-side
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Estimates"
        subtitle={`${estimates.size} estimates created`}
        action={
          <button onClick={() => setShowForm(!showForm)} className="gf-btn-primary">
            <Plus className="w-4 h-4" /> New Estimate
          </button>
        }
      />

      {showForm && (
        <Card>
          <CardContent className="p-5">
            <form onSubmit={create} className="space-y-4">
              <select value={selectedRO} onChange={(e) => setSelectedRO(e.target.value)} required className="gf-input">
                <option value="">Select Repair Order</option>
                {orders
                  .filter((o) => !estimates.has(o.id))
                  .map((ro) => (
                    <option key={ro.id} value={ro.id}>
                      {ro.description || ro.id.slice(0, 8)}
                    </option>
                  ))}
              </select>
              {items.map((item, i) => (
                <div key={i} className="grid grid-cols-1 sm:grid-cols-4 gap-2">
                  <select
                    value={item.type}
                    onChange={(e) => {
                      const n = [...items];
                      n[i].type = e.target.value;
                      setItems(n);
                    }}
                    className="gf-input"
                  >
                    <option value="part">Part</option>
                    <option value="labor">Labor</option>
                    <option value="fee">Fee</option>
                  </select>
                  <input
                    value={item.description}
                    onChange={(e) => {
                      const n = [...items];
                      n[i].description = e.target.value;
                      setItems(n);
                    }}
                    placeholder="Description"
                    className="gf-input col-span-1 sm:col-span-2"
                  />
                  <div className="flex gap-1">
                    <input
                      type="number"
                      value={item.quantity}
                      onChange={(e) => {
                        const n = [...items];
                        n[i].quantity = Number(e.target.value);
                        setItems(n);
                      }}
                      className="gf-input w-16"
                    />
                    <input
                      type="number"
                      value={item.unit_price}
                      onChange={(e) => {
                        const n = [...items];
                        n[i].unit_price = Number(e.target.value);
                        setItems(n);
                      }}
                      className="gf-input flex-1"
                    />
                  </div>
                </div>
              ))}
              <div className="flex gap-2">
                <button type="button" onClick={addItem} className="gf-btn-ghost">
                  + Add Item
                </button>
                <button type="submit" className="gf-btn-primary">
                  Create Estimate
                </button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {editing && (
        <Card className="border-amber/30">
          <CardContent className="p-5 space-y-4">
            <div className="flex items-center justify-between"><h2 className="text-sm font-bold text-ink">Edit Estimate</h2><button onClick={() => setEditing(null)} className="gf-btn-ghost text-xs">Cancel</button></div>
            <EstimateEditor repairOrderId={editing.roId} existing={editing.data} onSaved={() => { setEditing(null); load(); }} />
          </CardContent>
        </Card>
      )}

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr>
                <th className="gf-th">RO</th>
                <th className="gf-th">Total</th>
                <th className="gf-th">Status</th>
                <th className="gf-th">Items</th>
                <th className="gf-th">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {orders
                .filter((ro) => estimates.has(ro.id))
                .map((ro) => {
                  const est = estimates.get(ro.id);
                  return (
                    <tr key={ro.id} className="hover:bg-surface-2 transition-colors">
                      <td className="px-5 py-3.5 text-sm font-medium text-ink">{ro.description || ro.id.slice(0, 8)}</td>
                      <td className="px-5 py-3.5 text-sm font-semibold text-amber nums">${est?.estimate?.total.toFixed(2)}</td>
                      <td className="px-5 py-3.5">
                        <Badge status={est?.estimate?.status}>{est?.estimate?.status}</Badge>
                      </td>
                      <td className="px-5 py-3.5 text-sm text-ink-dim nums">{est?.items?.length || 0} items</td>
                      <td className="px-5 py-3.5">
                        <div className="flex gap-1.5">
                          {est?.estimate?.status !== "paid" && (
                            <button onClick={() => setEditing({ roId: ro.id, data: est })} className="gf-btn px-2.5 py-1.5 text-xs bg-blue-400/10 text-blue-300 border border-blue-400/25 hover:bg-blue-400/20"><Pencil className="w-3 h-3" /> Edit</button>
                          )}
                          {est?.estimate?.status === "draft" && (
                            <button
                              onClick={() => send(est.estimate.id)}
                              className="gf-btn px-2.5 py-1.5 text-xs bg-amber-400/10 text-amber-300 border border-amber-400/25 hover:bg-amber-400/20"
                            >
                              <Send className="w-3 h-3" /> Send
                            </button>
                          )}
                          {est?.estimate?.status === "sent" && (
                            <button
                              onClick={() => approve(est.estimate.id)}
                              className="gf-btn px-2.5 py-1.5 text-xs bg-emerald-400/10 text-emerald-300 border border-emerald-400/25 hover:bg-emerald-400/20"
                            >
                              <CheckCircle className="w-3 h-3" /> Approve
                            </button>
                          )}
                          {est?.estimate?.status === "approved" && (
                            <button
                              onClick={() => pay(est.estimate.id)}
                              className="gf-btn px-2.5 py-1.5 text-xs bg-amber/10 text-amber border border-amber/30 hover:bg-amber/20"
                            >
                              <CreditCard className="w-3 h-3" /> Collect payment
                            </button>
                          )}
                          {est?.estimate?.status === "paid" && (
                            <span className="text-xs font-semibold text-emerald-400 flex items-center gap-1">
                              <CheckCircle className="w-3 h-3" /> Paid
                            </span>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
            </tbody>
          </table>
        </div>
        {estimates.size === 0 && (
          <div className="px-5 py-14 text-center">
            <FileText className="w-8 h-8 text-ink-mute mx-auto mb-2" />
            <p className="text-sm text-ink-mute">No estimates yet</p>
          </div>
        )}
      </Card>
    </div>
  );
}
