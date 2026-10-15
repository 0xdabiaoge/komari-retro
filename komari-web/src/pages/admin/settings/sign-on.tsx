import { SettingCardLabel, SettingCardSelect, SettingCardShortTextInput } from "@/components/admin/SettingCard";
import { updateSettingsWithToast, useSettings } from "@/lib/api";
import { Button } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import Loading from "@/components/loading";
import React from "react";
import { toast } from "sonner";

export default function AccessSettings() {
 const { t } = useTranslation();
 const { loading, error } = useSettings();
 if (loading) return <Loading />;
 if (error) return <div>{error}</div>;
 return <><SettingCardLabel>{t("settings.api.title")}</SettingCardLabel><ApiCard /></>;
}

const ApiCard = () => {
  const { settings } = useSettings();
  const { t } = useTranslation();
  const [apiValues, setApiValues] = React.useState<string>(settings?.api_key || "" );

  // 生成32位随机字符串
  const generateRandomString = () => {
    return "komari-" + Array.from(crypto.getRandomValues(new Uint8Array(32)), byte => byte.toString(16).padStart(2, "0")).join("");
  };

  // 处理生成按钮点击
  const handleGenerateApiKey = () => {
    const newApiKey = generateRandomString();
    setApiValues(newApiKey);
  };

  // 初始化API值
  React.useEffect(() => {
    if (settings?.api_key) {
      setApiValues(settings.api_key);
    }
  }, [settings?.api_key]);

  return (
    <>
    <SettingCardShortTextInput
        title={t("settings.api.title")}
        description={t("settings.api.description")}
        value={apiValues}
        onChange={(e) => setApiValues(e.target.value)}
        OnSave={async (values) => {
          if (!values) {
            await updateSettingsWithToast({ api_key: "" }, t);
            return;
          }
          if (values.length < 12) {
            toast.error(t("settings.api.key_length_error"));
            return;
          }
          await updateSettingsWithToast({ api_key: values }, t);
        }}
      >
        <div className="flex flex-row gap-2 justify-start items-center">
          <Button variant="soft" color="green" onClick={handleGenerateApiKey}>{t('common.generate')}</Button>
        </div>
      </SettingCardShortTextInput>
      <SettingCardSelect
        title="API 权限"
        description="只读密钥用于查询监控数据；完整权限密钥可执行管理操作。"
        value={settings.api_key_scope || "full"}
        options={[{ value: "read-only", label: "只读监控数据" }, { value: "full", label: "完整管理权限" }]}
        OnSave={async (value) => { await updateSettingsWithToast({ api_key_scope: value }, t); }}
      />
    </>
  )
}
