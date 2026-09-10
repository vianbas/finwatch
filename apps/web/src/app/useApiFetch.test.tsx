import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { AuthProvider, useAuth } from "@/app/AuthContext";
import { useApiFetch } from "@/app/useApiFetch";
import type { ReactNode } from "react";

function wrapper({ children }: { children: ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>;
}

describe("useApiFetch", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("attaches the bearer token when one is present", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true, status: 200, json: async () => ({}) });
    const { result } = renderHook(
      () => ({ auth: useAuth(), apiFetch: useApiFetch() }),
      { wrapper },
    );

    (fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ accessToken: "token-123", user: { id: "u1", email: "a@example.com", role: "operator" } }),
    });
    await act(async () => {
      await result.current.auth.login("a@example.com", "correct-password");
    });

    await act(async () => {
      await result.current.apiFetch("/transactions");
    });

    const lastCall = (fetch as ReturnType<typeof vi.fn>).mock.calls.at(-1);
    expect(new Headers(lastCall?.[1]?.headers).get("Authorization")).toBe("Bearer token-123");
  });

  it("preserves caller-supplied headers alongside the bearer token", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true, status: 200, json: async () => ({}) });
    const { result } = renderHook(
      () => ({ auth: useAuth(), apiFetch: useApiFetch() }),
      { wrapper },
    );

    (fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ accessToken: "token-123", user: { id: "u1", email: "a@example.com", role: "operator" } }),
    });
    await act(async () => {
      await result.current.auth.login("a@example.com", "correct-password");
    });

    await act(async () => {
      await result.current.apiFetch("/transactions", { headers: new Headers({ "X-Test": "1" }) });
    });

    const lastCall = (fetch as ReturnType<typeof vi.fn>).mock.calls.at(-1);
    const headers = new Headers(lastCall?.[1]?.headers);
    expect(headers.get("Authorization")).toBe("Bearer token-123");
    expect(headers.get("X-Test")).toBe("1");
  });

  it("clears the token when a request returns 401", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ accessToken: "token-123", user: { id: "u1", email: "a@example.com", role: "operator" } }),
    });
    const { result } = renderHook(
      () => ({ auth: useAuth(), apiFetch: useApiFetch() }),
      { wrapper },
    );
    await act(async () => {
      await result.current.auth.login("a@example.com", "correct-password");
    });
    expect(result.current.auth.accessToken).toBe("token-123");

    (fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: false, status: 401, json: async () => ({}) });
    await act(async () => {
      await result.current.apiFetch("/transactions");
    });

    expect(result.current.auth.accessToken).toBeNull();
  });
});
