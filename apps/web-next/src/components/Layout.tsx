"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/hooks/useAuth";
import type { ReactNode } from "react";
import {
  LayoutDashboard,
  ClipboardList,
  Users,
  Car,
  FileText,
  Package,
  Wrench,
  CalendarDays,
  LogOut,
  Menu,
} from "lucide-react";
import { useState } from "react";

const navItems = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/repair-orders", label: "Repair Orders", icon: ClipboardList },
  { href: "/customers", label: "Customers", icon: Users },
  { href: "/vehicles", label: "Vehicles", icon: Car },
  { href: "/estimates", label: "Estimates", icon: FileText },
  { href: "/inventory", label: "Inventory", icon: Package },
  { href: "/labor", label: "Labor", icon: Wrench },
  { href: "/schedule", label: "Schedule", icon: CalendarDays },
];

function Logo({ size = "base" }: { size?: "base" | "sm" }) {
  const box = size === "sm" ? "w-7 h-7" : "w-9 h-9";
  const icon = size === "sm" ? "w-3.5 h-3.5" : "w-[18px] h-[18px]";
  return (
    <div className={`relative ${box} rounded-lg bg-amber flex items-center justify-center shadow-[0_0_18px_-4px_var(--color-amber)]`}>
      <Wrench className={`${icon} text-black`} strokeWidth={2.5} />
    </div>
  );
}

export function Layout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const { user, logout } = useAuth();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  if (!user) return <>{children}</>;

  const sidebar = (
    <>
      {/* brand */}
      <div className="flex items-center gap-3 px-5 h-16 border-b border-line">
        <Logo />
        <div className="leading-tight">
          <h1 className="text-[15px] font-bold tracking-tight text-ink">
            Garage<span className="text-amber">Flow</span>
          </h1>
          <p className="gf-eyebrow">{user.role.replace(/_/g, " ")}</p>
        </div>
      </div>

      {/* nav */}
      <nav className="flex-1 px-3 py-4 space-y-0.5">
        <p className="px-3 pb-2 gf-eyebrow text-[10px]">Workspace</p>
        {navItems.map((item) => {
          const Icon = item.icon;
          const active = pathname === item.href || pathname.startsWith(item.href + "/");
          return (
            <Link
              key={item.href}
              href={item.href}
              onClick={() => setSidebarOpen(false)}
              className={`group relative flex items-center gap-3 px-3 py-2.5 rounded-md text-sm transition-colors ${
                active
                  ? "bg-amber/10 text-amber font-semibold"
                  : "text-ink-dim hover:bg-surface-2 hover:text-ink"
              }`}
            >
              {active && (
                <span className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-[3px] rounded-full bg-amber shadow-[0_0_10px_-1px_var(--color-amber)]" />
              )}
              <Icon
                className={`w-[18px] h-[18px] transition-colors ${
                  active ? "text-amber" : "text-ink-mute group-hover:text-ink-dim"
                }`}
                strokeWidth={2}
              />
              {item.label}
            </Link>
          );
        })}
      </nav>

      {/* sign out */}
      <div className="p-3 border-t border-line">
        <button
          onClick={logout}
          className="flex items-center gap-3 w-full px-3 py-2.5 rounded-md text-sm text-ink-mute hover:bg-surface-2 hover:text-rose-400 transition-colors"
        >
          <LogOut className="w-[18px] h-[18px]" strokeWidth={2} />
          Sign out
        </button>
      </div>
    </>
  );

  return (
    <div className="flex h-screen bg-base">
      {/* desktop sidebar */}
      <aside className="hidden lg:flex w-64 shrink-0 bg-surface border-r border-line flex-col">
        {sidebar}
      </aside>

      {/* mobile drawer */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-64 bg-surface border-r border-line flex flex-col transition-transform duration-200 lg:hidden ${
          sidebarOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {sidebar}
      </aside>
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/60 backdrop-blur-sm z-40 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <div className="flex-1 flex flex-col min-w-0">
        {/* mobile top bar */}
        <header className="h-16 bg-surface border-b border-line flex items-center px-4 gap-3 lg:hidden">
          <button
            onClick={() => setSidebarOpen(true)}
            className="text-ink-dim hover:text-ink p-1.5 -ml-1.5 rounded-md hover:bg-surface-2 transition-colors"
            aria-label="Open menu"
          >
            <Menu className="w-5 h-5" />
          </button>
          <div className="flex items-center gap-2">
            <Logo size="sm" />
            <span className="text-sm font-bold text-ink">
              Garage<span className="text-amber">Flow</span>
            </span>
          </div>
        </header>
        <main className="flex-1 overflow-auto">
          <div className="mx-auto max-w-7xl p-4 lg:p-8">{children}</div>
        </main>
      </div>
    </div>
  );
}
