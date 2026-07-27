"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/useAuth";
import {
  Wrench,
  ClipboardList,
  FileText,
  Package,
  CalendarDays,
  Users,
  ArrowRight,
  Check,
  Gauge,
} from "lucide-react";

/* ---------------------------------- nav --------------------------------- */
function TopNav() {
  return (
    <header className="sticky top-0 z-40 border-b border-line/70 bg-base/80 backdrop-blur-md">
      <div className="mx-auto max-w-6xl px-5 h-16 flex items-center justify-between">
        <Link href="/" className="flex items-center gap-2.5">
          <span className="relative w-8 h-8 rounded-lg bg-amber flex items-center justify-center shadow-[0_0_18px_-4px_var(--color-amber)]">
            <Wrench className="w-4 h-4 text-black" strokeWidth={2.5} />
          </span>
          <span className="text-[15px] font-bold tracking-tight">
            Garage<span className="text-amber">Flow</span>
          </span>
        </Link>
        <nav className="hidden md:flex items-center gap-8 text-sm text-ink-dim">
          <a href="#features" className="hover:text-ink transition-colors">Features</a>
          <a href="#workflow" className="hover:text-ink transition-colors">How it works</a>
          <a href="#pricing" className="hover:text-ink transition-colors">Pricing</a>
        </nav>
        <div className="flex items-center gap-2.5">
          <Link href="/login" className="text-sm text-ink-dim hover:text-ink transition-colors px-3 py-2">
            Sign in
          </Link>
          <Link href="/login" className="gf-btn-primary text-sm">
            Start free <ArrowRight className="w-4 h-4" />
          </Link>
        </div>
      </div>
    </header>
  );
}

/* ----------------------------- hero floor board ------------------------- */
function BayRow({ bay, vehicle, status, tone }: { bay: string; vehicle: string; status: string; tone: string }) {
  return (
    <div className="flex items-center gap-3 px-4 py-3 border-b border-line last:border-0">
      <span className="nums text-[11px] text-ink-mute w-10 shrink-0">{bay}</span>
      <span className="text-sm text-ink truncate flex-1">{vehicle}</span>
      <span className={`text-[10px] font-semibold uppercase tracking-wide px-2 py-0.5 rounded border ${tone}`}>
        {status}
      </span>
    </div>
  );
}

function FloorBoard() {
  return (
    <div className="gf-card overflow-hidden w-full shadow-[0_30px_80px_-30px_rgba(0,0,0,0.8)]">
      <div className="hazard h-1.5 w-full opacity-90" />
      <div className="flex items-center justify-between px-4 py-3 border-b border-line">
        <div className="flex items-center gap-2">
          <Gauge className="w-4 h-4 text-amber" />
          <span className="text-sm font-bold">Shop Floor</span>
        </div>
        <span className="gf-eyebrow text-[10px]">live</span>
      </div>
      <BayRow bay="BAY 1" vehicle="’19 Tacoma · brake job" status="in progress" tone="bg-blue-400/10 text-blue-300 border-blue-400/25" />
      <BayRow bay="BAY 2" vehicle="’21 F-150 · oil + rotate" status="approved" tone="bg-emerald-400/10 text-emerald-300 border-emerald-400/25" />
      <BayRow bay="BAY 3" vehicle="’18 Civic · diagnostics" status="estimate" tone="bg-amber-400/10 text-amber-300 border-amber-400/25" />
      <BayRow bay="LOT" vehicle="’22 Sierra · waiting" status="created" tone="bg-zinc-400/10 text-zinc-300 border-zinc-400/20" />
      <div className="grid grid-cols-3 divide-x divide-line border-t border-line bg-surface-2/40">
        {[
          ["3", "in service"],
          ["7", "queued"],
          ["$4.2k", "today"],
        ].map(([n, l]) => (
          <div key={l} className="px-4 py-3 text-center">
            <p className="nums text-lg font-bold text-ink leading-none">{n}</p>
            <p className="gf-eyebrow text-[9px] mt-1">{l}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

function Hero() {
  return (
    <section className="relative overflow-hidden border-b border-line">
      <div className="absolute inset-0 blueprint opacity-60" aria-hidden />
      <div className="absolute inset-0 amber-wash" aria-hidden />
      <div className="relative mx-auto max-w-6xl px-5 pt-20 pb-24 grid lg:grid-cols-2 gap-14 items-center">
        <div>
          <p className="gf-eyebrow text-amber/90 mb-5">Shop management for independent garages</p>
          <h1 className="text-4xl sm:text-5xl lg:text-[3.4rem] font-bold tracking-tight leading-[1.05]">
            Run the bay,
            <br />
            not the <span className="text-amber">paperwork</span>.
          </h1>
          <p className="mt-6 text-lg text-ink-dim max-w-md leading-relaxed">
            GarageFlow keeps every repair order, estimate, part, and tech on one screen — from
            check-in to keys back in hand.
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Link href="/login" className="gf-btn-primary">
              Start free <ArrowRight className="w-4 h-4" />
            </Link>
            <a href="#workflow" className="gf-btn-ghost">See how it works</a>
          </div>
          <p className="mt-6 text-xs text-ink-mute">
            No credit card · one login for the whole shop
          </p>
        </div>
        <div className="relative">
          <div className="absolute -inset-6 hazard opacity-[0.06] rounded-xl" aria-hidden />
          <div className="relative">
            <FloorBoard />
          </div>
        </div>
      </div>
    </section>
  );
}

/* --------------------------------- stats -------------------------------- */
function StatBand() {
  const stats = [
    ["6", "modules, one login"],
    ["0", "spreadsheets needed"],
    ["24/7", "customer portal"],
    ["1", "source of truth"],
  ];
  return (
    <section className="border-b border-line bg-surface/40">
      <div className="mx-auto max-w-6xl px-5 grid grid-cols-2 lg:grid-cols-4 divide-x divide-y lg:divide-y-0 divide-line">
        {stats.map(([n, l]) => (
          <div key={l} className="px-6 py-8 text-center">
            <p className="nums text-3xl font-bold text-amber leading-none">{n}</p>
            <p className="text-sm text-ink-dim mt-2">{l}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

/* ------------------------------- features ------------------------------- */
const FEATURES = [
  { icon: ClipboardList, name: "Repair Orders", body: "Open a ticket at check-in and track it through every status until pickup." },
  { icon: FileText, name: "Estimates", body: "Build line-item estimates, send them, and capture approval before the work starts." },
  { icon: Package, name: "Inventory", body: "Track parts and stock levels with low-stock alerts before you run dry mid-job." },
  { icon: Wrench, name: "Labor Tracking", body: "Clock techs in and out per order so every billable minute is on the books." },
  { icon: CalendarDays, name: "Scheduling", body: "Assign jobs to bays and time slots so the floor never double-books." },
  { icon: Users, name: "Customer Portal", body: "Customers approve estimates and follow their repair without a single phone tag." },
];

function Features() {
  return (
    <section id="features" className="border-b border-line scroll-mt-16">
      <div className="mx-auto max-w-6xl px-5 py-20">
        <p className="gf-eyebrow text-amber/90">Everything the shop runs on</p>
        <h2 className="mt-3 text-3xl font-bold tracking-tight max-w-xl">
          One platform for the whole floor
        </h2>
        <div className="mt-12 grid sm:grid-cols-2 lg:grid-cols-3 gap-px bg-line rounded-xl overflow-hidden border border-line">
          {FEATURES.map((f) => {
            const Icon = f.icon;
            return (
              <div key={f.name} className="group bg-surface p-6 hover:bg-surface-2 transition-colors">
                <span className="inline-flex p-2.5 rounded-lg bg-amber/10 text-amber ring-1 ring-amber/20">
                  <Icon className="w-5 h-5" />
                </span>
                <h3 className="mt-4 text-base font-bold text-ink">{f.name}</h3>
                <p className="mt-2 text-sm text-ink-dim leading-relaxed">{f.body}</p>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

/* ------------------------------- workflow ------------------------------- */
const STEPS = [
  ["Check in", "Open a repair order with the customer and vehicle in seconds."],
  ["Diagnose", "Log findings and move the order to diagnosed when you know the job."],
  ["Estimate", "Quote parts and labor, then send it for the customer to approve."],
  ["Approve", "Approval flips the order to ready — no chasing signatures."],
  ["Repair", "Assign a bay, clock in, and work the job with everything in view."],
  ["Deliver", "Close it out, hand back the keys, and the history stays on file."],
];

function Workflow() {
  return (
    <section id="workflow" className="border-b border-line bg-surface/30 scroll-mt-16">
      <div className="mx-auto max-w-6xl px-5 py-20">
        <p className="gf-eyebrow text-amber/90">From keys-in to keys-out</p>
        <h2 className="mt-3 text-3xl font-bold tracking-tight max-w-xl">The repair, start to finish</h2>
        <div className="mt-12 grid sm:grid-cols-2 lg:grid-cols-3 gap-6">
          {STEPS.map(([title, body], i) => (
            <div key={title} className="relative gf-card p-6">
              <span className="nums text-2xl font-bold text-amber/30 absolute top-4 right-5">
                {String(i + 1).padStart(2, "0")}
              </span>
              <h3 className="text-base font-bold text-ink pr-10">{title}</h3>
              <p className="mt-2 text-sm text-ink-dim leading-relaxed">{body}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

/* -------------------------------- pricing ------------------------------- */
const TIERS = [
  { name: "Solo", price: "$0", sub: "one bay, getting started", features: ["1 service bay", "Repair orders & estimates", "Inventory tracking", "Email support"], cta: "Start free", highlight: false },
  { name: "Shop", price: "$89", sub: "the busy independent garage", features: ["Up to 6 bays", "Scheduling & labor tracking", "Customer portal", "Priority support"], cta: "Start free", highlight: true },
  { name: "Multi-Bay", price: "Let's talk", sub: "multiple locations", features: ["Unlimited bays", "Multi-location reporting", "Role-based access", "Dedicated onboarding"], cta: "Contact us", highlight: false },
];

function Pricing() {
  return (
    <section id="pricing" className="border-b border-line scroll-mt-16">
      <div className="mx-auto max-w-6xl px-5 py-20">
        <p className="gf-eyebrow text-amber/90">Pricing</p>
        <h2 className="mt-3 text-3xl font-bold tracking-tight">Priced per shop, not per headache</h2>
        <div className="mt-12 grid lg:grid-cols-3 gap-6">
          {TIERS.map((t) => (
            <div
              key={t.name}
              className={`relative rounded-xl p-6 flex flex-col ${
                t.highlight ? "bg-surface-2 border border-amber/40 shadow-[0_0_50px_-20px_var(--color-amber)]" : "gf-card"
              }`}
            >
              {t.highlight && (
                <span className="absolute -top-3 left-6 text-[10px] font-bold uppercase tracking-wide bg-amber text-black px-2.5 py-1 rounded">
                  Most popular
                </span>
              )}
              <h3 className="text-sm font-bold uppercase tracking-wide text-ink">{t.name}</h3>
              <p className="text-xs text-ink-mute mt-1">{t.sub}</p>
              <p className="mt-5 flex items-end gap-1">
                <span className="nums text-3xl font-bold text-ink">{t.price}</span>
                {t.price.startsWith("$") && t.price !== "$0" && <span className="text-sm text-ink-mute mb-1">/mo</span>}
              </p>
              <ul className="mt-6 space-y-3 flex-1">
                {t.features.map((f) => (
                  <li key={f} className="flex items-start gap-2.5 text-sm text-ink-dim">
                    <Check className="w-4 h-4 text-amber mt-0.5 shrink-0" />
                    {f}
                  </li>
                ))}
              </ul>
              <Link href="/login" className={`mt-7 ${t.highlight ? "gf-btn-primary" : "gf-btn-secondary"} w-full`}>
                {t.cta}
              </Link>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

/* ------------------------------ closing CTA ----------------------------- */
function ClosingCTA() {
  return (
    <section className="relative overflow-hidden border-b border-line">
      <div className="absolute inset-0 amber-wash" aria-hidden />
      <div className="relative mx-auto max-w-3xl px-5 py-24 text-center">
        <h2 className="text-3xl sm:text-4xl font-bold tracking-tight">
          Get the floor under control.
        </h2>
        <p className="mt-4 text-lg text-ink-dim">
          Set up your shop in minutes. Bring your bays, your techs, and your backlog.
        </p>
        <div className="mt-8 flex justify-center">
          <Link href="/login" className="gf-btn-primary">
            Start free <ArrowRight className="w-4 h-4" />
          </Link>
        </div>
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="bg-base">
      <div className="mx-auto max-w-6xl px-5 py-10 flex flex-col sm:flex-row items-center justify-between gap-4">
        <div className="flex items-center gap-2.5">
          <span className="w-7 h-7 rounded-lg bg-amber flex items-center justify-center">
            <Wrench className="w-3.5 h-3.5 text-black" strokeWidth={2.5} />
          </span>
          <span className="text-sm font-bold">
            Garage<span className="text-amber">Flow</span>
          </span>
        </div>
        <p className="text-xs text-ink-mute">© 2026 GarageFlow. Built for the bay.</p>
      </div>
    </footer>
  );
}

export default function MarketingHome() {
  const { user, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && user) router.replace("/dashboard");
  }, [user, loading, router]);

  return (
    <main className="bg-base text-ink">
      <TopNav />
      <Hero />
      <StatBand />
      <Features />
      <Workflow />
      <Pricing />
      <ClosingCTA />
      <Footer />
    </main>
  );
}
