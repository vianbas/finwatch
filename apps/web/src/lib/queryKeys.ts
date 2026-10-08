// Shared TanStack Query key prefixes. Feature pages build their keys on these
// prefixes (e.g. [...queryKeys.alerts, filters]) so realtime events can
// invalidate every variant of a list with one call.
export const queryKeys = {
  transactions: ["transactions"] as const,
  alerts: ["alerts"] as const,
  rules: ["rules"] as const,
};
