"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Customer } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Plus, Users } from "lucide-react";

export default function CustomersPage() {
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [notes, setNotes] = useState("");

  const load = useCallback(() => {
    api.get<Customer[]>("/customers").then(setCustomers).catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/customers", { name, phone, email, notes });
    setName("");
    setPhone("");
    setEmail("");
    setNotes("");
    setShowForm(false);
    load();
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Customers"
        subtitle={`${customers.length} registered customers`}
        action={
          <button onClick={() => setShowForm(!showForm)} className="gf-btn-primary">
            <Plus className="w-4 h-4" /> Add Customer
          </button>
        }
      />

      {showForm && (
        <Card>
          <CardContent className="p-5">
            <form onSubmit={create} className="space-y-4">
              <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Name" required className="gf-input" />
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="Phone" className="gf-input" />
                <input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="Email" className="gf-input" />
              </div>
              <textarea value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Notes" rows={2} className="gf-input" />
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

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr>
                <th className="gf-th">Name</th>
                <th className="gf-th">Phone</th>
                <th className="gf-th">Email</th>
                <th className="gf-th">Notes</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {customers.map((c) => (
                <tr key={c.id} className="hover:bg-surface-2 transition-colors">
                  <td className="px-5 py-3.5 text-sm font-medium">
                    <Link href={`/customers/${c.id}`} className="text-ink hover:text-amber transition-colors">
                      {c.name}
                    </Link>
                  </td>
                  <td className="px-5 py-3.5 text-sm text-ink-dim nums">{c.phone || "—"}</td>
                  <td className="px-5 py-3.5 text-sm text-ink-dim">{c.email || "—"}</td>
                  <td className="px-5 py-3.5 text-sm text-ink-mute truncate max-w-[240px]">{c.notes || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {customers.length === 0 && (
          <div className="px-5 py-14 text-center">
            <Users className="w-8 h-8 text-ink-mute mx-auto mb-2" />
            <p className="text-sm text-ink-mute">No customers yet</p>
          </div>
        )}
      </Card>
    </div>
  );
}
