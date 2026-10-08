import { useQuery } from "@tanstack/react-query";
import { useApiFetch } from "@/app/useApiFetch";
import { requestJson } from "@/lib/apiError";
import type { Rule } from "@/lib/apiTypes";
import { queryKeys } from "@/lib/queryKeys";

/** useRules lists every rule, enabled or not. The rule set is small and unpaginated. */
export function useRules() {
  const apiFetch = useApiFetch();
  return useQuery({
    queryKey: [...queryKeys.rules],
    queryFn: async () => (await requestJson<{ items: Rule[] }>(apiFetch, "/rules")).items,
  });
}
