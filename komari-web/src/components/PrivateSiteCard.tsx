import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Card, Flex, Text, TextField } from "@radix-ui/themes";
import { Lock, LogIn, KeyRound } from "lucide-react";
import { toast } from "sonner";

interface PrivateSiteCardProps {
  sitename?: string;
  onUnlockSuccess?: () => void;
}

export const PrivateSiteCard: React.FC<PrivateSiteCardProps> = ({
  sitename,
  onUnlockSuccess,
}) => {
  const { t } = useTranslation();
  const [tempKey, setTempKey] = useState("");
  const [verifying, setVerifying] = useState(false);

  const handleVerify = async () => {
    const key = tempKey.trim();
    if (!key) return;

    setVerifying(true);
    try {
      const res = await fetch(`/api/public?temp_key=${encodeURIComponent(key)}`);
      if (!res.ok) {
        throw new Error("Validation failed");
      }
      const data = await res.json();
      // 当临时密钥有效时，后端返回 private_site = false
      if (data?.data && data.data.private_site === false) {
        document.cookie = `temp_key=${encodeURIComponent(key)}; path=/; max-age=${60 * 60 * 24 * 30}`;
        toast.success(t("common.success", "验证成功"));
        if (onUnlockSuccess) {
          onUnlockSuccess();
        }
        // 带上 query 参数刷新页面以重置 WebSocket 连接与元数据
        window.location.search = `?temp_key=${encodeURIComponent(key)}`;
        return;
      }
      toast.error(
        t("settings.site.private_site_temp_key_invalid", "临时密钥无效或已过期"),
      );
    } catch {
      toast.error(
        t("settings.site.private_site_temp_key_invalid", "临时密钥无效或已过期"),
      );
    } finally {
      setVerifying(false);
    }
  };

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] px-4 py-8">
      <Card className="km-private-site-card w-full max-w-md p-6 shadow-xl border border-accent-4">
        <Flex direction="column" align="center" gap="4" className="text-center">
          <div className="w-16 h-16 rounded-full bg-accent-3 flex items-center justify-center text-accent-11 shadow-inner">
            <Lock size={32} />
          </div>

          <Flex direction="column" gap="1">
            <Text size="5" weight="bold">
              {sitename || "Komari Monitor"}
            </Text>
            <Text size="3" color="gray" weight="medium">
              {t("settings.site.private_site_locked_title", "私有监控站点")}
            </Text>
          </Flex>

          <Text size="2" color="gray" className="max-w-sm">
            {t(
              "settings.site.private_site_locked_desc",
              "此站点已开启私有保护，监控数据仅对授权用户或持有临时分享链接的访客开放。",
            )}
          </Text>

          <div className="w-full h-px bg-accent-4 my-1" />

          <Button
            size="3"
            className="w-full cursor-pointer"
            onClick={() => {
              window.location.href = "/admin/login";
            }}
          >
            <LogIn size={18} />
            {t("settings.site.private_site_login_btn", "登录管理面板")}
          </Button>

          <Flex direction="column" gap="2" className="w-full mt-2 text-left">
            <Text size="1" color="gray" weight="medium">
              {t("settings.site.temporary_share", "临时分享")}
            </Text>
            <Flex gap="2" className="w-full">
              <TextField.Root
                placeholder={t(
                  "settings.site.private_site_temp_key_placeholder",
                  "输入临时访问密钥",
                )}
                value={tempKey}
                onChange={(e) => setTempKey(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    void handleVerify();
                  }
                }}
                className="flex-1"
              >
                <TextField.Slot>
                  <KeyRound size={14} className="text-muted-foreground" />
                </TextField.Slot>
              </TextField.Root>
              <Button
                variant="soft"
                disabled={!tempKey.trim() || verifying}
                loading={verifying}
                onClick={() => void handleVerify()}
              >
                {t("settings.site.private_site_temp_key_submit", "验证密钥")}
              </Button>
            </Flex>
          </Flex>
        </Flex>
      </Card>
    </div>
  );
};

export default PrivateSiteCard;
