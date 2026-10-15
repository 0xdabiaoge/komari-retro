import React, { useMemo, useRef, useState } from "react";
import { Box, Button, Flex, Text, TextArea } from "@radix-ui/themes";
import { toast } from "sonner";
import { Copy, Check, Eye, RotateCcw, Send } from "lucide-react";
import { SettingCard } from "@/components/admin/SettingCard";
import {
  DEFAULT_NOTIFICATION_TEMPLATE,
  NOTIFICATION_PRESETS,
  NOTIFICATION_SCENARIOS,
  NOTIFICATION_VARIABLES,
  notificationSampleData,
  previewNotification,
} from "@/utils/notificationTemplate";
interface Props {
  defaultValue?: string;
  onSave: (value: string) => Promise<void>;
}
export function NotificationTemplateCard({ defaultValue, onSave }: Props) {
  const [value, setValue] = useState(
    defaultValue ?? DEFAULT_NOTIFICATION_TEMPLATE,
  );
  const [saved, setSaved] = useState(value),
    [saving, setSaving] = useState(false),
    [scenario, setScenario] = useState("offline"),
    [batch, setBatch] = useState(false),
    [copied, setCopied] = useState(false);
  const editor = useRef<HTMLTextAreaElement>(null);
  React.useEffect(() => {
    if (defaultValue !== undefined) {
      setValue(defaultValue);
      setSaved(defaultValue);
    }
  }, [defaultValue]);
  const sample = useMemo(
    () => notificationSampleData(scenario, batch),
    [scenario, batch],
  );
  const preview = useMemo(
    () => previewNotification(value, sample),
    [value, sample],
  );
  const [expandedVariables, setExpandedVariables] = useState(false);
  const variables = NOTIFICATION_VARIABLES.filter(
    (v) =>
      expandedVariables ||
      ["emoji", "status", "message", "client", "time", "site_name"].includes(
        v.key,
      ),
  );
  const groups = [...new Set(variables.map((v) => v.group))];
  function insert(open: string, close = "", fallback = "") {
    const el = editor.current,
      start = el?.selectionStart ?? value.length,
      end = el?.selectionEnd ?? value.length,
      selection = value.slice(start, end) || fallback;
    setValue(
      value.slice(0, start) + open + selection + close + value.slice(end),
    );
    requestAnimationFrame(() => {
      el?.focus();
      el?.setSelectionRange(
        start + open.length,
        start + open.length + selection.length,
      );
    });
  }
  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      toast.success("已复制模板");
      setTimeout(() => setCopied(false), 1600);
    } catch {
      toast.error("复制失败，请手动选择模板后复制");
    }
  }
  async function save() {
    if (preview.errors.length) return;
    setSaving(true);
    try {
      await onSave(value);
      setSaved(value);
      toast.success("通知模板已保存");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }
  return (
    <SettingCard
      title="Telegram 通知模板自定义"
      description="使用 Telegram 支持的文字格式组织告警。预览采用示例数据，不会发送消息；已有模板不会被自动覆盖。"
    >
      <Flex
        direction="column"
        gap="4"
        className="w-full mt-2"
        data-testid="notification-template-editor"
      >
        <div className="grid grid-cols-2 xl:grid-cols-4 gap-2">
          {NOTIFICATION_PRESETS.map((p, i) => (
            <button
              key={p.id}
              type="button"
              aria-pressed={value === p.template}
              onClick={() => {
                setValue(p.template);
                if (p.id === "batch") setBatch(true);
              }}
              className={`rounded-xl border p-3 text-left transition-colors ${value === p.template ? "border-sky-500 bg-sky-500/8" : "border-border hover:bg-muted/50"}`}
            >
              <span className="flex items-center gap-2 font-semibold text-sm">
                <span>{["✨", "🛠", "⚡", "📋"][i]}</span>
                {p.name}
                {i === 0 ? (
                  <span className="text-[10px] text-sky-600 bg-sky-500/10 rounded px-1.5 py-0.5">
                    推荐
                  </span>
                ) : null}
              </span>
              <span className="mt-1 block text-xs text-muted-foreground leading-relaxed">
                {p.description}
              </span>
            </button>
          ))}
        </div>
        <Box className="rounded-xl border border-border bg-muted/20 p-3">
          <Text as="div" size="2" weight="medium" className="mb-3">
            点击插入变量
          </Text>
          <Flex direction="column" gap="3">
            {groups.map((group) => (
              <div key={group} className="flex flex-wrap items-center gap-1.5">
                <span className="w-full sm:w-20 shrink-0 text-xs text-muted-foreground">
                  {group}
                </span>
                {variables
                  .filter((v) => v.group === group)
                  .map((v) => (
                    <button
                      key={v.key}
                      type="button"
                      title={v.description}
                      onClick={() => insert(`{{${v.key}}}`)}
                      className="rounded-md border border-border bg-background px-2 py-1 text-xs hover:border-sky-400 hover:text-sky-600 transition-colors"
                    >
                      <span>{v.label}</span>
                      <code className="ml-1.5 opacity-55 text-[10px]">{`{{${v.key}}}`}</code>
                    </button>
                  ))}
              </div>
            ))}
          </Flex>
          <Button
            type="button"
            size="1"
            variant="ghost"
            color="gray"
            className="mt-2"
            onClick={() => setExpandedVariables((v) => !v)}
          >
            {expandedVariables
              ? "收起高级变量"
              : `展开全部变量（${NOTIFICATION_VARIABLES.length}）`}
          </Button>
        </Box>
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-4 min-w-0">
          <Flex direction="column" gap="2" className="min-w-0">
            <Flex justify="between" align="center" gap="2" wrap="wrap">
              <Text weight="bold" size="2">
                模板编辑
              </Text>
              <Flex gap="2">
                <Button
                  type="button"
                  size="1"
                  color="gray"
                  variant="outline"
                  onClick={copy}
                >
                  {copied ? <Check size={13} /> : <Copy size={13} />}复制模板
                </Button>
                <Button
                  type="button"
                  size="1"
                  color="gray"
                  variant="outline"
                  onClick={() => setValue(DEFAULT_NOTIFICATION_TEMPLATE)}
                >
                  <RotateCcw size={13} />
                  重置默认
                </Button>
              </Flex>
            </Flex>
            <Flex
              gap="1"
              wrap="wrap"
              className="rounded-lg border border-border p-1.5"
            >
              {[
                { label: "加粗", open: "<b>", close: "</b>", text: "重点内容" },
                {
                  label: "代码",
                  open: "<code>",
                  close: "</code>",
                  text: "节点地址",
                },
                {
                  label: "引用",
                  open: "<blockquote>",
                  close: "</blockquote>",
                  text: "{{message}}",
                },
                {
                  label: "隐藏文本",
                  open: "<tg-spoiler>",
                  close: "</tg-spoiler>",
                  text: "隐藏内容",
                },
                {
                  label: "链接",
                  open: '<a href="https://example.com">',
                  close: "</a>",
                  text: "查看详情",
                },
              ].map((f) => (
                <Button
                  key={f.label}
                  type="button"
                  size="1"
                  variant="ghost"
                  color="gray"
                  onClick={() => insert(f.open, f.close, f.text)}
                >
                  {f.label}
                </Button>
              ))}
            </Flex>
            <TextArea
              aria-label="通知模板代码"
              ref={editor}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              className="w-full font-mono text-sm leading-relaxed"
              style={{ minHeight: 330 }}
              spellCheck={false}
              placeholder="输入通知模板，或选择上方预设"
            />
            <Flex justify="between" gap="2" wrap="wrap">
              <Text size="1" color="gray">
                {value !== saved ? "有尚未保存的修改" : "当前模板已保存"}
              </Text>
              <Text size="1" color={preview.length > 4096 ? "red" : "gray"}>
                示例正文 {preview.length} / 4096 字符
              </Text>
            </Flex>
            {preview.errors.length ? (
              <div
                role="alert"
                className="rounded-lg border border-red-500/30 bg-red-500/5 p-3 text-xs text-red-600"
              >
                {preview.errors.map((e) => (
                  <p key={e}>{e}</p>
                ))}
              </div>
            ) : null}
            {preview.warnings.length ? (
              <div
                role="status"
                className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-3 text-xs text-amber-700 dark:text-amber-400"
              >
                {preview.warnings.map((w) => (
                  <p key={w}>{w}</p>
                ))}
              </div>
            ) : null}
            <Text size="1" color="gray">
              只支持文字排版，不支持网页
              CSS、表格或自定义颜色。实际消息长度随节点和事件内容变化。
            </Text>
          </Flex>
          <Flex direction="column" gap="2" className="min-w-0">
            <Text weight="bold" size="2" className="flex items-center gap-1.5">
              <Eye size={15} className="text-sky-500" />
              消息效果预览{" "}
              <span className="text-[10px] font-normal rounded bg-muted px-1.5 py-0.5 text-muted-foreground">
                示例数据
              </span>
            </Text>
            <Flex gap="1" wrap="wrap">
              {NOTIFICATION_SCENARIOS.map((s) => (
                <Button
                  type="button"
                  size="1"
                  variant={scenario === s.id ? "solid" : "soft"}
                  color={scenario === s.id ? "blue" : "gray"}
                  key={s.id}
                  onClick={() => setScenario(s.id)}
                >
                  {s.emoji} {s.name}
                </Button>
              ))}
            </Flex>
            <label className="text-xs text-muted-foreground flex items-center gap-1.5">
              <input
                type="checkbox"
                checked={batch}
                onChange={(e) => setBatch(e.target.checked)}
              />
              多节点示例
            </label>
            <div
              className="flex-1 rounded-xl border border-sky-500/20 p-3 sm:p-4 bg-gradient-to-br from-sky-100/60 via-slate-100/30 to-emerald-100/30 dark:from-slate-900 dark:via-slate-900 dark:to-slate-800"
              style={{ minHeight: 330 }}
            >
              <div className="flex items-start gap-2.5 min-w-0">
                <div className="w-8 h-8 shrink-0 rounded-full bg-sky-500 text-white flex items-center justify-center shadow-sm">
                  <Send size={15} />
                </div>
                <div className="min-w-0 flex-1 rounded-2xl rounded-tl-sm border border-border/60 bg-background p-3.5 shadow-sm">
                  <div className="flex gap-1.5 items-center mb-2">
                    <span className="font-semibold text-xs text-sky-600 dark:text-sky-400">
                      Komari Retro 通知
                    </span>
                    <span className="text-[9px] rounded bg-sky-500/10 text-sky-600 px-1">
                      机器人
                    </span>
                  </div>
                  <div
                    data-testid="notification-preview"
                    className="text-sm leading-relaxed whitespace-pre-wrap break-words [overflow-wrap:anywhere]"
                    dangerouslySetInnerHTML={{ __html: preview.html }}
                  />
                  <div className="mt-3 text-right text-[10px] text-muted-foreground">
                    09:41 <span className="text-sky-500">✓✓</span>
                  </div>
                </div>
              </div>
              <Text
                as="p"
                size="1"
                className="mt-4 text-center text-muted-foreground"
              >
                本地格式模拟，最终显示以 Telegram
                客户端为准。切换示例不会发送通知。
              </Text>
            </div>
          </Flex>
        </div>
        <Flex justify="end">
          <Button
            type="button"
            disabled={saving || preview.errors.length > 0}
            onClick={save}
          >
            {saving ? "正在保存…" : "保存通知模板"}
          </Button>
        </Flex>
      </Flex>
    </SettingCard>
  );
}
export default NotificationTemplateCard;
