import { describe, expect, it } from "vitest";
import { formatMoney } from "@/lib/money";

describe("formatMoney", () => {
  it.each([
    [1234, "USD", "$12.34"],
    [5, "USD", "$0.05"],
    [0, "USD", "$0.00"],
    [100, "USD", "$1.00"],
    [-250, "USD", "-$2.50"],
    [1234, "JPY", "¥1,234"], // 0 minor-unit digits
    [12345, "KWD", "KWD 12.345"], // 3 minor-unit digits
  ])("formats %s %s as %s", (minor, currency, expected) => {
    expect(formatMoney(minor, currency).replaceAll("\u00a0", " ")).toBe(expected);
  });

  it("keeps every digit of a value beyond Number.MAX_SAFE_INTEGER", () => {
    // 2^53 + 1 cannot be represented as a number; bigint and string inputs stay exact.
    expect(formatMoney(9007199254740993n, "USD")).toBe("$90,071,992,547,409.93");
    expect(formatMoney("9007199254740993", "USD")).toBe("$90,071,992,547,409.93");
    expect(formatMoney(9223372036854775807n, "JPY")).toBe("¥9,223,372,036,854,775,807");
  });

  it("formats a large safe integer without float drift", () => {
    expect(formatMoney(Number.MAX_SAFE_INTEGER, "USD")).toBe("$90,071,992,547,409.91");
  });

  it("rejects non-integer amounts", () => {
    expect(() => formatMoney(12.5, "USD")).toThrow(RangeError);
  });

  it("falls back to the currency code for an unrecognised code", () => {
    expect(formatMoney(1234, "ZZ")).toBe("12.34 ZZ");
  });
});
