import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Save } from "lucide-react";
import { toast } from "sonner";
import { AccountProvider, useAccount } from "@/contexts/AccountContext";
import { usePublicInfo } from "@/contexts/PublicInfoContext";
import { Button } from "@/components/ui/button";

type Props = { charts: unknown; onSaved: () => void };
function Action({ charts, onSaved }: Props) {
  const { account } = useAccount();
  const { publicInfo } = usePublicInfo();
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
  if (!account?.logged_in) return null;
  const save = async () => {
    if (!publicInfo?.theme) return;
    setSaving(true);
    try {
      const response = await fetch(`/api/admin/theme/settings?theme=${encodeURIComponent(publicInfo.theme)}`, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...publicInfo.theme_settings, chartDashboardTemplate: JSON.stringify(charts, null, 2) }),
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      toast.success(t("chart.globalTemplateSaved"));
      onSaved();
    } catch (error) {
      toast.error(`${t("chart.globalTemplateSaveFailed")}: ${String(error)}`);
    } finally { setSaving(false); }
  };
  return <Button type="button" variant="outline" size="sm" onClick={save} disabled={saving}>
    <Save className="size-4" />{t("chart.saveGlobalTemplate")}
  </Button>;
}
export default function ChartTemplateAction(props: Props) {
  return <AccountProvider><Action {...props} /></AccountProvider>;
}
