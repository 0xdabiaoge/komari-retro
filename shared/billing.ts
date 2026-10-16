import currencies from "./currencies.json";

export const BILLING_CURRENCIES = currencies;
export const BILLING_CYCLES = [
  { label: "天", value: "1" },
  { label: "月", value: "30" },
  { label: "季", value: "92" },
  { label: "年", value: "365" },
  { label: "免费/一次性", value: "-1" },
];

// Match the existing calendar-renewal ranges without silently converting
// legacy custom/multi-year periods to a shorter cycle.
export function billingCycleOption(days: number): string {
  if (days === 1 || days === -1) return String(days);
  if (days >= 27 && days <= 32) return "30";
  if (days >= 87 && days <= 95) return "92";
  if (days >= 360 && days <= 370) return "365";
  return days === 0 ? "30" : "";
}
export type PriceMode = "hidden" | "free" | "paid";

export function priceModeFromPrice(price: number): PriceMode {
  return price === -1 ? "free" : price > 0 ? "paid" : "hidden";
}

export function billingPrice(mode: PriceMode, amount: string): number {
  if (mode === "hidden") return 0;
  if (mode === "free") return -1;
  const value = Number(amount);
  if (!amount.trim() || !Number.isFinite(value) || value <= 0)
    throw new Error("付费金额必须大于 0");
  return value;
}

export function normalizeBillingCurrency(value: string): string {
  const trimmed = value.trim();
  return (
    currencies.find(
      (c) => c.code === trimmed.toUpperCase() || c.aliases.includes(trimmed),
    )?.code ?? trimmed
  );
}

export function formatBillingAmount(currency: string, price: number): string {
  const code = normalizeBillingCurrency(currency);
  const known = currencies.find((c) => c.code === code);
  return `${known?.symbol ?? currency}${price}`;
}
