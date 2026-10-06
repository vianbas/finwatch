// Types mirroring the schemas in contracts/openapi.yaml. Keep them aligned.

export type AlertStatus = "open" | "acknowledged" | "resolved";
export type AlertSeverity = "low" | "medium" | "high" | "critical";

export const ALERT_STATUSES: readonly AlertStatus[] = ["open", "acknowledged", "resolved"];
export const ALERT_SEVERITIES: readonly AlertSeverity[] = ["low", "medium", "high", "critical"];

export interface Transaction {
  id: string;
  accountId: string;
  /** Integer minor units (e.g. cents). Always positive. */
  amountMinor: number;
  /** ISO 4217 currency code. */
  currency: string;
  direction: "debit" | "credit";
  status: "posted";
  /** RFC 3339 (UTC). */
  occurredAt: string;
}

export interface Alert {
  id: string;
  ruleId: string;
  transactionId: string;
  status: AlertStatus;
  severity: AlertSeverity;
  /** RFC 3339 (UTC). */
  createdAt: string;
  /** RFC 3339 (UTC). */
  updatedAt: string;
}

/** Cursor-paginated response shared by TransactionPage and AlertPage. */
export interface Page<T> {
  items: T[];
  nextCursor: string | null;
}
