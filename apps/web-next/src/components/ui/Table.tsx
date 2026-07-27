import React, { type ReactNode } from "react";
import { Card } from "./Card";

export function Table({
  headers,
  children,
  empty,
}: {
  headers: string[];
  children: ReactNode;
  empty?: string;
}) {
  const hasData = React.Children.count(children) > 0;
  return (
    <Card>
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr>
              {headers.map((h, i) => (
                <th key={i} className="gf-th">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-line">{children}</tbody>
        </table>
      </div>
      {!hasData && (
        <div className="px-5 py-12 text-center text-sm text-ink-mute">{empty || "No data"}</div>
      )}
    </Card>
  );
}

export function Td({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <td className={`px-5 py-3.5 text-sm ${className}`}>{children}</td>;
}
