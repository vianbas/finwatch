/* eslint-disable react-refresh/only-export-components -- test helpers, not app modules */
import { vi } from "vitest";
import { render } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement, ReactNode } from "react";
import { AuthProvider } from "@/app/AuthContext";
import type { Alert, Transaction } from "@/lib/apiTypes";

// Synthetic fixtures only.
export function makeAlert(n: number, overrides: Partial<Alert> = {}): Alert {
  const id = String(n).padStart(12, "0");
  return {
    id: `a0000000-0000-4000-8000-${id}`,
    ruleId: `b0000000-0000-4000-8000-${id}`,
    transactionId: `c0000000-0000-4000-8000-${id}`,
    status: "open",
    severity: "low",
    createdAt: "2026-01-02T03:04:05Z",
    updatedAt: "2026-01-02T03:05:06Z",
    ...overrides,
  };
}

export function makeTransaction(n: number, overrides: Partial<Transaction> = {}): Transaction {
  return {
    id: `d0000000-0000-4000-8000-${String(n).padStart(12, "0")}`,
    accountId: `ACC-${1000 + n}`,
    amountMinor: 1234,
    currency: "USD",
    direction: "debit",
    status: "posted",
    occurredAt: "2026-01-02T03:04:05Z",
    ...overrides,
  };
}

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

export function errorResponse(status: number, code: string, message: string): Response {
  return jsonResponse({ error: { code, message } }, status);
}

export type ApiHandler = (url: URL, init: RequestInit | undefined) => Response | Promise<Response>;

/** stubApi replaces global fetch with a handler that receives parsed URLs. */
export function stubApi(handler: ApiHandler) {
  const mock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) =>
    handler(new URL(String(input)), init),
  );
  vi.stubGlobal("fetch", mock);
  return mock;
}

/** requestedUrls lists the URLs fetch was called with, in order. */
export function requestedUrls(mock: ReturnType<typeof stubApi>): URL[] {
  return mock.mock.calls.map(([input]) => new URL(String(input)));
}

export function makeQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

/** wrapper provides auth + query client for hook tests. */
export function hookWrapper(client = makeQueryClient()) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <AuthProvider>
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      </AuthProvider>
    );
  };
}

function LocationProbe() {
  const { pathname, search } = useLocation();
  return <div data-testid="location">{`${pathname}${search}`}</div>;
}

/** renderPage mounts a page at a path with auth, query client, router and a location probe. */
export function renderPage(ui: ReactElement, path = "/") {
  const client = makeQueryClient();
  return render(
    <AuthProvider>
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[path]}>
          {ui}
          <LocationProbe />
        </MemoryRouter>
      </QueryClientProvider>
    </AuthProvider>,
  );
}
