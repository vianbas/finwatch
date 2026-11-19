import { AlertSummary } from "@/features/alerts/AlertSummary";
import { RecentTransactions } from "@/features/transactions/RecentTransactions";

export function DashboardPage() {
  return (
    <section aria-labelledby="page-heading" className="space-y-6">
      <div className="space-y-1">
        <h1 id="page-heading" className="text-2xl font-semibold tracking-tight">
          Dashboard
        </h1>
        <p className="text-muted-foreground">
          Operational overview of synthetic transaction monitoring.
        </p>
      </div>

      <AlertSummary />

      <section aria-labelledby="recent-tx-heading" className="space-y-3">
        <h2 id="recent-tx-heading" className="text-lg font-semibold">Recent transactions</h2>
        <RecentTransactions />
      </section>
    </section>
  );
}
