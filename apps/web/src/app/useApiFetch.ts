import { useCallback } from "react";
import { useAuth } from "@/app/AuthContext";
import { env } from "@/lib/env";

/**
 * useApiFetch wraps fetch with the current access token and, on a 401
 * response, logs out — which clears the in-memory token and lets
 * ProtectedRoute redirect to /login on the next render.
 */
export function useApiFetch() {
  const { accessToken, logout } = useAuth();

  return useCallback(
    async (path: string, init: RequestInit = {}) => {
      const headers = new Headers(init.headers);
      if (accessToken) {
        headers.set("Authorization", `Bearer ${accessToken}`);
      }
      const res = await fetch(`${env.apiUrl}${path}`, { ...init, headers });
      if (res.status === 401) {
        logout();
      }
      return res;
    },
    [accessToken, logout],
  );
}
