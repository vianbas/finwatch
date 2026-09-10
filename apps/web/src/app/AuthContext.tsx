import { createContext, useCallback, useContext, useState, type ReactNode } from "react";
import { env } from "@/lib/env";

export interface AuthUser {
  id: string;
  email: string;
  role: "operator" | "admin";
}

interface AuthContextValue {
  /** Held in memory only — never written to localStorage or sessionStorage. */
  accessToken: string | null;
  user: AuthUser | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

/** AuthProvider holds the access token and current user in memory only. */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [user, setUser] = useState<AuthUser | null>(null);

  const login = useCallback(async (email: string, password: string) => {
    const res = await fetch(`${env.apiUrl}/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      throw new Error(body?.error?.message ?? "Login failed");
    }
    const data = await res.json();
    setAccessToken(data.accessToken);
    setUser(data.user);
  }, []);

  const logout = useCallback(() => {
    setAccessToken(null);
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider value={{ accessToken, user, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

/** useAuth reads the auth state; must be used within an AuthProvider. */
// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
