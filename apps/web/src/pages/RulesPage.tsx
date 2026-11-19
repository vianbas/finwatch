import { useState } from "react";
import { useAuth } from "@/app/AuthContext";
import { Button } from "@/components/ui/button";
import { EmptyState, ErrorState, LoadingState } from "@/components/DataStates";
import { describeCondition } from "@/features/rules/condition";
import { RuleToggle } from "@/features/rules/RuleToggle";
import { useRules } from "@/features/rules/useRules";
import { formatTimestamp } from "@/lib/time";

export function RulesPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  const [failure, setFailure] = useState<string | null>(null);
  const rules = useRules();

  return (
    <section aria-labelledby="page-heading" className="space-y-4">
      <div className="space-y-1">
        <h1 id="page-heading" className="text-2xl font-semibold tracking-tight">
          Rules
        </h1>
        <p className="text-muted-foreground">Monitoring rules applied to synthetic transactions.</p>
        {!isAdmin && (
          <p className="text-sm text-muted-foreground">Only admins can enable or disable rules.</p>
        )}
      </div>

      {failure && (
        <div role="alert" className="flex items-start justify-between gap-4 rounded-md border border-border p-3 text-sm">
          <p>{failure}</p>
          <Button type="button" variant="ghost" size="sm" onClick={() => setFailure(null)}>
            Dismiss
          </Button>
        </div>
      )}

      {rules.isPending ? (
        <LoadingState label="Loading rules…" />
      ) : rules.isError ? (
        <ErrorState
          title="Could not load rules"
          error={rules.error}
          onRetry={() => void rules.refetch()}
        />
      ) : rules.data.length === 0 ? (
        <EmptyState>No rules configured.</EmptyState>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="w-full text-left text-sm">
            <caption className="sr-only">Monitoring rules, oldest first</caption>
            <thead className="border-b border-border bg-muted/50">
              <tr>
                <th scope="col" className="px-4 py-2 font-medium">Name</th>
                <th scope="col" className="px-4 py-2 font-medium">Condition</th>
                <th scope="col" className="px-4 py-2 font-medium">Severity</th>
                <th scope="col" className="px-4 py-2 font-medium">Status</th>
                <th scope="col" className="px-4 py-2 font-medium">Created</th>
                {isAdmin && <th scope="col" className="px-4 py-2 font-medium">Actions</th>}
              </tr>
            </thead>
            <tbody>
              {rules.data.map((rule) => (
                <tr key={rule.id} className="border-b border-border last:border-0">
                  <td className="px-4 py-2">{rule.name}</td>
                  <td className="px-4 py-2">{describeCondition(rule.field, rule.operator, rule.value)}</td>
                  <td className="px-4 py-2 capitalize">{rule.severity}</td>
                  <td className="px-4 py-2">{rule.enabled ? "Enabled" : "Disabled"}</td>
                  <td className="whitespace-nowrap px-4 py-2">
                    <time dateTime={rule.createdAt}>{formatTimestamp(rule.createdAt)}</time>
                  </td>
                  {isAdmin && (
                    <td className="px-4 py-2">
                      <RuleToggle rule={rule} onFailure={setFailure} />
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
