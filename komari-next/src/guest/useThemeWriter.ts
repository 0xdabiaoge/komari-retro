"use client";
const write = async (_value: Record<string, unknown>) => {};
export function useThemeWriter() { return { state: "no" as const, write }; }
