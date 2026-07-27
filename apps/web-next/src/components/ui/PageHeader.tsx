import type { ReactNode } from "react";

export function PageHeader({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-end justify-between gap-4">
      <div className="flex items-center gap-3">
        <span className="h-9 w-1 rounded-full bg-amber shadow-[0_0_12px_-2px_var(--color-amber)]" />
        <div>
          <h1 className="text-xl font-bold tracking-tight text-ink">{title}</h1>
          {subtitle && <p className="text-sm text-ink-dim mt-0.5">{subtitle}</p>}
        </div>
      </div>
      {action}
    </div>
  );
}
