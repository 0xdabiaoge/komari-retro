"use client";
import { useCallback, useEffect, useState } from "react";
import { updateSettings } from "@/lib/api";
export function useThemeWriter() {
  const [state, setState] = useState<"loading" | "yes" | "no">("loading");
  useEffect(() => {
    let mounted = true;
    fetch("/api/me").then(r => r.ok ? r.json() : null).then(value => {
      if (mounted) setState(value?.logged_in ? "yes" : "no");
    }).catch(() => { if (mounted) setState("no"); });
    return () => { mounted = false; };
  }, []);
  const write = useCallback((value: Record<string, unknown>) => updateSettings({ theme_settings: value }), []);
  return { state, write };
}
