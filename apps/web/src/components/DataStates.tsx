import { Button } from "@/components/ui/button";
import { errorMessage } from "@/lib/apiError";

/** LoadingState is a polite live region shown while a view's data is loading. */
export function LoadingState({ label }: { label: string }) {
  return (
    <p role="status" className="py-6 text-sm text-muted-foreground">
      {label}
    </p>
  );
}

/** EmptyState explains that a successful request returned nothing to show. */
export function EmptyState({ children }: { children: string }) {
  return <p className="py-6 text-sm text-muted-foreground">{children}</p>;
}

/** ErrorState announces a failed request with a display-safe message and a retry. */
export function ErrorState({
  title,
  error,
  onRetry,
}: {
  title: string;
  error: unknown;
  onRetry: () => void;
}) {
  return (
    <div role="alert" className="space-y-2 rounded-md border border-border p-4 text-sm">
      <p className="font-medium">{title}</p>
      <p className="text-muted-foreground">{errorMessage(error)}</p>
      <Button type="button" variant="outline" size="sm" onClick={onRetry}>
        Retry
      </Button>
    </div>
  );
}
