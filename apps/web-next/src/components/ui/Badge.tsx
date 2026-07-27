// Status palette tuned for dark steel surfaces.
// Each entry: translucent fill + bright text + faint border.
const statusStyles: Record<string, string> = {
  created: "bg-zinc-400/10 text-zinc-300 border-zinc-400/20",
  diagnosed: "bg-sky-400/10 text-sky-300 border-sky-400/25",
  estimate_sent: "bg-amber-400/10 text-amber-300 border-amber-400/25",
  approved: "bg-emerald-400/10 text-emerald-300 border-emerald-400/25",
  paid: "bg-emerald-500/15 text-emerald-300 border-emerald-500/30",
  in_progress: "bg-blue-400/10 text-blue-300 border-blue-400/25",
  completed: "bg-emerald-400/10 text-emerald-300 border-emerald-400/25",
  invoiced: "bg-violet-400/10 text-violet-300 border-violet-400/25",
  closed: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  draft: "bg-zinc-400/10 text-zinc-300 border-zinc-400/20",
  sent: "bg-amber-400/10 text-amber-300 border-amber-400/25",
  active: "bg-emerald-400/10 text-emerald-300 border-emerald-400/25",
  inactive: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  low: "bg-rose-500/10 text-rose-300 border-rose-500/25",
  ok: "bg-emerald-400/10 text-emerald-300 border-emerald-400/25",
};

export function Badge({
  status,
  children,
  className = "",
  ...rest
}: { status?: string; children: React.ReactNode; className?: string } & React.HTMLAttributes<HTMLSpanElement>) {
  const key = status?.toLowerCase().replace(/\s+/g, "_") || "";
  const style = statusStyles[key] || "bg-zinc-400/10 text-zinc-300 border-zinc-400/20";
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-md text-[11px] font-semibold uppercase tracking-wide border ${style} ${className}`}
      {...rest}
    >
      {children}
    </span>
  );
}
