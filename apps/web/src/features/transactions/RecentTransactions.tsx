import { EmptyState, ErrorState, LoadingState } from "@/components/DataStates";
import { useTransactions } from "@/features/transactions/useTransactions";
import { formatMoney } from "@/lib/money";
import { formatTimestamp } from "@/lib/time";

const RECENT_COUNT = 10;

/** RecentTransactions is a table of the most recent synthetic transactions. */
export function RecentTransactions() {
  const query = useTransactions(RECENT_COUNT);

  if (query.isPending) return <LoadingState label="Loading transactions…" />;
  if (query.isError) {
    return (
      <ErrorState
        title="Could not load transactions"
        error={query.error}
        onRetry={() => void query.refetch()}
      />
    );
  }
  if (query.data.items.length === 0) return <EmptyState>No transactions yet.</EmptyState>;

  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-left text-sm">
        <caption className="sr-only">Most recent transactions</caption>
        <thead className="border-b border-border bg-muted/50">
          <tr>
            <th scope="col" className="px-4 py-2 font-medium">Occurred</th>
            <th scope="col" className="px-4 py-2 font-medium">Account</th>
            <th scope="col" className="px-4 py-2 font-medium">Direction</th>
            <th scope="col" className="px-4 py-2 text-right font-medium">Amount</th>
          </tr>
        </thead>
        <tbody>
          {query.data.items.map((tx) => (
            <tr key={tx.id} className="border-b border-border last:border-0">
              <td className="whitespace-nowrap px-4 py-2">
                <time dateTime={tx.occurredAt}>{formatTimestamp(tx.occurredAt)}</time>
              </td>
              <td className="px-4 py-2">{tx.accountId}</td>
              <td className="px-4 py-2 capitalize">{tx.direction}</td>
              <td className="whitespace-nowrap px-4 py-2 text-right tabular-nums">
                {formatMoney(tx.amountMinor, tx.currency)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
