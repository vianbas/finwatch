import { describe, expect, it } from "vitest";
import { formatTimestamp } from "@/lib/time";

describe("formatTimestamp", () => {
  it("renders an RFC 3339 UTC value in UTC", () => {
    expect(formatTimestamp("2026-01-02T03:04:05Z")).toBe("2 Jan 2026, 03:04:05 UTC");
  });

  it("converts an offset timestamp to UTC", () => {
    expect(formatTimestamp("2026-01-02T03:04:05+02:00")).toBe("2 Jan 2026, 01:04:05 UTC");
  });

  it("returns an unparseable value unchanged", () => {
    expect(formatTimestamp("not-a-date")).toBe("not-a-date");
  });
});
