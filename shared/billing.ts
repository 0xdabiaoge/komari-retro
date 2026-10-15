import currencies from "./currencies.json";

export const BILLING_CURRENCIES = currencies;
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
