interface HistoryPolicy {
  is_share_view?: boolean;
  ping_record_preserve_time?: number;
}

// Wait for the audience before starting a cached history request. A realtime
// share has no historical window, while management keeps its requested window.
export function pingHistoryHours(policy: HistoryPolicy | null, requested = 24): number {
  if (!policy || !Number.isFinite(requested) || requested <= 0) return 0;
  if (!policy.is_share_view) return requested;
  const allowed = policy.ping_record_preserve_time;
  return typeof allowed === "number" && Number.isFinite(allowed) && allowed > 0
    ? Math.min(requested, allowed)
    : 0;
}
