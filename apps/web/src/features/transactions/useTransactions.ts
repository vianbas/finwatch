import { useQuery } from "@tanstack/react-query";
import { useApiFetch } from "@/app/useApiFetch";
import { requestJson } from "@/lib/apiError";
import type { Page, Transaction } from "@/lib/apiTypes";
import { queryKeys } from "@/lib/queryKeys";

/** useTransactions fetches the most recent page of transactions (most-recent first). */
export function useTransactions(limit = 10) {
  const apiFetch = useApiFetch();
  return useQuery({
    queryKey: [...queryKeys.transactions, { limit }],
    queryFn: () => requestJson<Page<Transaction>>(apiFetch, `/transactions?limit=${limit}`),
  });
}
