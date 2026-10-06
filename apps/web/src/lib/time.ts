const formatter = new Intl.DateTimeFormat("en-GB", {
  dateStyle: "medium",
  timeStyle: "medium",
  timeZone: "UTC",
});

/**
 * formatTimestamp renders an RFC 3339 timestamp for display, always in UTC
 * (e.g. "6 Oct 2026, 14:03:22 UTC"). Values stay RFC 3339 strings everywhere
 * else; an unparseable value is returned unchanged.
 */
export function formatTimestamp(rfc3339: string): string {
  const date = new Date(rfc3339);
  if (Number.isNaN(date.getTime())) return rfc3339;
  return `${formatter.format(date)} UTC`;
}
