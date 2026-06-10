# Frontend rules (apps/web)

## Stack

- React + TypeScript + Vite.
- Tailwind CSS with **shadcn-compatible** component conventions (CSS variables,
  `cn()` class merge helper, `class-variance-authority` for variants).
- Routing via React Router.
- Server state via **TanStack Query** — do not hand-roll fetch caching.
- Charts via **Recharts**.

## Structure

- `src/main.tsx` mounts the app inside providers and the error boundary.
- `src/app/` holds the shell, router, and providers.
- `src/pages/` holds route pages (placeholders in the bootstrap).
- `src/components/` holds shared UI; `src/lib/` holds utilities (env, query client).
- Keep pages thin; push shared logic into hooks/lib.

## Environment

- Read configuration only through `src/lib/env.ts`, which **validates**
  `VITE_API_URL` and `VITE_WS_URL` at module load and throws on misconfiguration.
- Never hard-code API or WebSocket URLs in components.

## Accessibility & semantics

- Use semantic landmarks (`header`, `nav`, `main`, `footer`).
- Navigation must be keyboard-operable and responsive.
- Every interactive control has an accessible name.

## Money & time

- Format money from integer minor units; never do float math on money.
- Treat timestamps as RFC 3339 UTC; format for display only at the edge.

## Quality gates (must pass before commit)

ESLint clean · `tsc --noEmit` clean · Vitest green · `vite build` succeeds.
