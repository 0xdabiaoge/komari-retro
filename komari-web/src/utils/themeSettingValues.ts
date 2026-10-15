type Settings = Record<string, unknown>;
const isRecord = (value: unknown): value is Settings =>
  !!value && typeof value === "object" && !Array.isArray(value);
const isRetroNestedKey = (theme: string, key: string) =>
  theme === "retro" && /^(guestDisplay|statusCardsVisibility)\.[^.]+$/.test(key);

export function readThemeSetting(settings: Settings, key: string, theme: string): unknown {
  if (isRetroNestedKey(theme, key)) {
    const [group, field] = key.split(".");
    const nested = settings[group];
    if (isRecord(nested) && nested[field] !== undefined) return nested[field];
  }
  return settings[key];
}

// Retro's frontend stores these groups as objects. Canonicalize managed-form
// dotted keys too, retaining unknown options and old flat-key compatibility.
export function mergeThemeSettings(current: Settings, patch: Settings, theme: string): Settings {
  const next = { ...current };
  for (const [key, value] of Object.entries(patch)) {
    if (isRetroNestedKey(theme, key)) {
      const [group, field] = key.split(".");
      next[group] = { ...(isRecord(next[group]) ? next[group] : {}), [field]: value };
      delete next[key];
    } else {
      next[key] = value;
      if (theme === "retro" && (key === "guestDisplay" || key === "statusCardsVisibility") && isRecord(value)) {
        for (const field of Object.keys(value)) delete next[`${key}.${field}`];
      }
    }
  }
  return next;
}
