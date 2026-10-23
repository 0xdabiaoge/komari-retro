import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import {
  defaultPreferences,
  normalizePreferences,
  type TerminalPreferences,
} from "./terminalPreferences";

export function useTerminalPreferences() {
  const [preferences, setPreferences] = useState(defaultPreferences);
  const [ready, setReady] = useState(false);
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    fetch("/api/admin/settings/xtermjs", { signal: controller.signal })
      .then(async (r) => {
        if (!r.ok) throw new Error("读取工作台设置失败");
        const body = await r.json();
        setPreferences(normalizePreferences(body.data ?? body));
        setReady(true);
      })
      .catch((error) => {
        if (!controller.signal.aborted) {
          toast.error(error.message);
          setReady(true);
        }
      });
    return () => controller.abort();
  }, []);
  const save = useCallback(async (next: TerminalPreferences) => {
    setSaving(true);
    try {
      const normalized = normalizePreferences(next);
      const r = await fetch("/api/admin/settings/xtermjs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(normalized),
      });
      if (!r.ok) throw new Error("保存工作台设置失败，请检查登录状态");
      setPreferences(normalized);
      toast.success("工作台设置已保存");
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存失败");
      return false;
    } finally {
      setSaving(false);
    }
  }, []);
  return { preferences, ready, saving, save };
}
