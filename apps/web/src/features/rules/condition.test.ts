import { describe, expect, it } from "vitest";
import { describeCondition } from "@/features/rules/condition";

describe("describeCondition", () => {
  it.each([
    ["amount_minor", "gte", "500000", "Amount ≥ 500,000 minor units"],
    ["amount_minor", "lte", "999", "Amount ≤ 999 minor units"],
    ["amount_minor", "eq", "1000", "Amount = 1,000 minor units"],
    ["amount_minor", "gte", "0", "Amount ≥ 0 minor units"],
    ["amount_minor", "gte", "007500", "Amount ≥ 7,500 minor units"],
    ["currency", "eq", "GBP", "Currency = GBP"],
    ["direction", "eq", "debit", "Direction = debit"],
  ])("renders %s %s %s", (field, operator, value, expected) => {
    expect(describeCondition(field, operator, value)).toBe(expected);
  });

  it("keeps every digit of a value beyond Number.MAX_SAFE_INTEGER", () => {
    expect(describeCondition("amount_minor", "gte", "9007199254740993123")).toBe(
      "Amount ≥ 9,007,199,254,740,993,123 minor units",
    );
  });

  it.each([
    ["amount_minor", "gte", "12.5"],
    ["amount_minor", "gte", "abc"],
    ["amount_minor", "gte", "-5"],
    ["currency", "gte", "GBP"],
    ["direction", "lte", "debit"],
    ["currency", "eq", ""],
    ["account", "eq", "ACC-1"],
    ["amount_minor", "neq", "100"],
  ])("falls back to the raw text for %s %s %s", (field, operator, value) => {
    expect(describeCondition(field, operator, value)).toBe(`${field} ${operator} ${value}`);
  });
});
