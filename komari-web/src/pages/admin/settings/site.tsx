import { useTranslation } from "react-i18next";
import { Button, Dialog, Flex, Text } from "@radix-ui/themes";
import { updateSettingsWithToast, useSettings } from "@/lib/api";
import { authorizeSensitiveAccess } from "@/lib/sensitive";
import {
  SettingCard,
  SettingCardButton,
  SettingCardCollapse,
  SettingCardIconButton,
  SettingCardLabel,
  SettingCardLongTextInput,
  SettingCardShortTextInput,
  SettingCardSwitch,
} from "@/components/admin/SettingCard";
import { toast } from "sonner";
import Loading from "@/components/loading";
import { DownloadIcon } from "lucide-react";
import { useRef, useState } from "react";
import UploadDialog from "@/components/UploadDialog";
import type { ChunkUploadTask } from "@/lib/chunkUpload";
import { createBackupUploadTask } from "@/lib/backupUpload";

export default function SiteSettings() {
  const { t } = useTranslation();
  const { settings, loading, error, refetch } = useSettings();
  const [shareHours, setShareHours] = useState(1);

  // 恢复备份对话框与上传状态
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [restoreProgress, setRestoreProgress] = useState(0);
  const restoreTaskRef = useRef<ChunkUploadTask | null>(null);

  const uploadBackup = async (file: File) => {
    if (restoring) return;

    if (!file.name.toLowerCase().endsWith(".zip") || file.size === 0) {
      toast.error(t("theme.invalid_file_type", "仅支持 .zip 文件"));
      return;
    }

    setRestoring(true);
    setRestoreProgress(0);
    const task = createBackupUploadTask();
    restoreTaskRef.current = task;
    try {
      await task.upload("backup", file, setRestoreProgress);
      toast.success(t("account_settings.upload_success", "上传成功"));
      setRestoreOpen(false);
      setRestoreProgress(0);
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") return;
      const msg =
        err instanceof Error
          ? err.message
          : t("settings.site.backup_restore_error", "恢复备份失败");
      toast.error(msg);
    } finally {
      setRestoring(false);
      restoreTaskRef.current = null;
    }
  };

  const cancelRestore = () => {
    restoreTaskRef.current?.cancel();
    setRestoreProgress(0);
  };

  if (loading) {
    return <Loading />;
  }

  if (error) {
    return <Text color="red">{error}</Text>;
  }

  return (
    <>
      <SettingCardLabel>{t("settings.site.title")}</SettingCardLabel>
      <SettingCardShortTextInput
        title={t("settings.site.name")}
        description={t("settings.site.name_description")}
        defaultValue={settings.sitename || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ sitename: data }, t);
        }}
      />
      <SettingCardLongTextInput
        title={t("settings.site.description")}
        description={t("settings.site.description_description")}
        defaultValue={settings.description || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ description: data }, t);
        }}
      />
      <SettingCardSwitch
        title={t("settings.site.cors_origin_check_enabled")}
        description={t("settings.site.cors_origin_check_enabled_description")}
        defaultChecked={settings.cors_origin_check_enabled ?? true}
        onChange={async (checked) => {
          await updateSettingsWithToast({ cors_origin_check_enabled: checked }, t);
        }}
        className="km-page-admin-settings-site km-setting-card"
      />
      <SettingCardLongTextInput
        title={t("settings.site.cors_allowed_origins", "API CORS 允许列表")}
        description={t("settings.site.origins_list_description",
          "每行或用逗号分隔一个 Origin，例如 https://example.com",
        )}
        defaultValue={settings.cors_allowed_origins || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ cors_allowed_origins: data }, t);
        }}
      />
      <SettingCardSwitch
        title={t("settings.site.ws_origin_check_enabled", "WebSocket Origin 校验")}
        description={t(
          "settings.site.ws_origin_check_enabled_description",
          "开启后 WebSocket 请求只允许同源或允许列表中的 Origin",
        )}
        defaultChecked={settings.ws_origin_check_enabled ?? true}
        onChange={async (checked) => {
          await updateSettingsWithToast(
            { ws_origin_check_enabled: checked },
            t,
          );
        }}
        className="km-setting-card"
      />
      <SettingCardLongTextInput
        title={t("settings.site.ws_allowed_origins", "WebSocket Origin 允许列表")}
        description={t("settings.site.origins_list_description",
          "每行或用逗号分隔一个 Origin，例如 https://example.com",
        )}
        defaultValue={settings.ws_allowed_origins || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ ws_allowed_origins: data }, t);
        }}
      />
      <SettingCard
        title={t("settings.site.ssrf_protection_enabled")}
        description={t("settings.site.ssrf_protection_enabled_description")}
        className="km-setting-card"
      />
      <SettingCardSwitch
        title={t("settings.site.send_ip_addr_to_guest")}
        description={t("settings.site.send_ip_addr_to_guest_description")}
        defaultChecked={settings.send_ip_addr_to_guest}
        onChange={async (checked) => {
          await updateSettingsWithToast({ send_ip_addr_to_guest: checked }, t);
        }}
        className="km-setting-card"
      />
      <SettingCardShortTextInput
        title={t("settings.site.script_domain")}
        description={t("settings.site.script_domain_description")}
        placeholder={`${window.location.origin}`}
        defaultValue={settings.script_domain || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ script_domain: data }, t);
        }}
      />
      <SettingCardLabel>{t("settings.site.private_site")}</SettingCardLabel>
      <SettingCardSwitch
        title={t("settings.site.private_site")}
        description={t("settings.site.private_site_description")}
        defaultChecked={settings.private_site}
        onChange={async (checked) => {
          await updateSettingsWithToast({ private_site: checked }, t);
          await refetch();
        }}
        className="km-setting-card"
      />
      {settings.private_site && (
        <>
          <SettingCardShortTextInput
            title={t("settings.site.admin_view_path")}
            description={t("settings.site.admin_view_path_description")}
            defaultValue={settings.admin_view_path || ""}
            placeholder="/view-xxxxxx"
            OnSave={async (data) => {
              let path = data.trim();
              if (!path) {
                toast.error(t("settings.site.admin_view_path_empty"));
                return;
              }
              if (!path.startsWith("/")) path = "/" + path;
              if (path === "/" || path === "/admin" || path === "/login" || path.startsWith("/api/")) {
                toast.error(t("settings.site.admin_view_path_invalid"));
                return;
              }
              await updateSettingsWithToast({ admin_view_path: path }, t);
              await refetch();
            }}
            className="km-setting-card"
          >
            <Button
              variant="soft"
              onClick={() => {
                const fullUrl = `${window.location.origin}${settings.admin_view_path || ""}`;
                navigator.clipboard.writeText(fullUrl);
                toast.success(t("common.copy", "已复制到剪贴板"));
              }}
            >
              {t("settings.site.admin_view_path_copy")}
            </Button>
          </SettingCardShortTextInput>

          <SettingCardShortTextInput
            title={t("settings.site.admin_path")}
            description={t("settings.site.admin_path_description")}
            defaultValue={settings.admin_path || ""}
            placeholder="/entry-xxxxxx"
            OnSave={async (data) => {
              let path = data.trim();
              if (!path) {
                toast.error(t("settings.site.admin_path_empty"));
                return;
              }
              if (!path.startsWith("/")) path = "/" + path;
              if (path === "/" || path === "/admin" || path === "/login" || path.startsWith("/api/")) {
                toast.error(t("settings.site.admin_path_invalid"));
                return;
              }
              await updateSettingsWithToast({ admin_path: path }, t);
              await refetch();
            }}
            className="km-setting-card"
          >
            <Button
              variant="soft"
              onClick={() => {
                const fullUrl = `${window.location.origin}${settings.admin_path || ""}`;
                navigator.clipboard.writeText(fullUrl);
                toast.success(t("common.copy", "已复制到剪贴板"));
              }}
            >
              {t("settings.site.admin_path_copy")}
            </Button>
          </SettingCardShortTextInput>
        </>
      )}
      <SettingCardCollapse
        title={t("settings.site.share_links", "对外只读分享链接")}
        description={t("settings.site.share_links_description", "为访客生成独立的只读监控专属页面，彻底隔离管理功能与登录入口。支持限时分享与永久分享。")}
      >
        {(() => {
          const isPermValid = Boolean(settings.permanent_share_token);
          const permShareLink = isPermValid
            ? `${window.location.origin}/s/${settings.permanent_share_token}`
            : "";
          const permDescription = isPermValid
            ? t("settings.site.permanent_share_valid", "永久有效（除非手动撤销）")
            : t("settings.site.permanent_share_none", "暂未生成永久分享链接");

          const isTempValid = Boolean(
            settings.tempory_share_token &&
              (settings.tempory_share_token_expire_at || 0) * 1000 > Date.now(),
          );
          const tempShareLink = isTempValid
            ? `${window.location.origin}/s/${settings.tempory_share_token}`
            : "";
          const tempExpireDescription = isTempValid
            ? `${t("admin.nodeTable.expiredAt")}: ${new Date(
                (settings.tempory_share_token_expire_at || 0) * 1000,
              ).toLocaleString()}`
            : t("settings.site.temporary_share_none", "暂无有效限时分享链接或已过期");

          return (
            <div className="flex w-full flex-col gap-6">
              {/* 永久分享 */}
              <div className="flex flex-col gap-3 rounded-lg border p-4">
                <div className="flex flex-col gap-1">
                  <span className="font-medium text-base">
                    {t("settings.site.permanent_share", "永久分享链接")}
                  </span>
                  <span className="text-sm text-muted-foreground">
                    {t("settings.site.permanent_share_description", "永久有效的只读监控页面，除非手动撤销否则永不过期。")}
                  </span>
                </div>
                <SettingCardShortTextInput
                  title=""
                  value={permShareLink}
                  placeholder={t("settings.site.permanent_share_none", "暂未生成永久分享链接")}
                  showSaveButton={false}
                  description={permDescription}
                  disabled
                  bordless
                >
                  <Button
                    disabled={!isPermValid}
                    onClick={() => {
                      if (!permShareLink) return;
                      navigator.clipboard.writeText(permShareLink);
                      toast.success(t("common.copy", "已复制到剪贴板"));
                    }}
                  >
                    {t("common.copy")}
                  </Button>
                </SettingCardShortTextInput>

                <div className="flex flex-row w-full gap-2">
                  <Button
                    onClick={async () => {
                      const key = Array.from(crypto.getRandomValues(new Uint8Array(32)), byte => byte.toString(16).padStart(2, "0")).join("");
                      await updateSettingsWithToast(
                        { permanent_share_token: key },
                        t,
                      );
                      await refetch();
                    }}
                  >
                    {t("settings.site.permanent_share_generate", "生成永久链接")}
                  </Button>
                  <Button
                    color="red"
                    variant="soft"
                    disabled={!settings.permanent_share_token}
                    onClick={async () => {
                      await updateSettingsWithToast(
                        { permanent_share_token: "" },
                        t,
                      );
                      await refetch();
                    }}
                  >
                    {t("settings.site.permanent_share_revoke", "撤销永久链接")}
                  </Button>
                </div>
              </div>

              {/* 限时分享 */}
              <div className="flex flex-col gap-3 rounded-lg border p-4">
                <div className="flex flex-col gap-1">
                  <span className="font-medium text-base">
                    {t("settings.site.temporary_share", "限时分享链接")}
                  </span>
                  <span className="text-sm text-muted-foreground">
                    {t("settings.site.temporary_share_description", "具备有效期的只读监控页面，到期后自动失效拒绝访问。")}
                  </span>
                </div>
                <SettingCardShortTextInput
                  title={t("settings.site.temporary_share_current_link", "当前限时分享链接")}
                  value={tempShareLink}
                  placeholder={t("settings.site.temporary_share_none", "暂无有效限时分享链接或已过期")}
                  showSaveButton={false}
                  description={tempExpireDescription}
                  disabled
                  bordless
                >
                  <Button
                    disabled={!isTempValid}
                    onClick={() => {
                      if (!tempShareLink) return;
                      navigator.clipboard.writeText(tempShareLink);
                      toast.success(t("common.copy", "已复制到剪贴板"));
                    }}
                  >
                    {t("common.copy")}
                  </Button>
                </SettingCardShortTextInput>

                <div className="flex flex-col gap-2">
                  <SettingCardShortTextInput
                    title={t("settings.site.temporary_share_hours", "分享时长（小时）")}
                    bordless
                    showSaveButton={false}
                    value={shareHours}
                    type="number"
                    onChange={(e) => {
                      const val = Number.parseInt(e.target.value, 10);
                      if (!Number.isNaN(val) && val > 0) {
                        setShareHours(val);
                      }
                    }}
                  />
                  <div className="flex flex-wrap gap-2 pt-1">
                    {[
                      { label: "1 小时", hours: 1 },
                      { label: "6 小时", hours: 6 },
                      { label: "24 小时", hours: 24 },
                      { label: "7 天", hours: 168 },
                      { label: "30 天", hours: 720 },
                    ].map((preset) => (
                      <Button
                        key={preset.hours}
                        size="1"
                        variant={shareHours === preset.hours ? "solid" : "soft"}
                        onClick={() => setShareHours(preset.hours)}
                      >
                        {preset.label}
                      </Button>
                    ))}
                  </div>
                </div>

                <div className="flex flex-row w-full gap-2">
                  <Button
                    onClick={async () => {
                      const key = Array.from(crypto.getRandomValues(new Uint8Array(32)), byte => byte.toString(16).padStart(2, "0")).join("");
                      const validHours = Math.max(1, shareHours);
                      await updateSettingsWithToast(
                        {
                          tempory_share_token: key,
                          tempory_share_token_expire_at:
                            Math.floor(Date.now() / 1000) + validHours * 3600,
                        },
                        t,
                      );
                      await refetch();
                    }}
                  >
                    {t("settings.site.temporary_share_generate", "生成限时链接")}
                  </Button>
                  <Button
                    color="red"
                    variant="soft"
                    disabled={!settings.tempory_share_token}
                    onClick={async () => {
                      await updateSettingsWithToast(
                        { tempory_share_token: "", tempory_share_token_expire_at: 0 },
                        t,
                      );
                      await refetch();
                    }}
                  >
                    {t("settings.site.temporary_share_revoke", "撤销限时链接")}
                  </Button>
                </div>
              </div>
            </div>
          );
        })()}
      </SettingCardCollapse>
      <SettingCardLabel>{t("settings.site.custom")}</SettingCardLabel>
      <label className="text-sm text-muted-foreground -mt-4">
        {t("settings.custom.note")}
      </label>
      <SettingCardLongTextInput
        title={t("settings.custom.header")}
        description={t("settings.custom.header_description")}
        defaultValue={settings.custom_head || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ custom_head: data }, t);
        }}
      />
      <SettingCardLongTextInput
        title={t("settings.custom.body", "自定义 Body")}
        description={t(
          "settings.custom.body_description",
          "在页面底部添加自定义内容",
        )}
        defaultValue={settings.custom_body || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ custom_body: data }, t);
        }}
      />
      <SettingCardCollapse
        title={t("settings.custom.favicon", "自定义 Favicon")}
        description={t(
          "settings.custom.favicon_description",
          "在浏览器标签页显示的图标",
        )}
        defaultOpen={true}
      >
        <Flex
          width={"100%"}
          justify="between"
          align="start"
          direction={"column"}
          gap="2"
        >
          <Flex gap="2" align="center">
            {t("settings.custom.favicon_current", "当前 Favicon")}
            <img
              src="/favicon.ico"
              alt="Favicon"
              style={{ width: 32, height: 32 }}
            />
          </Flex>
          <label className="text-sm text-muted-foreground">
            {t(
              "settings.custom.favicon_note",
              "Favicon 图标的更新速度可能较慢，通常需要清除浏览器缓存后才能看到更改。",
            )}
          </label>
          <Flex gap="2" align="center">
            <Dialog.Root>
              <Dialog.Trigger>
                <Button color="tomato">
                  {t("settings.custom.favicon_default", "恢复默认")}
                </Button>
              </Dialog.Trigger>
              <Dialog.Content>
                <Dialog.Title>
                  {t("settings.custom.favicon_default", "恢复默认")}
                </Dialog.Title>
                <Dialog.Description>
                  {t(
                    "settings.custom.favicon_default_description",
                    "这将恢复默认的 Favicon 图标，是否继续？",
                  )}
                </Dialog.Description>
                <Flex gap="2" justify="end">
                  <Dialog.Close>
                    <Button variant="soft">{t("common.cancel", "取消")}</Button>
                  </Dialog.Close>
                  <Dialog.Trigger>
                    <Button
                      color="red"
                      onClick={async () => {
                        fetch("/api/admin/update/favicon", {
                          method: "POST",
                        })
                          .then((response) => {
                            return response.json();
                          })
                          .then((data) => {
                            if (data.status === "success") {
                              toast.success(t("settings.custom.favicon_default_success"));
                            } else {
                              toast.error(
                                data.message || t("settings.custom.favicon_default_error"),
                              );
                            }
                          })
                          .catch((error) => {
                            toast.error("" + error);
                          });
                      }}
                    >
                      {t("common.confirm")}
                    </Button>
                  </Dialog.Trigger>
                </Flex>
              </Dialog.Content>
            </Dialog.Root>
            <Button
              onClick={async () => {
                const input = document.createElement("input");
                input.type = "file";
                input.accept = "image/*";
                input.onchange = async (e) => {
                  const file = (e.target as HTMLInputElement).files?.[0];
                  if (file) {
                    try {
                      const response = await fetch(
                        "/api/admin/update/favicon",
                        {
                          method: "PUT",
                          body: file,
                          headers: {
                            "Content-Type": "application/octet-stream",
                          },
                        },
                      );
                      const data = await response.json();
                      if (data.status === "success") {
                        toast.success(
                          t(
                            "settings.custom.favicon_update_success"
                          ),
                        );
                      } else {
                        toast.error(data.message || "Failed to update Favicon");
                      }
                    } catch (error) {
                      toast.error("" + error);
                    }
                  }
                };
                input.click();
              }}
            >
              {t("settings.custom.favicon_change")}
            </Button>
          </Flex>
        </Flex>
      </SettingCardCollapse>
      <SettingCardLabel>{t("settings.site.backup")}</SettingCardLabel>
      <SettingCardIconButton
        title={t("settings.site.backup_download")}
        description={t("settings.site.backup_download_description")}
        onClick={async () => {
          try { await authorizeSensitiveAccess(); window.location.assign("/api/admin/download/backup"); }
          catch (error) { toast.error(String(error)); }
        }}
        className="km-setting-card"
      >
        <DownloadIcon size={16} />
      </SettingCardIconButton>
      <SettingCardButton
        title={t("settings.site.backup_restore")}
        description={t("settings.site.backup_restore_description")}
        onClick={() => setRestoreOpen(true)}
        className="km-setting-card"
      >
        {t("common.select")}
      </SettingCardButton>

      {/* 上传备份对话框 */}
      <UploadDialog
        open={restoreOpen}
        onOpenChange={(open) => {
          if (!open && restoring) {
            cancelRestore();
            return;
          }
          setRestoreOpen(open);
        }}
        title={t("settings.site.backup_restore")}
        description={t("settings.site.backup_restore_description")}
        accept=".zip"
        dragDropText={t("theme.drag_drop")}
        clickToBrowseText={t("theme.or_click_to_browse")}
        hintText={t("theme.zip_files_only")}
        uploading={restoring}
        progress={restoreProgress}
        cancelUploadLabel={t("common.cancel")}
        onCancelUpload={cancelRestore}
        onFileSelected={(file) => uploadBackup(file)}
        closeLabel={t("common.cancel")}
      />
    </>
  );
}
