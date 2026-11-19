import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { useAlertPage, useAlerts } from "@/features/alerts/useAlerts";
import { useAlertTransition } from "@/features/alerts/useAlertTransition";
import { useTransactions } from "@/features/transactions/useTransactions";
import { ApiError } from "@/lib/apiError";
import { queryKeys } from "@/lib/queryKeys";
import {
  errorResponse,
  hookWrapper,
  jsonResponse,
  makeAlert,
  makeQueryClient,
  makeTransaction,
  requestedUrls,
  stubApi,
} from "@/test/api";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useAlerts", () => {
  it("requests all alerts with the default page size and no status param", async () => {
    const api = stubApi(() => jsonResponse({ items: [makeAlert(1)], nextCursor: null }));
    const { result } = renderHook(() => useAlerts(), { wrapper: hookWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const url = requestedUrls(api)[0];
    expect(url.pathname).toBe("/alerts");
    expect(url.searchParams.get("status")).toBeNull();
    expect(url.searchParams.get("limit")).toBe("50");
    expect(url.searchParams.get("cursor")).toBeNull();
    expect(result.current.hasNextPage).toBe(false);
  });

  it("sends the status filter and follows nextCursor for the next page", async () => {
    const api = stubApi((url) =>
      url.searchParams.get("cursor") === "cur-2"
        ? jsonResponse({ items: [makeAlert(2)], nextCursor: null })
        : jsonResponse({ items: [makeAlert(1)], nextCursor: "cur-2" }),
    );
    const { result } = renderHook(() => useAlerts("open"), { wrapper: hookWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(requestedUrls(api)[0].searchParams.get("status")).toBe("open");
    expect(result.current.hasNextPage).toBe(true);

    await act(async () => {
      await result.current.fetchNextPage();
    });

    const second = requestedUrls(api)[1];
    expect(second.searchParams.get("status")).toBe("open");
    expect(second.searchParams.get("cursor")).toBe("cur-2");
    await waitFor(() => expect(result.current.data?.pages).toHaveLength(2));
    expect(result.current.data?.pages.flatMap((p) => p.items)).toHaveLength(2);
    expect(result.current.hasNextPage).toBe(false);
  });

  it("builds its key on the shared alerts prefix", async () => {
    stubApi(() => jsonResponse({ items: [], nextCursor: null }));
    const client = makeQueryClient();
    const { result } = renderHook(() => useAlerts("resolved"), { wrapper: hookWrapper(client) });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const keys = client.getQueryCache().findAll({ queryKey: queryKeys.alerts });
    expect(keys).toHaveLength(1);
  });

  it("surfaces the error envelope as a typed ApiError", async () => {
    stubApi(() => errorResponse(400, "INVALID_QUERY", "limit must be between 1 and 200"));
    const { result } = renderHook(() => useAlerts(), { wrapper: hookWrapper() });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(ApiError);
    expect((result.current.error as ApiError).code).toBe("INVALID_QUERY");
  });
});

describe("useAlertPage", () => {
  it("requests a single page with status and limit", async () => {
    const api = stubApi(() => jsonResponse({ items: [], nextCursor: null }));
    const { result } = renderHook(() => useAlertPage("acknowledged", 200), {
      wrapper: hookWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const url = requestedUrls(api)[0];
    expect(url.searchParams.get("status")).toBe("acknowledged");
    expect(url.searchParams.get("limit")).toBe("200");
  });
});

describe("useTransactions", () => {
  it("requests the given number of transactions", async () => {
    const api = stubApi(() => jsonResponse({ items: [makeTransaction(1)], nextCursor: null }));
    const client = makeQueryClient();
    const { result } = renderHook(() => useTransactions(10), { wrapper: hookWrapper(client) });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const url = requestedUrls(api)[0];
    expect(url.pathname).toBe("/transactions");
    expect(url.searchParams.get("limit")).toBe("10");
    expect(client.getQueryCache().findAll({ queryKey: queryKeys.transactions })).toHaveLength(1);
  });
});

describe("useAlertTransition", () => {
  it("POSTs the transition and invalidates the alerts queries on success", async () => {
    const alert = makeAlert(1, { status: "acknowledged" });
    const api = stubApi(() => jsonResponse(alert));
    const client = makeQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useAlertTransition(), { wrapper: hookWrapper(client) });

    await act(async () => {
      await result.current.mutateAsync({ id: alert.id, action: "acknowledge" });
    });

    const [input, init] = api.mock.calls[0];
    expect(new URL(String(input)).pathname).toBe(`/alerts/${alert.id}/acknowledge`);
    expect(init?.method).toBe("POST");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.alerts });
  });

  it("reports a 409 as a typed error and still invalidates", async () => {
    stubApi(() => errorResponse(409, "INVALID_TRANSITION", "not allowed"));
    const client = makeQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const onError = vi.fn();
    const { result } = renderHook(() => useAlertTransition({ onError }), {
      wrapper: hookWrapper(client),
    });

    await act(async () => {
      await result.current.mutateAsync({ id: "x", action: "resolve" }).catch(() => undefined);
    });

    expect(onError).toHaveBeenCalledTimes(1);
    const err = onError.mock.calls[0][0] as ApiError;
    expect(err.status).toBe(409);
    expect(err.code).toBe("INVALID_TRANSITION");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.alerts });
  });
});
