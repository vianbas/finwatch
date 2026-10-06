import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApiFetch } from "@/app/useApiFetch";
import { requestJson, type ApiError } from "@/lib/apiError";
import type { Alert } from "@/lib/apiTypes";
import { queryKeys } from "@/lib/queryKeys";

export type AlertTransition = "acknowledge" | "resolve";

/** transitionFailureMessage is the non-fatal inline message for a failed transition. */
export function transitionFailureMessage(error: ApiError, action: AlertTransition): string {
  if (error.status === 409 && error.code === "INVALID_TRANSITION") {
    const past = action === "acknowledge" ? "acknowledged" : "resolved";
    return `This alert can no longer be ${past}: its status has changed. The list has been refreshed.`;
  }
  return `Could not ${action} the alert: ${error.message}`;
}

interface TransitionVars {
  id: string;
  action: AlertTransition;
}

/**
 * useAlertTransition acknowledges or resolves an alert. The alerts queries
 * are invalidated whether the call succeeds or fails: a 409 means the list
 * was stale, and the refetch brings it back in line with the server.
 */
export function useAlertTransition(options: { onError?: (error: ApiError, vars: TransitionVars) => void } = {}) {
  const apiFetch = useApiFetch();
  const queryClient = useQueryClient();
  const { onError } = options;

  return useMutation<Alert, ApiError, TransitionVars>({
    mutationFn: ({ id, action }) =>
      requestJson<Alert>(apiFetch, `/alerts/${encodeURIComponent(id)}/${action}`, {
        method: "POST",
      }),
    onError: (error, vars) => onError?.(error, vars),
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.alerts }),
  });
}
