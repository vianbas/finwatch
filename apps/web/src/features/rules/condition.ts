const symbols: Record<string, string> = { gte: "≥", lte: "≤", eq: "=" };

/** groupDigits inserts thousands separators into a string of digits without any Number math. */
function groupDigits(integer: string): string {
  return integer.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

/**
 * describeCondition renders a rule's condition for display, e.g.
 * "Amount ≥ 500,000 minor units". Rules carry no currency, so amounts stay in
 * integer minor units (grouped via BigInt/string handling, never float math).
 * Unrecognised combinations fall back to the raw "field operator value".
 */
export function describeCondition(field: string, operator: string, value: string): string {
  const symbol = symbols[operator];
  if (symbol) {
    if (field === "amount_minor" && /^\d+$/.test(value)) {
      return `Amount ${symbol} ${groupDigits(BigInt(value).toString())} minor units`;
    }
    if (operator === "eq" && field === "currency" && value !== "") return `Currency = ${value}`;
    if (operator === "eq" && field === "direction" && value !== "") return `Direction = ${value}`;
  }
  return `${field} ${operator} ${value}`;
}
