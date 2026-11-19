import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useApiFetch } from "@/app/useApiFetch";
import { requestJson } from "@/lib/apiError";
import type { Alert, AlertStatus, Page } from "@/lib/apiTypes";
import { queryKeys } from "@/lib/queryKeys";

export const ALERTS_PAGE_SIZE = 50;

/** alertsPath builds the GET /alerts URL; omitted filters are not sent. */
function alertsPath(status: AlertStatus | undefined, limit: number, cursor?: string): string {
  const params = new URLSearchParams();
  if (status) params.set("status", status);
  params.set("limit", String(limit));
  if (cursor) params.set("cursor", cursor);
  return `/alerts?${params.toString()}`;
}

/**
 * useAlerts lists alerts, most-recent first, with cursor ("Load more")
 * pagination. Keys extend queryKeys.alerts so realtime events refresh them.
 */
export function useAlerts(status?: AlertStatus, limit = ALERTS_PAGE_SIZE) {
  const apiFetch = useApiFetch();
  return useInfiniteQuery({
    queryKey: [...queryKeys.alerts, "list", { status: status ?? null, limit }],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      requestJson<Page<Alert>>(apiFetch, alertsPath(status, limit, pageParam)),
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  });
}

/** useAlertPage fetches a single page of alerts (no pagination), e.g. for dashboard counts. */
export function useAlertPage(status: AlertStatus, limit: number) {
  const apiFetch = useApiFetch();
  return useQuery({
    queryKey: [...queryKeys.alerts, "page", { status, limit }],
    queryFn: () => requestJson<Page<Alert>>(apiFetch, alertsPath(status, limit)),
  });
}
