import type { ReactNode } from "react";

export function StatusBadge({ tone, children }: { tone: string; children: ReactNode }) {
  return <span className={`status-badge status-${tone}`}>{children}</span>;
}
