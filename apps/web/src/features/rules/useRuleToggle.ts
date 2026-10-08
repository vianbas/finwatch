import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useApiFetch } from "@/app/useApiFetch";
import { requestJson, type ApiError } from "@/lib/apiError";
import type { Rule } from "@/lib/apiTypes";
import { queryKeys } from "@/lib/queryKeys";

/** toggleFailureMessage is the non-fatal inline message for a failed enable/disable. */
export function toggleFailureMessage(error: ApiError): string {
  if (error.status === 403) return "Your role can't change rules.";
  if (error.status === 404) return "This rule no longer exists. The list has been refreshed.";
  return `Could not update the rule: ${error.message}`;
}

interface ToggleVars {
  id: string;
  enabled: boolean;
}

/**
 * useRuleToggle enables or disables a rule (admin only on the server). The
 * rules query is invalidated whether the call succeeds or fails: a 404 means
 * the list was stale, and the refetch brings it back in line with the server.
 */
export function useRuleToggle(options: { onError?: (error: ApiError) => void } = {}) {
  const apiFetch = useApiFetch();
  const queryClient = useQueryClient();
  const { onError } = options;

  return useMutation<Rule, ApiError, ToggleVars>({
    mutationFn: ({ id, enabled }) =>
      requestJson<Rule>(apiFetch, `/rules/${encodeURIComponent(id)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled }),
      }),
    onError: (error) => onError?.(error),
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.rules }),
  });
}
