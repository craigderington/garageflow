"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { RepairOrder, InventoryPart, Customer } from "@/lib/types";
import { useAuth } from "@/hooks/useAuth";
import { useRouter } from "next/navigation";
import { StatCard } from "@/components/ui/StatCard";
import { Card, CardHeader, CardContent } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Wrench, Clock, Car, AlertTriangle, ArrowRight } from "lucide-react";

function dotForStatus(status: string) {
  if (["created", "diagnosed", "estimate_sent"].includes(status)) return "bg-amber-400";
  if (["approved", "in_progress"].includes(status)) return "bg-blue-400";
  if (status === "completed") return "bg-emerald-400";
  return "bg-zinc-500";
}

export default function DashboardPage() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const [orders, setOrders] = useState<RepairOrder[]>([]);
  const [inventory, setInventory] = useState<InventoryPart[]>([]);
  const [customers, setCustomers] = useState<Map<string, string>>(new Map());

  useEffect(() => {
    if (!loading && !user) router.push("/login");
  }, [user, loading, router]);

  useEffect(() => {
    api.get<RepairOrder[]>("/repair-orders").then(setOrders).catch(() => {});
    api.get<InventoryPart[]>("/inventory").then(setInventory).catch(() => {});
    api
      .get<Customer[]>("/customers")
      .then((cs) => {
        const m = new Map<string, string>();
        cs.forEach((c) => m.set(c.id, c.name));
        setCustomers(m);
      })
      .catch(() => {});
  }, []);

  if (loading || !user) return null;

  const inService = orders.filter((o) => ["approved", "in_progress"].includes(o.status));
  const awaitingService = orders.filter((o) =>
    ["created", "diagnosed", "estimate_sent"].includes(o.status)
  );
  const completedToday = orders.filter(
    (o) =>
      o.status === "completed" &&
      new Date(o.updated_at).toDateString() === new Date().toDateString()
  );
  const lowStock = inventory.filter((p) => p.stock_level <= p.min_stock);

  return (
    <div className="space-y-8">
      <div className="flex items-center gap-3">
        <span className="h-9 w-1 rounded-full bg-amber shadow-[0_0_12px_-2px_var(--color-amber)]" />
        <div>
          <h1 className="text-xl font-bold tracking-tight text-ink">
            Good {new Date().getHours() < 12 ? "morning" : "afternoon"}
          </h1>
          <p className="text-sm text-ink-dim mt-0.5">Here&apos;s what&apos;s happening on the floor today</p>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="In Service" value={inService.length} icon={<Wrench className="w-5 h-5" />} color="blue" />
        <StatCard label="Awaiting Service" value={awaitingService.length} icon={<Clock className="w-5 h-5" />} color="amber" />
        <StatCard label="Completed Today" value={completedToday.length} icon={<Car className="w-5 h-5" />} color="emerald" />
        <StatCard
          label="Low Stock Items"
          value={lowStock.length}
          icon={<AlertTriangle className="w-5 h-5" />}
          color="rose"
          sub={lowStock.slice(0, 3).map((p) => p.name).join(", ")}
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardHeader className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-blue-400 shadow-[0_0_8px_0] shadow-blue-400/60" />
              Vehicles In Service
            </h2>
            <span className="gf-eyebrow">{inService.length} active</span>
          </CardHeader>
          <CardContent className="p-0">
            {inService.length === 0 ? (
              <p className="px-5 py-10 text-sm text-ink-mute text-center">No vehicles currently in service</p>
            ) : (
              <div className="divide-y divide-line">
                {inService.map((ro) => (
                  <div key={ro.id} className="px-5 py-3.5 flex items-center justify-between hover:bg-surface-2 transition-colors">
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium text-ink truncate">{ro.description || "Repair Order"}</p>
                      <p className="text-xs text-ink-mute mt-0.5">
                        {customers.get(ro.customer_id) || "Unknown"} · <span className="nums">RO#{ro.id.slice(0, 7)}</span>
                      </p>
                    </div>
                    <Badge status={ro.status}>{ro.status.replace(/_/g, " ")}</Badge>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-amber-400 shadow-[0_0_8px_0] shadow-amber-400/60" />
              Awaiting Service
            </h2>
            <span className="gf-eyebrow">{awaitingService.length} queued</span>
          </CardHeader>
          <CardContent className="p-0">
            {awaitingService.length === 0 ? (
              <p className="px-5 py-10 text-sm text-ink-mute text-center">No vehicles waiting</p>
            ) : (
              <div className="divide-y divide-line">
                {awaitingService.map((ro) => (
                  <div key={ro.id} className="px-5 py-3.5 flex items-center justify-between hover:bg-surface-2 transition-colors">
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium text-ink truncate">{ro.description || "Repair Order"}</p>
                      <p className="text-xs text-ink-mute mt-0.5">
                        {customers.get(ro.customer_id) || "Unknown"} · <span className="nums">{ro.mileage.toLocaleString()} mi</span>
                      </p>
                    </div>
                    <Badge status={ro.status}>{ro.status.replace(/_/g, " ")}</Badge>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <Card className="lg:col-span-2">
          <CardHeader>
            <h2 className="text-sm font-semibold text-ink">Recent Repair Orders</h2>
          </CardHeader>
          <CardContent className="p-0">
            {orders.length === 0 ? (
              <p className="px-5 py-10 text-sm text-ink-mute text-center">No repair orders yet</p>
            ) : (
              <div className="divide-y divide-line">
                {orders.slice(0, 8).map((ro) => (
                  <div key={ro.id} className="px-5 py-3 flex items-center justify-between hover:bg-surface-2 transition-colors">
                    <div className="flex items-center gap-3 min-w-0">
                      <span className={`w-2 h-2 rounded-full shrink-0 ${dotForStatus(ro.status)}`} />
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-ink truncate">{ro.description || "No description"}</p>
                        <p className="text-xs text-ink-mute">
                          {customers.get(ro.customer_id) || "Unknown"} ·{" "}
                          <span className="nums">{new Date(ro.created_at).toLocaleDateString()}</span>
                        </p>
                      </div>
                    </div>
                    <Badge status={ro.status}>{ro.status.replace(/_/g, " ")}</Badge>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <h2 className="text-sm font-semibold text-ink flex items-center gap-2">
              <AlertTriangle className="w-3.5 h-3.5 text-rose-400" />
              Low Stock Alerts
            </h2>
          </CardHeader>
          <CardContent className="p-0">
            {lowStock.length === 0 ? (
              <p className="px-5 py-10 text-sm text-ink-mute text-center">All stock levels are healthy</p>
            ) : (
              <div className="divide-y divide-line">
                {lowStock.slice(0, 6).map((p) => (
                  <div key={p.id} className="px-5 py-3 flex items-center justify-between">
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-ink truncate">{p.name}</p>
                      <p className="text-xs text-ink-mute">SKU: <span className="nums">{p.sku || "—"}</span></p>
                    </div>
                    <span className="nums text-sm font-bold text-rose-400">{p.stock_level}</span>
                  </div>
                ))}
              </div>
            )}
            <div className="px-5 py-3 border-t border-line">
              <a href="/inventory" className="text-xs text-amber hover:text-amber-bright font-semibold flex items-center gap-1 transition-colors">
                View all inventory <ArrowRight className="w-3 h-3" />
              </a>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
