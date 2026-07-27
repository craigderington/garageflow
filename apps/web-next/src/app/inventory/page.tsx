"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { InventoryPart } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Plus, Package, ArrowDown, Trash2 } from "lucide-react";

export default function InventoryPage() {
  const [parts, setParts] = useState<InventoryPart[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ name: "", sku: "", description: "", stock_level: 0, min_stock: 5, unit_price: 0 });
  const [restock, setRestock] = useState({ part_id: "", quantity: 0 });

  const load = useCallback(() => {
    api.get<InventoryPart[]>("/inventory").then(setParts).catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/inventory", form);
    setForm({ name: "", sku: "", description: "", stock_level: 0, min_stock: 5, unit_price: 0 });
    setShowForm(false);
    load();
  };

  const doRestock = async () => {
    await api.post("/inventory/restock", restock);
    setRestock({ part_id: "", quantity: 0 });
    load();
  };

  const remove = async (id: string, name: string) => {
    if (!confirm(`Delete part "${name}"?`)) return;
    await api.del(`/inventory/${id}`);
    load();
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Inventory"
        subtitle={`${parts.length} parts tracked`}
        action={
          <button onClick={() => setShowForm(!showForm)} className="gf-btn-primary">
            <Plus className="w-4 h-4" /> Add Part
          </button>
        }
      />

      {showForm && (
        <Card>
          <CardContent className="p-5">
            <form onSubmit={create} className="space-y-4">
              <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="Part name" required className="gf-input" />
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <input value={form.sku} onChange={(e) => setForm({ ...form, sku: e.target.value })} placeholder="SKU" className="gf-input" />
                <input type="number" value={form.stock_level || ""} onChange={(e) => setForm({ ...form, stock_level: Number(e.target.value) })} placeholder="Stock level" className="gf-input" />
                <input type="number" value={form.unit_price || ""} onChange={(e) => setForm({ ...form, unit_price: Number(e.target.value) })} placeholder="Unit price" className="gf-input" />
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

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <Card className="lg:col-span-2">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr>
                  <th className="gf-th">Name</th>
                  <th className="gf-th">SKU</th>
                  <th className="gf-th">Stock</th>
                  <th className="gf-th">Min</th>
                  <th className="gf-th">Price</th>
                  <th className="gf-th"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {parts.map((p) => (
                  <tr key={p.id} className="hover:bg-surface-2 transition-colors">
                    <td className="px-5 py-3.5 text-sm font-medium text-ink">{p.name}</td>
                    <td className="px-5 py-3.5 text-sm text-ink-dim nums">{p.sku || "—"}</td>
                    <td className="px-5 py-3.5">
                      <span className={`nums inline-flex items-center gap-1.5 text-sm font-bold ${p.stock_level <= p.min_stock ? "text-rose-400" : "text-ink"}`}>
                        {p.stock_level <= p.min_stock && <span data-testid="low-stock" className="w-1.5 h-1.5 rounded-full bg-rose-500 shadow-[0_0_6px_0] shadow-rose-500/70" />}
                        {p.stock_level}
                      </span>
                    </td>
                    <td className="px-5 py-3.5 text-sm text-ink-dim nums">{p.min_stock}</td>
                    <td className="px-5 py-3.5 text-sm font-medium text-ink nums">${p.unit_price.toFixed(2)}</td>
                    <td className="px-5 py-3.5 text-right">
                      <button
                        onClick={() => remove(p.id, p.name)}
                        aria-label={`Delete ${p.name}`}
                        className="text-ink-mute hover:text-rose-400 transition-colors p-1 rounded hover:bg-rose-500/10"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {parts.length === 0 && (
            <div className="px-5 py-14 text-center">
              <Package className="w-8 h-8 text-ink-mute mx-auto mb-2" />
              <p className="text-sm text-ink-mute">No parts in inventory</p>
            </div>
          )}
        </Card>

        <Card>
          <CardContent className="p-5 space-y-4">
            <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
              <ArrowDown className="w-4 h-4 text-emerald-400" />
              Restock Parts
            </h2>
            <select value={restock.part_id} onChange={(e) => setRestock({ ...restock, part_id: e.target.value })} className="gf-input">
              <option value="">Select part</option>
              {parts.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name} ({p.stock_level} in stock)
                </option>
              ))}
            </select>
            <input
              type="number"
              value={restock.quantity || ""}
              onChange={(e) => setRestock({ ...restock, quantity: Number(e.target.value) })}
              placeholder="Quantity to add"
              className="gf-input"
            />
            <button onClick={doRestock} disabled={!restock.part_id || !restock.quantity} className="gf-btn-primary w-full">
              Restock
            </button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
