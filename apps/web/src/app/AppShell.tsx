import { useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { Activity, Bell, LayoutDashboard, Menu, Wrench } from "lucide-react";
import { cn } from "@/lib/utils";

const navItems = [
  { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { to: "/alerts", label: "Alerts", icon: Bell },
  { to: "/ops", label: "Ops", icon: Wrench },
];

/**
 * AppShell is the authenticated application layout: a responsive header with
 * primary navigation and a routed main content region. It uses semantic
 * landmarks (header, nav, main) for accessibility.
 */
export function AppShell() {
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-b border-border">
        <div className="container flex h-16 items-center justify-between gap-4">
          <div className="flex items-center gap-2 font-semibold">
            <Activity className="h-5 w-5" aria-hidden="true" />
            <span>FinWatch</span>
          </div>

          <nav aria-label="Primary" className="hidden md:block">
            <ul className="flex items-center gap-1">
              {navItems.map((item) => (
                <li key={item.to}>
                  <NavLink to={item.to} className={navLinkClasses}>
                    <item.icon className="h-4 w-4" aria-hidden="true" />
                    {item.label}
                  </NavLink>
                </li>
              ))}
            </ul>
          </nav>

          <button
            type="button"
            className="inline-flex items-center justify-center rounded-md p-2 hover:bg-accent md:hidden"
            aria-label="Toggle navigation menu"
            aria-expanded={mobileOpen}
            onClick={() => setMobileOpen((open) => !open)}
          >
            <Menu className="h-5 w-5" aria-hidden="true" />
          </button>
        </div>

        {mobileOpen && (
          <nav aria-label="Primary mobile" className="border-t border-border md:hidden">
            <ul className="container flex flex-col py-2">
              {navItems.map((item) => (
                <li key={item.to}>
                  <NavLink
                    to={item.to}
                    className={navLinkClasses}
                    onClick={() => setMobileOpen(false)}
                  >
                    <item.icon className="h-4 w-4" aria-hidden="true" />
                    {item.label}
                  </NavLink>
                </li>
              ))}
            </ul>
          </nav>
        )}
      </header>

      <main className="container flex-1 py-6">
        <Outlet />
      </main>

      <footer className="border-t border-border">
        <div className="container py-4 text-sm text-muted-foreground">
          FinWatch — reference implementation. Synthetic data only.
        </div>
      </footer>
    </div>
  );
}

function navLinkClasses({ isActive }: { isActive: boolean }): string {
  return cn(
    "flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground",
    isActive ? "bg-accent text-accent-foreground" : "text-muted-foreground",
  );
}
