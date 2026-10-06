/**
 * Money is carried as integer minor units. Display scaling is done on a
 * decimal string built from a bigint, never with float division, so very
 * large amounts keep every digit.
 */

/** minorUnitDigits is the currency's minor-unit exponent as known to Intl. */
function minorUnitDigits(formatter: Intl.NumberFormat): number {
  return formatter.resolvedOptions().maximumFractionDigits as number;
}

/** scaleMinor turns integer minor units into an exact decimal string. */
function scaleMinor(minor: bigint, digits: number): string {
  const negative = minor < 0n;
  const abs = (negative ? -minor : minor).toString().padStart(digits + 1, "0");
  const whole = abs.slice(0, abs.length - digits);
  const fraction = abs.slice(abs.length - digits);
  return `${negative ? "-" : ""}${whole}${digits > 0 ? `.${fraction}` : ""}`;
}

/** formatDecimal formats an exact decimal string. Intl accepts these (ES2023); lib typings predate it. */
function formatDecimal(formatter: Intl.NumberFormat, decimal: string): string {
  return (formatter.format as unknown as (value: string) => string)(decimal);
}

/**
 * formatMoney formats an integer amount of minor units for an ISO 4217
 * currency, e.g. (1234, "USD") -> "$12.34" and (1234, "JPY") -> "¥1,234".
 * Pass a bigint or an integer string for values beyond Number.MAX_SAFE_INTEGER.
 * Throws RangeError for non-integer input; falls back to "<amount> <CODE>"
 * (2 decimals) if the currency code is not recognised by Intl.
 */
export function formatMoney(
  amountMinor: number | bigint | string,
  currency: string,
  locale = "en-US",
): string {
  if (typeof amountMinor === "number" && !Number.isInteger(amountMinor)) {
    throw new RangeError("amountMinor must be an integer");
  }
  const minor = BigInt(amountMinor);
  let formatter: Intl.NumberFormat;
  try {
    formatter = new Intl.NumberFormat(locale, { style: "currency", currency });
  } catch (e) {
    if (!(e instanceof RangeError)) throw e;
    const plain = new Intl.NumberFormat(locale, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
    return `${formatDecimal(plain, scaleMinor(minor, 2))} ${currency}`;
  }
  return formatDecimal(formatter, scaleMinor(minor, minorUnitDigits(formatter)));
}
