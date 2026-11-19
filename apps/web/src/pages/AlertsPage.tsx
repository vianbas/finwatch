import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { EmptyState, ErrorState, LoadingState } from "@/components/DataStates";
import { AlertActions } from "@/features/alerts/AlertActions";
import { useAlerts } from "@/features/alerts/useAlerts";
import { ALERT_STATUSES, type AlertStatus } from "@/lib/apiTypes";
import { formatTimestamp } from "@/lib/time";

/** parseStatus accepts only contract status values from the URL; anything else means "all". */
function parseStatus(raw: string | null): AlertStatus | undefined {
  return ALERT_STATUSES.find((s) => s === raw);
}

export function AlertsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const status = parseStatus(searchParams.get("status"));
  const [failure, setFailure] = useState<string | null>(null);

  const alerts = useAlerts(status);
  const items = alerts.data?.pages.flatMap((p) => p.items) ?? [];

  function onStatusChange(value: string) {
    setFailure(null);
    setSearchParams(parseStatus(value) ? { status: value } : {});
  }

  return (
    <section aria-labelledby="page-heading" className="space-y-4">
      <div className="space-y-1">
        <h1 id="page-heading" className="text-2xl font-semibold tracking-tight">
          Alerts
        </h1>
        <p className="text-muted-foreground">Operational alerts raised from synthetic activity.</p>
      </div>

      <div className="flex items-center gap-2">
        <label htmlFor="alert-status" className="text-sm font-medium">
          Status
        </label>
        <select
          id="alert-status"
          value={status ?? ""}
          onChange={(e) => onStatusChange(e.target.value)}
          className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        >
          <option value="">All</option>
          {ALERT_STATUSES.map((s) => (
            <option key={s} value={s}>
              {s.charAt(0).toUpperCase() + s.slice(1)}
            </option>
          ))}
        </select>
      </div>

      {failure && (
        <div role="alert" className="flex items-start justify-between gap-4 rounded-md border border-border p-3 text-sm">
          <p>{failure}</p>
          <Button type="button" variant="ghost" size="sm" onClick={() => setFailure(null)}>
            Dismiss
          </Button>
        </div>
      )}

      {alerts.isPending ? (
        <LoadingState label="Loading alerts…" />
      ) : alerts.isError ? (
        <ErrorState
          title="Could not load alerts"
          error={alerts.error}
          onRetry={() => void alerts.refetch()}
        />
      ) : items.length === 0 ? (
        <EmptyState>
          {status ? `No ${status} alerts.` : "No alerts yet."}
        </EmptyState>
      ) : (
        <>
          <div className="overflow-x-auto rounded-lg border border-border">
            <table className="w-full text-left text-sm">
              <caption className="sr-only">
                {status ? `Alerts with status ${status}` : "All alerts"}, most recent first
              </caption>
              <thead className="border-b border-border bg-muted/50">
                <tr>
                  <th scope="col" className="px-4 py-2 font-medium">Severity</th>
                  <th scope="col" className="px-4 py-2 font-medium">Status</th>
                  <th scope="col" className="px-4 py-2 font-medium">Created</th>
                  <th scope="col" className="px-4 py-2 font-medium">Updated</th>
                  <th scope="col" className="px-4 py-2 font-medium">Transaction</th>
                  <th scope="col" className="px-4 py-2 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((alert) => (
                  <tr key={alert.id} className="border-b border-border last:border-0">
                    <td className="px-4 py-2 capitalize">{alert.severity}</td>
                    <td className="px-4 py-2 capitalize">{alert.status}</td>
                    <td className="whitespace-nowrap px-4 py-2">
                      <time dateTime={alert.createdAt}>{formatTimestamp(alert.createdAt)}</time>
                    </td>
                    <td className="whitespace-nowrap px-4 py-2">
                      <time dateTime={alert.updatedAt}>{formatTimestamp(alert.updatedAt)}</time>
                    </td>
                    <td className="px-4 py-2 font-mono text-xs">{alert.transactionId}</td>
                    <td className="px-4 py-2">
                      <AlertActions alert={alert} onFailure={setFailure} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {alerts.hasNextPage && (
            <Button
              type="button"
              variant="outline"
              disabled={alerts.isFetchingNextPage}
              onClick={() => void alerts.fetchNextPage()}
            >
              {alerts.isFetchingNextPage ? "Loading…" : "Load more"}
            </Button>
          )}
          {alerts.isFetchNextPageError && (
            <p role="alert" className="text-sm text-muted-foreground">
              Could not load more alerts. Try again.
            </p>
          )}
        </>
      )}
    </section>
  );
}
