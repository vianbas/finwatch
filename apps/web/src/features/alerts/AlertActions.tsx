import { Button } from "@/components/ui/button";
import type { Alert } from "@/lib/apiTypes";
import {
  transitionFailureMessage,
  useAlertTransition,
  type AlertTransition,
} from "@/features/alerts/useAlertTransition";

const labels: Record<AlertTransition, string> = {
  acknowledge: "Acknowledge",
  resolve: "Resolve",
};

interface AlertActionsProps {
  alert: Alert;
  /** Called with a display message when a transition fails. */
  onFailure: (message: string) => void;
}

/**
 * AlertActions shows the one transition valid for the alert's status:
 * Acknowledge for open alerts, Resolve for acknowledged ones, nothing otherwise.
 */
export function AlertActions({ alert, onFailure }: AlertActionsProps) {
  const transition = useAlertTransition({
    onError: (error, vars) => onFailure(transitionFailureMessage(error, vars.action)),
  });

  const action: AlertTransition | null =
    alert.status === "open" ? "acknowledge" : alert.status === "acknowledged" ? "resolve" : null;
  if (!action) {
    return (
      <span className="text-muted-foreground">
        <span aria-hidden="true">—</span>
        <span className="sr-only">No actions available</span>
      </span>
    );
  }

  return (
    <Button
      type="button"
      size="sm"
      variant="outline"
      disabled={transition.isPending}
      aria-label={`${labels[action]} alert ${alert.id.slice(0, 8)}`}
      onClick={() => transition.mutate({ id: alert.id, action })}
    >
      {transition.isPending ? "Working…" : labels[action]}
    </Button>
  );
}
