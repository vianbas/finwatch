import { cn } from "@/lib/utils";
import type { RealtimeStatus } from "@/app/useRealtimeEvents";

const labels: Record<RealtimeStatus, string> = {
  connecting: "Connecting",
  live: "Live",
  offline: "Offline",
};

const dotClasses: Record<RealtimeStatus, string> = {
  connecting: "bg-yellow-500 motion-safe:animate-pulse",
  live: "bg-green-600",
  offline: "bg-red-600",
};

/**
 * ConnectionStatus announces the realtime connection state. The text label
 * carries the meaning; the dot colour is decorative reinforcement only.
 */
export function ConnectionStatus({ status }: { status: RealtimeStatus }) {
  return (
    <div
      role="status"
      aria-live="polite"
      className="flex items-center gap-2 text-sm text-muted-foreground"
    >
      <span
        aria-hidden="true"
        className={cn("h-2 w-2 rounded-full", dotClasses[status])}
      />
      <span>
        <span className="sr-only">Realtime updates: </span>
        {labels[status]}
      </span>
    </div>
  );
}
