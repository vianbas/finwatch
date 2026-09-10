import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AuthProvider } from "@/app/AuthContext";
import { LoginPage } from "@/pages/LoginPage";

function renderLoginPage() {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={["/login"]}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/dashboard" element={<div>dashboard page</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  );
}

describe("LoginPage", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("navigates to /dashboard on successful submit", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "token-123",
        user: { id: "u1", email: "operator@example.com", role: "operator" },
      }),
    });

    renderLoginPage();
    await userEvent.type(screen.getByLabelText(/email/i), "operator@example.com");
    await userEvent.type(screen.getByLabelText(/password/i), "correct-password");
    await userEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => expect(screen.getByText("dashboard page")).toBeInTheDocument());
  });

  it("shows an error message on invalid credentials", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      json: async () => ({ error: { code: "INVALID_CREDENTIALS", message: "email or password is incorrect" } }),
    });

    renderLoginPage();
    await userEvent.type(screen.getByLabelText(/email/i), "operator@example.com");
    await userEvent.type(screen.getByLabelText(/password/i), "wrong-password");
    await userEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => expect(screen.getByText(/email or password is incorrect/i)).toBeInTheDocument());
  });
});
