import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
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
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("redirects unauthenticated requests for the dashboard to /login", () => {
    renderAt("/dashboard");
    expect(screen.getByRole("heading", { name: /sign in/i })).toBeInTheDocument();
  });

  it("renders the dashboard with primary navigation once authenticated", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "token-123",
        user: { id: "u1", email: "operator@example.com", role: "operator" },
      }),
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
        <LoginThenRoutes />
      </AuthProvider>,
    );
    await userEvent.click(screen.getByText("do-login"));
    expect(await screen.findByRole("heading", { name: /dashboard/i })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: /primary/i })).toBeInTheDocument();
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
