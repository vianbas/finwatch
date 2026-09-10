import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthProvider } from "@/app/AuthContext";
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
  it("renders the dashboard with primary navigation", () => {
    renderAt("/dashboard");
    expect(
      screen.getByRole("heading", { name: /dashboard/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("navigation", { name: /primary/i }),
    ).toBeInTheDocument();
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
