/* eslint-disable react-refresh/only-export-components -- test helpers, not app modules */
import { vi } from "vitest";
import { render } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, type ReactElement, type ReactNode } from "react";
import { AuthProvider } from "@/app/AuthContext";
import { useAuth, type AuthUser } from "@/app/AuthContext";
import type { Alert, Rule, Transaction } from "@/lib/apiTypes";

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

export function makeRule(n: number, overrides: Partial<Rule> = {}): Rule {
  return {
    id: `e0000000-0000-4000-8000-${String(n).padStart(12, "0")}`,
    name: `Example rule ${n}`,
    field: "amount_minor",
    operator: "gte",
    value: "500000",
    severity: "high",
    enabled: true,
    createdAt: "2026-01-02T03:04:05Z",
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

/**
 * stubApi replaces global fetch with a handler that receives parsed URLs.
 * POST /login is answered here (the role is the email's local part, see
 * renderPage) so tests can sign in as a given role.
 */
export function stubApi(handler: ApiHandler) {
  const mock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input));
    if (url.pathname === "/login" && init?.method === "POST") {
      const { email } = JSON.parse(String(init.body)) as { email: string };
      const role = email.split("@")[0] as AuthUser["role"];
      return jsonResponse({ accessToken: "test-token", user: { id: "u1", email, role } });
    }
    return handler(url, init);
  });
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

/** SignedInAs logs in with the stubbed /login and renders its children once signed in. */
function SignedInAs({ role, children }: { role: AuthUser["role"]; children: ReactNode }) {
  const { user, login } = useAuth();
  useEffect(() => {
    void login(`${role}@example.com`, "example-password");
  }, [login, role]);
  return user ? <>{children}</> : null;
}

/**
 * renderPage mounts a page at a path with auth, query client, router and a
 * location probe. Pass `role` to sign in first (requires stubApi to be set up).
 */
export function renderPage(ui: ReactElement, path = "/", options: { role?: AuthUser["role"] } = {}) {
  const client = makeQueryClient();
  const page = (
    <MemoryRouter initialEntries={[path]}>
      {ui}
      <LocationProbe />
    </MemoryRouter>
  );
  return render(
    <AuthProvider>
      <QueryClientProvider client={client}>
        {options.role ? <SignedInAs role={options.role}>{page}</SignedInAs> : page}
      </QueryClientProvider>
    </AuthProvider>,
  );
}
