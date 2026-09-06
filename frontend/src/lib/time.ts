export function formatDateTime(value?: string | null) {
  if (!value) return "N/D";
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return "N/D";
  return new Intl.DateTimeFormat("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(date);
}

export function ageLabel(value?: string | null, now = Date.now()) {
  if (!value) return "";
  const elapsed = Math.max(0, now - Date.parse(value));
  if (!Number.isFinite(elapsed)) return "";
  if (elapsed < 60_000) return `${Math.floor(elapsed / 1000)} s`;
  if (elapsed < 3_600_000) return `${Math.floor(elapsed / 60_000)} min`;
  return `${Math.floor(elapsed / 3_600_000)} h`;
}
