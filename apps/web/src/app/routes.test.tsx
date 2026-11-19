import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider, useAuth } from "@/app/AuthContext";
import { AppRoutes } from "@/app/routes";

function renderAt(path: string) {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[path]}>
        <AppRoutes />
      </MemoryRouter>
    </AuthProvider>,
  );
}

describe("AppRoutes", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
    // The app shell opens the realtime socket once authenticated.
    vi.stubGlobal(
      "WebSocket",
      class {
        close() {}
      },
    );
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("redirects unauthenticated requests for the dashboard to /login", () => {
    renderAt("/dashboard");
    expect(screen.getByRole("heading", { name: /sign in/i })).toBeInTheDocument();
  });

  it("renders the dashboard with primary navigation once authenticated", async () => {
    // /login returns a token; the dashboard's data requests get empty pages.
    (fetch as ReturnType<typeof vi.fn>).mockImplementation(async (input: string) => {
      if (String(input).endsWith("/login")) {
        return {
          ok: true,
          json: async () => ({
            accessToken: "token-123",
            user: { id: "u1", email: "operator@example.com", role: "operator" },
          }),
        };
      }
      return { ok: true, status: 200, json: async () => ({ items: [], nextCursor: null }) };
    });

    function LoginThenRoutes() {
      const { accessToken, login } = useAuth();
      if (!accessToken) {
        return (
          <button
            onClick={() => {
              void login("operator@example.com", "correct-password");
            }}
          >
            do-login
          </button>
        );
      }
      return (
        <MemoryRouter initialEntries={["/dashboard"]}>
          <AppRoutes />
        </MemoryRouter>
      );
    }

    render(
      <AuthProvider>
        <QueryClientProvider client={new QueryClient()}>
          <LoginThenRoutes />
        </QueryClientProvider>
      </AuthProvider>,
    );
    await userEvent.click(screen.getByText("do-login"));
    expect(await screen.findByRole("heading", { name: /dashboard/i })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: /primary/i })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/connecting/i);
    expect(screen.getByRole("link", { name: "Rules" })).toHaveAttribute("href", "/rules");
  });

  it("renders a not-found page for unknown routes", () => {
    renderAt("/nope");
    expect(screen.getByText("404")).toBeInTheDocument();
  });

  it("renders the login page outside the app shell", () => {
    renderAt("/login");
    expect(screen.getByRole("heading", { name: /sign in/i })).toBeInTheDocument();
  });
});
