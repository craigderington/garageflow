"use client";

import { useEffect, useState } from "react";
import { Plus, Save, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import type { Estimate, EstimateItem } from "@/lib/types";

type DraftItem = { type: "part" | "labor" | "fee"; description: string; quantity: number; unit_price: number };
const blankItem = (): DraftItem => ({ type: "part", description: "", quantity: 1, unit_price: 0 });

export function EstimateEditor({ repairOrderId, existing, onSaved }: {
  repairOrderId: string;
  existing?: { estimate: Estimate; items: EstimateItem[] } | null;
  onSaved: () => void;
}) {
  const [items, setItems] = useState<DraftItem[]>([blankItem()]);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    setItems(existing?.items.length ? existing.items.map((item) => ({ type: item.type, description: item.description, quantity: item.quantity, unit_price: item.unit_price })) : [blankItem()]);
  }, [existing]);

  const change = (index: number, patch: Partial<DraftItem>) => setItems((current) => current.map((item, i) => i === index ? { ...item, ...patch } : item));
  const total = items.reduce((sum, item) => sum + item.quantity * item.unit_price, 0);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setMessage("");
    try {
      if (existing) await api.put(`/estimates/${existing.estimate.id}`, { items });
      else await api.post("/estimates", { repair_order_id: repairOrderId, items });
      setMessage(existing ? "Estimate updated." : "Estimate created.");
      onSaved();
    } catch {
      setMessage("Estimate could not be saved.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-3">
      {items.map((item, index) => (
        <div key={index} className="grid grid-cols-1 sm:grid-cols-12 gap-2 items-center">
          <select value={item.type} onChange={(e) => change(index, { type: e.target.value as DraftItem["type"] })} className="gf-input sm:col-span-2">
            <option value="part">Part</option><option value="labor">Labor</option><option value="fee">Fee</option>
          </select>
          <input value={item.description} onChange={(e) => change(index, { description: e.target.value })} required placeholder="Description" className="gf-input sm:col-span-5" />
          <input type="number" min="0.01" step="0.01" value={item.quantity} onChange={(e) => change(index, { quantity: Number(e.target.value) })} aria-label="Quantity" className="gf-input sm:col-span-2" />
          <input type="number" min="0" step="0.01" value={item.unit_price} onChange={(e) => change(index, { unit_price: Number(e.target.value) })} aria-label="Unit price" className="gf-input sm:col-span-2" />
          <button type="button" disabled={items.length === 1} onClick={() => setItems(items.filter((_, i) => i !== index))} aria-label="Remove estimate item" className="p-2 text-ink-mute hover:text-rose-400 disabled:opacity-30"><Trash2 className="w-4 h-4" /></button>
        </div>
      ))}
      <div className="flex flex-wrap items-center justify-between gap-3 pt-2">
        <button type="button" onClick={() => setItems([...items, blankItem()])} className="gf-btn-ghost"><Plus className="w-4 h-4" /> Add Line Item</button>
        <div className="flex items-center gap-4"><span className="text-sm text-ink-dim">Total <strong className="text-amber nums ml-1">${total.toFixed(2)}</strong></span><button disabled={saving || items.some((item) => !item.description || item.quantity <= 0)} className="gf-btn-primary"><Save className="w-4 h-4" /> {saving ? "Saving…" : existing ? "Save Estimate" : "Create Estimate"}</button></div>
      </div>
      {message && <p role="status" className="text-xs text-ink-dim">{message}</p>}
    </form>
  );
}
