import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "@/app/AuthContext";

/**
 * ProtectedRoute renders its nested routes only when an access token is
 * present; otherwise it redirects to /login. Reactivity comes from
 * useAuth() — clearing the token (e.g. on a 401) re-renders this guard.
 */
export function ProtectedRoute() {
  const { accessToken } = useAuth();
  if (!accessToken) {
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}
