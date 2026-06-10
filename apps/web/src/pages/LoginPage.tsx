import { Button } from "@/components/ui/button";

/**
 * LoginPage is a placeholder. Authentication (short-lived JWT + RBAC) is
 * implemented in a later issue; the form here is non-functional.
 */
export function LoginPage() {
  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-6 p-6">
      <div className="space-y-1 text-center">
        <h1 className="text-2xl font-semibold tracking-tight">Sign in</h1>
        <p className="text-sm text-muted-foreground">
          FinWatch operator access (placeholder).
        </p>
      </div>
      <form
        className="space-y-4"
        onSubmit={(event) => event.preventDefault()}
        aria-label="Sign in"
      >
        <div className="space-y-1">
          <label htmlFor="email" className="text-sm font-medium">
            Email
          </label>
          <input
            id="email"
            type="email"
            autoComplete="username"
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            placeholder="operator@example.com"
            disabled
          />
        </div>
        <div className="space-y-1">
          <label htmlFor="password" className="text-sm font-medium">
            Password
          </label>
          <input
            id="password"
            type="password"
            autoComplete="current-password"
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            disabled
          />
        </div>
        <Button type="submit" className="w-full" disabled>
          Sign in (coming soon)
        </Button>
      </form>
    </main>
  );
}
