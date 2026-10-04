import React, { useState, useRef, useMemo } from "react";
import { Box, Button, Flex, Text, TextArea } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  Sparkles,
  RotateCcw,
  Eye,
  Check,
  Send,
  HelpCircle,
  Copy,
} from "lucide-react";
import { SettingCard } from "@/components/admin/SettingCard";

const DEFAULT_TEMPLATE = `{{emoji}} <b>【Komari 监控告警】</b> {{emoji}}
━━━━━━━━━━━━━━━
📌 <b>告警事件</b>: <code>{{status}}</code>
🖥 <b>监控节点</b>: <b>{{client}}</b>
🌐 <b>节点网络</b>: <code>{{ip}}</code> ({{region}})
📝 <b>告警详情</b>: {{message}}
⏰ <b>告警时间</b>: <code>{{time}}</code>
━━━━━━━━━━━━━━━
🔔 <i>来自 {{site_name}} 监控平台</i>`;

const PRESETS = [
  {
    name: "🌟 精美卡片 (推荐)",
    template: DEFAULT_TEMPLATE,
  },
  {
    name: "📋 详细列表",
    template: `{{emoji}} <b>【{{site_name}} · {{status}}】</b>
────────────────────
• <b>目标节点:</b> <code>{{client}}</code>
• <b>网络地址:</b> <code>{{ip}}</code>
• <b>所属地区:</b> <code>{{region}}</code>
• <b>系统版本:</b> <code>{{os}}</code>
• <b>分组标签:</b> <code>{{group}}</code>
• <b>详细情况:</b> {{message}}
• <b>触发时间:</b> <code>{{time}}</code>
────────────────────`,
  },
  {
    name: "⚡ 极简一行",
    template: `{{emoji}} [{{site_name}}] <b>{{status}}</b> - {{client}} ({{ip}})\n消息: {{message}} ({{time}})`,
  },
];

const VARIABLES = [
  { key: "{{emoji}}", label: "状态表情", desc: "匹配告警图标 🔴/🟢/📈/📊/⏰/🔐" },
  { key: "{{status}}", label: "状态文本", desc: "友好状态描述 (如: 节点离线 ⚠️)" },
  { key: "{{client}}", label: "节点名称", desc: "发生事件的服务器节点" },
  { key: "{{ip}}", label: "节点 IP", desc: "节点公网 IPv4 / IPv6" },
  { key: "{{region}}", label: "所属地区", desc: "节点地理地区 (如: 香港, 日本)" },
  { key: "{{os}}", label: "操作系统", desc: "系统版本 (如: Debian 12)" },
  { key: "{{group}}", label: "节点分组", desc: "节点所属分组" },
  { key: "{{message}}", label: "告警详情", desc: "具体告警指标或详情描述" },
  { key: "{{time}}", label: "触发时间", desc: "时间 (2006-01-02 15:04:05)" },
  { key: "{{date}}", label: "触发日期", desc: "日期 (2006-01-02)" },
  { key: "{{site_name}}", label: "站点名称", desc: "平台站点名称" },
  { key: "{{event}}", label: "原始事件", desc: "事件代码 (offline, online, load 等)" },
];

const MOCK_SCENARIOS = {
  offline: {
    name: "🔴 节点离线",
    data: {
      emoji: "🔴",
      status: "节点离线 ⚠️",
      status_text: "节点离线 ⚠️",
      client: "HK-BGP-Node-01",
      client_name: "HK-BGP-Node-01",
      ip: "103.21.244.18",
      client_ip: "103.21.244.18",
      region: "中国香港",
      os: "Debian 12",
      group: "亚太高防组",
      message: "节点心跳失联超过 60 秒，无网络响应",
      time: "2026-10-04 14:15:30",
      date: "2026-10-04",
      raw_time: "2026-10-04T14:15:30Z",
      site_name: "Komari Retro",
      server: "Komari Retro",
      event: "offline",
    },
  },
  online: {
    name: "🟢 节点上线",
    data: {
      emoji: "🟢",
      status: "节点恢复上线 ✅",
      status_text: "节点恢复上线 ✅",
      client: "HK-BGP-Node-01",
      client_name: "HK-BGP-Node-01",
      ip: "103.21.244.18",
      client_ip: "103.21.244.18",
      region: "中国香港",
      os: "Debian 12",
      group: "亚太高防组",
      message: "网络与心跳已恢复正常，当前延迟 18ms",
      time: "2026-10-04 14:18:02",
      date: "2026-10-04",
      raw_time: "2026-10-04T14:18:02Z",
      site_name: "Komari Retro",
      server: "Komari Retro",
      event: "online",
    },
  },
  load: {
    name: "📈 系统高负载",
    data: {
      emoji: "📈",
      status: "系统高负载预警 📈",
      status_text: "系统高负载预警 📈",
      client: "US-LA-Node-02",
      client_name: "US-LA-Node-02",
      ip: "198.51.100.42",
      client_ip: "198.51.100.42",
      region: "美国洛杉矶",
      os: "Ubuntu 24.04",
      group: "美洲核心组",
      message: "CPU 使用率持续高于 90.0% (当前 95.8%)",
      time: "2026-10-04 14:20:15",
      date: "2026-10-04",
      raw_time: "2026-10-04T14:20:15Z",
      site_name: "Komari Retro",
      server: "Komari Retro",
      event: "load",
    },
  },
  test: {
    name: "🚀 测试通知",
    data: {
      emoji: "🚀",
      status: "通知测试 🚀",
      status_text: "通知测试 🚀",
      client: "Komari-Master",
      client_name: "Komari-Master",
      ip: "127.0.0.1",
      client_ip: "127.0.0.1",
      region: "本机系统",
      os: "Linux x86_64",
      group: "管理集群",
      message: "这是一条来自 Komari 监控的测试消息，通知配置正常！",
      time: "2026-10-04 14:22:00",
      date: "2026-10-04",
      raw_time: "2026-10-04T14:22:00Z",
      site_name: "Komari Retro",
      server: "Komari Retro",
      event: "test",
    },
  },
};

function renderTelegramHTML(template: string, data: Record<string, string>): string {
  let rendered = template;
  for (const [key, val] of Object.entries(data)) {
    rendered = rendered.split(`{{${key}}}`).join(val);
  }

  // Escape HTML but allow valid Telegram tags
  let safe = rendered
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");

  safe = safe
    .replace(/&lt;b&gt;/gi, "<b>")
    .replace(/&lt;\/b&gt;/gi, "</b>")
    .replace(/&lt;strong&gt;/gi, "<strong>")
    .replace(/&lt;\/strong&gt;/gi, "</strong>")
    .replace(/&lt;i&gt;/gi, "<i>")
    .replace(/&lt;\/i&gt;/gi, "</i>")
    .replace(/&lt;em&gt;/gi, "<em>")
    .replace(/&lt;\/em&gt;/gi, "</em>")
    .replace(/&lt;code&gt;/gi, '<code class="bg-black/10 dark:bg-white/10 px-1 py-0.5 rounded text-[13px] font-mono">')
    .replace(/&lt;\/code&gt;/gi, "</code>")
    .replace(/&lt;pre&gt;/gi, '<pre class="bg-black/15 dark:bg-white/15 p-2 rounded text-xs font-mono my-1 overflow-x-auto">')
    .replace(/&lt;\/pre&gt;/gi, "</pre>")
    .replace(/&lt;s&gt;/gi, "<s>")
    .replace(/&lt;\/s&gt;/gi, "</s>")
    .replace(/&lt;del&gt;/gi, "<del>")
    .replace(/&lt;\/del&gt;/gi, "</del>")
    .replace(/&lt;u&gt;/gi, "<u>")
    .replace(/&lt;\/u&gt;/gi, "</u>")
    .replace(/&lt;a href="([^"]*)"&gt;/gi, '<a href="$1" target="_blank" rel="noopener noreferrer" class="text-sky-500 underline">')
    .replace(/&lt;\/a&gt;/gi, "</a>");

  safe = safe.replace(/\n/g, "<br/>");
  return safe;
}

interface NotificationTemplateCardProps {
  defaultValue?: string;
  onSave: (value: string) => Promise<void>;
}

export const NotificationTemplateCard: React.FC<NotificationTemplateCardProps> = ({
  defaultValue,
  onSave,
}) => {
  const { t } = useTranslation();
  const [value, setValue] = useState(defaultValue || DEFAULT_TEMPLATE);
  const [saving, setSaving] = useState(false);
  const [scenario, setScenario] = useState<keyof typeof MOCK_SCENARIOS>("offline");
  const [copied, setCopied] = useState(false);
  const textAreaRef = useRef<HTMLTextAreaElement>(null);

  React.useEffect(() => {
    if (defaultValue) {
      setValue(defaultValue);
    }
  }, [defaultValue]);

  const insertVariable = (varKey: string) => {
    const el = textAreaRef.current;
    if (!el) {
      setValue((prev) => prev + varKey);
      return;
    }
    const start = el.selectionStart;
    const end = el.selectionEnd;
    const current = el.value;
    const updated = current.substring(0, start) + varKey + current.substring(end);
    setValue(updated);
    setTimeout(() => {
      el.focus();
      el.setSelectionRange(start + varKey.length, start + varKey.length);
    }, 0);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(value);
      toast.success(t("common.success"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    toast.success("已复制模板内容");
    setTimeout(() => setCopied(false), 2000);
  };

  const currentMockData = useMemo(() => {
    return MOCK_SCENARIOS[scenario].data;
  }, [scenario]);

  const renderedPreviewHTML = useMemo(() => {
    return renderTelegramHTML(value, currentMockData);
  }, [value, currentMockData]);

  return (
    <SettingCard
      title="Telegram 通知模板自定义"
      description="支持自定义消息排版与 Telegram HTML 格式。点击标签可快速插入变量，右侧为真实 Telegram 消息气泡实时渲染预览。"
    >
      <Flex direction="column" gap="4" className="w-full mt-2">
        {/* 预设与快捷操作工具栏 */}
        <Flex justify="between" align="center" wrap="wrap" gap="2">
          <Flex align="center" gap="2" wrap="wrap">
            <Text size="2" weight="bold" className="text-muted-foreground flex items-center gap-1">
              <Sparkles size={14} className="text-amber-500" />
              预设模板:
            </Text>
            {PRESETS.map((preset) => (
              <Button
                key={preset.name}
                size="1"
                variant="soft"
                type="button"
                onClick={() => setValue(preset.template)}
              >
                {preset.name}
              </Button>
            ))}
          </Flex>
          <Flex align="center" gap="2">
            <Button
              size="1"
              variant="outline"
              color="gray"
              type="button"
              onClick={handleCopy}
            >
              {copied ? <Check size={14} /> : <Copy size={14} />}
              复制
            </Button>
            <Button
              size="1"
              variant="outline"
              color="gray"
              type="button"
              onClick={() => setValue(DEFAULT_TEMPLATE)}
            >
              <RotateCcw size={14} />
              重置默认
            </Button>
          </Flex>
        </Flex>

        {/* 可点击插入变量列表 */}
        <Box className="p-3 rounded-lg border border-border/50 bg-muted/30">
          <Text size="2" weight="medium" className="text-muted-foreground mb-2 block">
            点击插入变量：
          </Text>
          <Flex wrap="wrap" gap="2">
            {VARIABLES.map((v) => (
              <button
                key={v.key}
                type="button"
                onClick={() => insertVariable(v.key)}
                title={v.desc}
                className="group inline-flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-md bg-secondary/80 hover:bg-primary hover:text-primary-foreground border border-border/60 transition-all cursor-pointer shadow-xs"
              >
                <code className="font-mono font-semibold">{v.key}</code>
                <span className="opacity-75 group-hover:opacity-100">({v.label})</span>
              </button>
            ))}
          </Flex>
        </Box>

        {/* 编辑区与实时预览区 Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* 左侧：模板编辑器 */}
          <Flex direction="column" gap="2">
            <Flex justify="between" align="center">
              <Text size="2" weight="bold">模板代码编辑</Text>
              <Text size="1" color="gray">支持 HTML 标签: &lt;b&gt;, &lt;code&gt;, &lt;i&gt;, &lt;a&gt;</Text>
            </Flex>
            <TextArea
              ref={textAreaRef}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              className="w-full font-mono text-sm leading-relaxed"
              style={{ minHeight: "260px" }}
              placeholder="请输入 Telegram 消息模板..."
            />
          </Flex>

          {/* 右侧：Telegram 真实气泡预览 */}
          <Flex direction="column" gap="2">
            <Flex justify="between" align="center">
              <Text size="2" weight="bold" className="flex items-center gap-1">
                <Eye size={16} className="text-sky-500" />
                Telegram 实时效果预览
              </Text>
              <Flex gap="1">
                {(Object.keys(MOCK_SCENARIOS) as Array<keyof typeof MOCK_SCENARIOS>).map((key) => (
                  <Button
                    key={key}
                    size="1"
                    variant={scenario === key ? "solid" : "ghost"}
                    color={scenario === key ? "blue" : "gray"}
                    type="button"
                    onClick={() => setScenario(key)}
                  >
                    {MOCK_SCENARIOS[key].name}
                  </Button>
                ))}
              </Flex>
            </Flex>

            {/* Telegram 消息气泡模拟卡片 */}
            <div className="flex-1 rounded-xl border border-sky-500/20 bg-linear-to-b from-sky-500/5 to-transparent p-4 flex flex-col justify-between" style={{ minHeight: "260px" }}>
              <div className="flex items-start gap-3">
                {/* Bot 头像 */}
                <div className="w-10 h-10 rounded-full bg-gradient-to-tr from-sky-500 to-blue-600 flex items-center justify-center text-white font-bold text-sm shadow-sm shrink-0">
                  <Send size={18} className="translate-x-[-1px] translate-y-[-1px]" />
                </div>

                {/* 气泡本体 */}
                <div className="flex-1 bg-card dark:bg-[#1f2937]/90 border border-border/80 rounded-2xl rounded-tl-xs p-3.5 shadow-sm text-sm leading-relaxed text-foreground select-text">
                  {/* Bot 头部 */}
                  <div className="flex items-center gap-1.5 mb-2">
                    <span className="font-semibold text-sky-600 dark:text-sky-400 text-xs">
                      Komari Retro Bot
                    </span>
                    <span className="text-[9px] px-1 py-0.2 rounded bg-sky-500/15 text-sky-600 dark:text-sky-400 font-bold uppercase tracking-wider">
                      BOT
                    </span>
                  </div>

                  {/* 消息正文渲染 */}
                  <div
                    className="space-y-1 break-words font-sans"
                    dangerouslySetInnerHTML={{ __html: renderedPreviewHTML }}
                  />

                  {/* 发送时间戳与已读勾选 */}
                  <div className="mt-2 flex justify-end items-center gap-1 text-[11px] text-muted-foreground select-none">
                    <span>{currentMockData.time.slice(11, 16)}</span>
                    <span className="text-sky-500 font-bold">✓✓</span>
                  </div>
                </div>
              </div>

              {/* 底部提示 */}
              <div className="mt-3 text-[11px] text-muted-foreground flex items-center gap-1 justify-center">
                <HelpCircle size={12} />
                <span>提示: 当前预览已自动模拟所选事件变量替换</span>
              </div>
            </div>
          </Flex>
        </div>

        {/* 保存按钮 */}
        <Flex justify="end" className="pt-2">
          <Button
            size="2"
            variant="solid"
            type="button"
            disabled={saving}
            onClick={handleSave}
          >
            {saving ? t("common.saving") : t("common.save")}
          </Button>
        </Flex>
      </Flex>
    </SettingCard>
  );
};

export default NotificationTemplateCard;
