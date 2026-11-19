import { type Alert, type AlertSeverity, type Page } from "@/lib/apiTypes";

/** SUMMARY_LIMIT is the largest page the API allows; more than this shows as "200+". */
export const SUMMARY_LIMIT = 200;

export type SeverityCounts = Record<AlertSeverity, number>;

/** countBySeverity always returns all four severities, zero-filled. */
export function countBySeverity(alerts: Alert[]): SeverityCounts {
  const counts: SeverityCounts = { low: 0, medium: 0, high: 0, critical: 0 };
  for (const a of alerts) counts[a.severity] += 1;
  return counts;
}

/** formatCount shows an exact count, or "200+" when more pages exist. */
export function formatCount(page: Page<Alert>): string {
  return page.nextCursor !== null ? `${SUMMARY_LIMIT}+` : String(page.items.length);
}
