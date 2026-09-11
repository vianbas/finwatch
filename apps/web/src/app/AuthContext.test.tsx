import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, useAuth } from "@/app/AuthContext";

function Probe() {
  const { accessToken, user, login, logout } = useAuth();
  const [error, setError] = useState<string | null>(null);
  return (
    <div>
      <span data-testid="token">{accessToken ?? "none"}</span>
      <span data-testid="role">{user?.role ?? "none"}</span>
      <span data-testid="error">{error ?? "none"}</span>
      <button
        onClick={() => {
          login("operator@example.com", "correct-password").catch((e: Error) => setError(e.message));
        }}
      >
        login
      </button>
      <button onClick={logout}>logout</button>
    </div>
  );
}

describe("AuthContext", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("stores the access token and user after a successful login", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "token-123",
        user: { id: "u1", email: "operator@example.com", role: "operator" },
      }),
    });

    render(<AuthProvider><Probe /></AuthProvider>);
    await userEvent.click(screen.getByText("login"));

    await waitFor(() => expect(screen.getByTestId("token")).toHaveTextContent("token-123"));
    expect(screen.getByTestId("role")).toHaveTextContent("operator");
  });

  it("throws and leaves state empty when login fails", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      json: async () => ({ error: { code: "INVALID_CREDENTIALS", message: "email or password is incorrect" } }),
    });

    render(<AuthProvider><Probe /></AuthProvider>);
    await userEvent.click(screen.getByText("login"));

    await waitFor(() => expect(screen.getByTestId("token")).toHaveTextContent("none"));
    await waitFor(() => expect(screen.getByTestId("error")).toHaveTextContent("email or password is incorrect"));
  });

  it("clears the token and user on logout", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "token-123",
        user: { id: "u1", email: "operator@example.com", role: "operator" },
      }),
    });

    render(<AuthProvider><Probe /></AuthProvider>);
    await userEvent.click(screen.getByText("login"));
    await waitFor(() => expect(screen.getByTestId("token")).toHaveTextContent("token-123"));

    await userEvent.click(screen.getByText("logout"));
    expect(screen.getByTestId("token")).toHaveTextContent("none");
    expect(screen.getByTestId("role")).toHaveTextContent("none");
  });
});
