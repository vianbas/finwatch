import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { AuthProvider, useAuth } from "@/app/AuthContext";
import { useRealtimeEvents } from "@/app/useRealtimeEvents";

class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: unknown }) => void) | null = null;
  onclose: ((ev: { code: number }) => void) | null = null;
  close = vi.fn();
  constructor(
    public url: string,
    public protocols: string[],
  ) {
    FakeWebSocket.instances.push(this);
  }
  open() {
    this.onopen?.();
  }
  message(data: unknown) {
    this.onmessage?.({ data });
  }
  closeWith(code: number) {
    this.onclose?.({ code });
  }
}

function envelope(id: string, type: string, extra: Record<string, unknown> = {}) {
  return JSON.stringify({
    id,
    type,
    version: 1,
    occurredAt: "2026-01-02T03:04:05Z",
    data: {},
    ...extra,
  });
}

const last = () => FakeWebSocket.instances[FakeWebSocket.instances.length - 1];

function setup() {
  const queryClient = new QueryClient();
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  function wrapper({ children }: { children: ReactNode }) {
    return (
      <AuthProvider>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </AuthProvider>
    );
  }
  const hook = renderHook(
    () => ({ auth: useAuth(), realtime: useRealtimeEvents() }),
    { wrapper },
  );
  return { hook, invalidate };
}

async function loginAs(hook: ReturnType<typeof setup>["hook"]) {
  await act(async () => {
    await hook.result.current.auth.login("operator@example.com", "correct-password");
  });
}

const keysCalled = (invalidate: ReturnType<typeof setup>["invalidate"]) =>
  invalidate.mock.calls.map((c) => c[0]?.queryKey);

describe("useRealtimeEvents", () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    vi.useFakeTimers();
    vi.spyOn(Math, "random").mockReturnValue(1); // jitter -> full delay
    vi.stubGlobal("WebSocket", FakeWebSocket);
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          accessToken: "token-123",
          user: { id: "u1", email: "operator@example.com", role: "operator" },
        }),
      }),
    );
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("does not connect while unauthenticated and reports offline", () => {
    const { hook } = setup();
    expect(FakeWebSocket.instances).toHaveLength(0);
    expect(hook.result.current.realtime.status).toBe("offline");
  });

  it("offers the finwatch.v1 and bearer subprotocols to the configured URL", async () => {
    const { hook } = setup();
    await loginAs(hook);
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(last().url).toBe("ws://localhost:8080/ws");
    expect(last().protocols).toEqual(["finwatch.v1", "bearer.token-123"]);
    expect(hook.result.current.realtime.status).toBe("connecting");
  });

  it("reports live and invalidates transactions and alerts on open", async () => {
    const { hook, invalidate } = setup();
    await loginAs(hook);
    act(() => last().open());
    expect(hook.result.current.realtime.status).toBe("live");
    expect(keysCalled(invalidate)).toEqual([["transactions"], ["alerts"]]);
  });

  it("invalidates the matching query on each event type", async () => {
    const { hook, invalidate } = setup();
    await loginAs(hook);
    act(() => last().open());
    invalidate.mockClear();

    act(() => last().message(envelope("1", "transaction.observed")));
    expect(keysCalled(invalidate)).toEqual([["transactions"]]);

    act(() => last().message(envelope("2", "alert.raised")));
    expect(keysCalled(invalidate)).toEqual([["transactions"], ["alerts"]]);
  });

  it("de-duplicates redelivered events by envelope id", async () => {
    const { hook, invalidate } = setup();
    await loginAs(hook);
    act(() => last().open());
    invalidate.mockClear();

    act(() => {
      last().message(envelope("7", "alert.raised"));
      last().message(envelope("7", "alert.raised"));
    });
    expect(invalidate).toHaveBeenCalledTimes(1);
  });

  it("ignores malformed or unknown messages", async () => {
    const { hook, invalidate } = setup();
    await loginAs(hook);
    act(() => last().open());
    invalidate.mockClear();

    act(() => {
      last().message("not json");
      last().message(new ArrayBuffer(4));
      last().message("null");
      last().message("[]");
      last().message(envelope("", "alert.raised"));
      last().message(envelope("8", "something.else"));
      last().message(envelope("9", "alert.raised", { version: "1" }));
      last().message(envelope("10", "alert.raised", { occurredAt: 5 }));
      last().message(envelope("11", "alert.raised", { data: null }));
    });
    expect(invalidate).not.toHaveBeenCalled();
    expect(hook.result.current.realtime.status).toBe("live");
  });

  it("reconnects after an abnormal close with capped exponential backoff", async () => {
    const { hook, invalidate } = setup();
    await loginAs(hook);
    act(() => last().open());

    act(() => last().closeWith(1006));
    expect(hook.result.current.realtime.status).toBe("offline");
    act(() => vi.advanceTimersByTime(999));
    expect(FakeWebSocket.instances).toHaveLength(1);
    act(() => vi.advanceTimersByTime(1));
    expect(FakeWebSocket.instances).toHaveLength(2);
    expect(hook.result.current.realtime.status).toBe("connecting");

    // Fails before ever opening: delay doubles (2s, 4s, ...) up to 30s.
    act(() => last().closeWith(1006));
    act(() => vi.advanceTimersByTime(2_000));
    expect(FakeWebSocket.instances).toHaveLength(3);
    act(() => last().closeWith(1006));
    act(() => vi.advanceTimersByTime(4_000));
    expect(FakeWebSocket.instances).toHaveLength(4);
    for (let i = 0; i < 10; i++) {
      act(() => last().closeWith(1006));
      act(() => vi.advanceTimersByTime(30_000));
    }
    const count = FakeWebSocket.instances.length;
    act(() => last().closeWith(1006));
    act(() => vi.advanceTimersByTime(29_999));
    expect(FakeWebSocket.instances).toHaveLength(count);
    act(() => vi.advanceTimersByTime(1));
    expect(FakeWebSocket.instances).toHaveLength(count + 1);

    // A successful open resets the backoff and refetches missed events.
    invalidate.mockClear();
    act(() => last().open());
    expect(keysCalled(invalidate)).toEqual([["transactions"], ["alerts"]]);
    act(() => last().closeWith(1013));
    act(() => vi.advanceTimersByTime(1_000));
    expect(FakeWebSocket.instances).toHaveLength(count + 2);
  });

  it("reconnects after 1001 going away", async () => {
    const { hook } = setup();
    await loginAs(hook);
    act(() => last().open());
    act(() => last().closeWith(1001));
    act(() => vi.advanceTimersByTime(1_000));
    expect(FakeWebSocket.instances).toHaveLength(2);
  });

  it("does not reconnect on 1008 and logs the user out", async () => {
    const { hook } = setup();
    await loginAs(hook);
    act(() => last().open());

    act(() => last().closeWith(1008));
    expect(hook.result.current.auth.accessToken).toBeNull();
    act(() => vi.advanceTimersByTime(120_000));
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(hook.result.current.realtime.status).toBe("offline");
  });

  it("closes the socket and stops reconnecting on logout", async () => {
    const { hook } = setup();
    await loginAs(hook);
    const ws = last();
    act(() => ws.open());
    act(() => hook.result.current.auth.logout());
    expect(ws.close).toHaveBeenCalledTimes(1);
    expect(hook.result.current.realtime.status).toBe("offline");
    act(() => vi.advanceTimersByTime(120_000));
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  it("closes the socket and cancels a pending reconnect on unmount", async () => {
    const { hook } = setup();
    await loginAs(hook);
    const ws = last();
    act(() => ws.open());
    act(() => ws.closeWith(1006));
    hook.unmount();
    act(() => vi.advanceTimersByTime(120_000));
    expect(FakeWebSocket.instances).toHaveLength(1);

    const second = setup();
    await loginAs(second.hook);
    const ws2 = last();
    second.hook.unmount();
    expect(ws2.close).toHaveBeenCalledTimes(1);
  });

  it("remembers at most 500 recent ids", async () => {
    const { hook, invalidate } = setup();
    await loginAs(hook);
    act(() => last().open());
    act(() => {
      for (let i = 0; i < 501; i++) last().message(envelope(String(i), "alert.raised"));
    });
    invalidate.mockClear();
    act(() => {
      last().message(envelope("500", "alert.raised")); // still remembered
      last().message(envelope("0", "alert.raised")); // evicted, treated as new
    });
    expect(invalidate).toHaveBeenCalledTimes(1);
  });
});
