import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AuthProvider } from "@/app/AuthContext";
import { ProtectedRoute } from "@/app/ProtectedRoute";

function renderProtected(initialPath: string) {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/login" element={<div>login page</div>} />
          <Route element={<ProtectedRoute />}>
            <Route path="/dashboard" element={<div>dashboard page</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  );
}

describe("ProtectedRoute", () => {
  it("redirects to /login when there is no access token", () => {
    renderProtected("/dashboard");
    expect(screen.getByText("login page")).toBeInTheDocument();
    expect(screen.queryByText("dashboard page")).not.toBeInTheDocument();
  });
});
