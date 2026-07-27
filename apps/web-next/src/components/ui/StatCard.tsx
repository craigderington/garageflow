import { Card } from "./Card";

const accents: Record<string, { chip: string; bar: string }> = {
  amber: { chip: "bg-amber-400/10 text-amber-300 ring-amber-400/20", bar: "bg-amber-400" },
  blue: { chip: "bg-blue-400/10 text-blue-300 ring-blue-400/20", bar: "bg-blue-400" },
  emerald: { chip: "bg-emerald-400/10 text-emerald-300 ring-emerald-400/20", bar: "bg-emerald-400" },
  indigo: { chip: "bg-indigo-400/10 text-indigo-300 ring-indigo-400/20", bar: "bg-indigo-400" },
  violet: { chip: "bg-violet-400/10 text-violet-300 ring-violet-400/20", bar: "bg-violet-400" },
  rose: { chip: "bg-rose-400/10 text-rose-300 ring-rose-400/20", bar: "bg-rose-400" },
};

export function StatCard({
  label,
  value,
  sub,
  icon,
  trend,
  color = "amber",
}: {
  label: string;
  value: string | number;
  sub?: string;
  icon: React.ReactNode;
  trend?: { up: boolean; text: string };
  color?: "blue" | "amber" | "emerald" | "indigo" | "violet" | "rose";
}) {
  const a = accents[color] ?? accents.amber;

  return (
    <Card className="relative overflow-hidden">
      {/* left accent rail */}
      <div className={`absolute left-0 top-0 h-full w-[3px] ${a.bar} opacity-70`} />
      <div className="p-5 pl-6">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="gf-eyebrow">{label}</p>
            <p className="nums text-3xl font-bold text-ink mt-2 leading-none">{value}</p>
            {sub && <p className="text-xs text-ink-mute mt-2 truncate">{sub}</p>}
            {trend && (
              <p className={`text-xs mt-2 font-medium ${trend.up ? "text-emerald-400" : "text-rose-400"}`}>
                {trend.up ? "▲" : "▼"} {trend.text}
              </p>
            )}
          </div>
          <div className={`shrink-0 p-2.5 rounded-lg ring-1 ${a.chip}`}>{icon}</div>
        </div>
      </div>
    </Card>
  );
}
