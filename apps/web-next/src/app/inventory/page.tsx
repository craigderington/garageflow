"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { InventoryPart } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Plus, Package, ArrowDown, Trash2, Pencil, Archive, ArchiveRestore, Eye } from "lucide-react";

export default function InventoryPage() {
  const [parts, setParts] = useState<InventoryPart[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ name: "", sku: "", description: "", stock_level: 0, min_stock: 5, unit_price: 0 });
  const [restock, setRestock] = useState({ part_id: "", quantity: 0 });
  const [selected, setSelected] = useState<InventoryPart | null>(null);
  const [editing, setEditing] = useState<InventoryPart | null>(null);
  const [showArchived, setShowArchived] = useState(false);

  const load = useCallback(() => {
    api.get<InventoryPart[]>("/inventory?include_archived=true").then((items) => {
      setParts(items);
      const requested = typeof window !== "undefined" ? new URLSearchParams(window.location.search).get("item") : null;
      if (requested) setSelected(items.find((item) => item.id === requested) || null);
    }).catch(() => {});
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

  const saveEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editing) return;
    const updated = await api.patch<InventoryPart>(`/inventory/${editing.id}`, editing);
    setEditing(null);
    setSelected(updated);
    load();
  };

  const archivePart = async (part: InventoryPart) => {
    await api.post(`/inventory/${part.id}/archive`, { archived: !part.archived });
    setSelected(null);
    load();
  };

  const activeParts = parts.filter((part) => !part.archived);
  const displayedParts = showArchived ? parts : activeParts;

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

      <label className="inline-flex items-center gap-2 text-sm text-ink-dim cursor-pointer">
        <input type="checkbox" checked={showArchived} onChange={(e) => setShowArchived(e.target.checked)} /> Show archived inventory
      </label>

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

      {(selected || editing) && (
        <Card className="border-amber/30">
          <CardContent className="p-5">
            {editing ? (
              <form onSubmit={saveEdit} className="space-y-4">
                <h2 className="text-sm font-bold text-ink">Edit Inventory Item</h2>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                  <input value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} required className="gf-input" placeholder="Part name" />
                  <input value={editing.sku} onChange={(e) => setEditing({ ...editing, sku: e.target.value })} className="gf-input" placeholder="SKU" />
                  <input type="number" value={editing.unit_price} onChange={(e) => setEditing({ ...editing, unit_price: Number(e.target.value) })} className="gf-input" placeholder="Price" />
                  <input type="number" value={editing.stock_level} onChange={(e) => setEditing({ ...editing, stock_level: Number(e.target.value) })} className="gf-input" placeholder="Stock" />
                  <input type="number" value={editing.min_stock} onChange={(e) => setEditing({ ...editing, min_stock: Number(e.target.value) })} className="gf-input" placeholder="Minimum stock" />
                  <input value={editing.description} onChange={(e) => setEditing({ ...editing, description: e.target.value })} className="gf-input" placeholder="Description" />
                </div>
                <div className="flex gap-2"><button className="gf-btn-primary">Save Item</button><button type="button" onClick={() => setEditing(null)} className="gf-btn-ghost">Cancel</button></div>
              </form>
            ) : selected && (
              <div className="flex flex-col md:flex-row md:items-start justify-between gap-4">
                <div><p className="gf-eyebrow">Inventory Detail</p><h2 className="text-xl font-bold text-ink mt-1">{selected.name}</h2><p className="text-sm text-ink-dim mt-2">{selected.description || "No description provided."}</p><p className="text-xs text-ink-mute mt-3 nums">SKU {selected.sku || "—"} · {selected.stock_level} in stock · Minimum {selected.min_stock} · ${selected.unit_price.toFixed(2)}</p></div>
                <div className="flex gap-2"><button onClick={() => setEditing(selected)} className="gf-btn-secondary"><Pencil className="w-4 h-4" /> Edit</button><button onClick={() => archivePart(selected)} className="gf-btn-ghost">{selected.archived ? <ArchiveRestore className="w-4 h-4" /> : <Archive className="w-4 h-4" />} {selected.archived ? "Restore" : "Archive"}</button></div>
              </div>
            )}
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
                {displayedParts.map((p) => (
                  <tr key={p.id} className={`hover:bg-surface-2 transition-colors ${p.archived ? "opacity-55" : ""}`}>
                    <td className="px-5 py-3.5 text-sm font-medium text-ink"><button onClick={() => setSelected(p)} className="hover:text-amber">{p.name}</button></td>
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
                      <button onClick={() => setSelected(p)} aria-label={`View ${p.name}`} className="text-ink-mute hover:text-amber p-1"><Eye className="w-4 h-4" /></button>
                      <button onClick={() => setEditing(p)} aria-label={`Edit ${p.name}`} className="text-ink-mute hover:text-amber p-1"><Pencil className="w-4 h-4" /></button>
                      <button onClick={() => archivePart(p)} aria-label={`${p.archived ? "Restore" : "Archive"} ${p.name}`} className="text-ink-mute hover:text-amber p-1">{p.archived ? <ArchiveRestore className="w-4 h-4" /> : <Archive className="w-4 h-4" />}</button>
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
          {displayedParts.length === 0 && (
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
              {activeParts.map((p) => (
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
