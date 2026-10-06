import { ErrorState, LoadingState } from "@/components/DataStates";
import type { UseQueryResult } from "@tanstack/react-query";
import { SeverityChart } from "@/features/alerts/SeverityChart";
import { SUMMARY_LIMIT, countBySeverity, formatCount } from "@/features/alerts/summary";
import { useAlertPage } from "@/features/alerts/useAlerts";
import type { Alert, Page } from "@/lib/apiTypes";

function Tile({ label, query }: { label: string; query: UseQueryResult<Page<Alert>> }) {
  return (
    <section aria-label={label} className="rounded-lg border border-border p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      {query.isPending ? (
        <p role="status" className="text-sm text-muted-foreground">Loading…</p>
      ) : query.isError ? (
        <p role="alert" className="text-sm">Unavailable</p>
      ) : (
        <p className="text-3xl font-semibold">{formatCount(query.data)}</p>
      )}
    </section>
  );
}

/** AlertSummary is the dashboard's alert tiles plus the open-alerts-by-severity chart. */
export function AlertSummary() {
  const open = useAlertPage("open", SUMMARY_LIMIT);
  const acknowledged = useAlertPage("acknowledged", SUMMARY_LIMIT);

  return (
    <>
      <div className="grid gap-4 sm:grid-cols-2">
        <Tile label="Open alerts" query={open} />
        <Tile label="Acknowledged alerts" query={acknowledged} />
      </div>

      <section aria-labelledby="severity-heading" className="space-y-3">
        <h2 id="severity-heading" className="text-lg font-semibold">Open alerts by severity</h2>
        {open.isPending ? (
          <LoadingState label="Loading severity breakdown…" />
        ) : open.isError ? (
          <ErrorState
            title="Could not load open alerts"
            error={open.error}
            onRetry={() => void open.refetch()}
          />
        ) : (
          <SeverityChart
            counts={countBySeverity(open.data.items)}
            truncated={open.data.nextCursor !== null}
          />
        )}
      </section>
    </>
  );
}
