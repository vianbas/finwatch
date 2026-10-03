import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/app/AuthContext";
import { env } from "@/lib/env";
import { queryKeys } from "@/lib/queryKeys";

export type RealtimeStatus = "connecting" | "live" | "offline";

const SUBPROTOCOL = "finwatch.v1";
const MAX_SEEN_IDS = 500;
const BASE_DELAY_MS = 1_000;
const MAX_DELAY_MS = 30_000;
const CLOSE_POLICY_VIOLATION = 1008;

const EVENT_TYPES = ["transaction.observed", "alert.raised"] as const;
type EventType = (typeof EVENT_TYPES)[number];

/**
 * parseEnvelope returns the event id and type of a well-formed realtime
 * envelope (see contracts/asyncapi.yaml), or null for anything else.
 */
function parseEnvelope(raw: unknown): { id: string; type: EventType } | null {
  if (typeof raw !== "string") return null;
  let msg: unknown;
  try {
    msg = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof msg !== "object" || msg === null) return null;
  const { id, type, version, occurredAt, data } = msg as Record<string, unknown>;
  if (typeof id !== "string" || id === "") return null;
  if (!EVENT_TYPES.includes(type as EventType)) return null;
  if (!Number.isInteger(version) || typeof occurredAt !== "string") return null;
  if (typeof data !== "object" || data === null) return null;
  return { id, type: type as EventType };
}

/** backoffDelay is capped exponential backoff with "equal" jitter (50-100%). */
function backoffDelay(attempt: number): number {
  const ceiling = Math.min(MAX_DELAY_MS, BASE_DELAY_MS * 2 ** attempt);
  return ceiling / 2 + (Math.random() * ceiling) / 2;
}

/**
 * useRealtimeEvents keeps one WebSocket open to the realtime stream while the
 * user is authenticated and invalidates the matching TanStack Query caches
 * when events arrive. Events carry no data the UI needs directly: pages
 * refetch over REST.
 *
 * - The access token travels as the `bearer.<token>` subprotocol.
 * - Delivery is at-least-once, so envelope ids are de-duplicated (bounded).
 * - There is no replay, so every successful (re)connect invalidates the
 *   transactions and alerts queries.
 * - Abnormal closes (and 1001/1013) reconnect with capped exponential backoff.
 *   Close code 1008 means the token expired: the user is logged out instead.
 */
export function useRealtimeEvents(): { status: RealtimeStatus } {
  const { accessToken, logout } = useAuth();
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<RealtimeStatus>("connecting");

  useEffect(() => {
    if (!accessToken) return;

    const seen = new Set<string>();
    let socket: WebSocket | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;
    let disposed = false;

    const invalidateAll = () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.transactions });
      void queryClient.invalidateQueries({ queryKey: queryKeys.alerts });
    };

    const connect = () => {
      const ws = new WebSocket(env.wsUrl, [SUBPROTOCOL, `bearer.${accessToken}`]);
      socket = ws;

      ws.onopen = () => {
        attempt = 0;
        setStatus("live");
        invalidateAll();
      };

      ws.onmessage = (ev: MessageEvent) => {
        const envelope = parseEnvelope(ev.data);
        if (!envelope || seen.has(envelope.id)) return;
        seen.add(envelope.id);
        if (seen.size > MAX_SEEN_IDS) {
          // Sets iterate in insertion order, so the first entry is the oldest.
          seen.delete(seen.values().next().value as string);
        }
        const key =
          envelope.type === "transaction.observed"
            ? queryKeys.transactions
            : queryKeys.alerts;
        void queryClient.invalidateQueries({ queryKey: key });
      };

      ws.onclose = (ev: CloseEvent) => {
        if (disposed) return;
        setStatus("offline");
        if (ev.code === CLOSE_POLICY_VIOLATION) {
          // Token expired: reconnecting would fail the same way. Clearing the
          // auth state lets ProtectedRoute send the user back to /login.
          logout();
          return;
        }
        timer = setTimeout(() => {
          setStatus("connecting");
          connect();
        }, backoffDelay(attempt));
        attempt += 1;
      };
    };

    connect();

    return () => {
      disposed = true;
      clearTimeout(timer);
      if (socket) {
        socket.onopen = socket.onmessage = socket.onclose = null;
        socket.close();
      }
    };
  }, [accessToken, logout, queryClient]);

  return { status: accessToken ? status : "offline" };
}
