import catalog from "../../../pkg/notification/catalog.json";
export const NOTIFICATION_PRESETS = catalog.presets;
export const NOTIFICATION_VARIABLES = catalog.variables;
export const DEFAULT_NOTIFICATION_TEMPLATE = catalog.presets[0].template;
export const VARIABLE_ALIASES: Record<string, string> = {
  client_name: "client",
  client_ip: "ip",
  status_text: "status",
  server: "site_name",
};
export function escapeNotificationHTML(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&#34;")
    .replace(/'/g, "&#39;");
}
export function substituteNotificationTemplate(
  template: string,
  data: Record<string, string>,
): string {
  return template.replace(
    /\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}/g,
    (original, key: string) => {
      const value = data[VARIABLE_ALIASES[key] || key];
      return value === undefined ? original : escapeNotificationHTML(value);
    },
  );
}
export const NOTIFICATION_SCENARIOS = [
  {
    id: "offline",
    name: "节点离线",
    emoji: "🔴",
    status: "节点离线 ⚠️",
    message: "节点心跳失联超过 60 秒，暂未收到新的上报。",
  },
  {
    id: "online",
    name: "恢复在线",
    emoji: "🟢",
    status: "节点恢复上线 ✅",
    message: "节点已重新连接，监控数据恢复上报。",
  },
  {
    id: "load",
    name: "负载告警",
    emoji: "📈",
    status: "系统高负载预警 📈",
    message: "CPU 使用率持续高于告警阈值，请检查运行中的任务。",
  },
  {
    id: "traffic_report",
    name: "流量报告",
    emoji: "📑",
    status: "月度流量报告 📑",
    message: "本月统计：上传 128 GB，下载 256 GB。",
  },
  {
    id: "test",
    name: "测试通知",
    emoji: "🚀",
    status: "通知测试 🚀",
    message: "这是一条示例通知，用于检查模板排版。",
  },
];
export function notificationSampleData(
  id: string,
  batch: boolean,
): Record<string, string> {
  const scenario =
    NOTIFICATION_SCENARIOS.find((s) => s.id === id) ||
    NOTIFICATION_SCENARIOS[0];
  return {
    emoji: scenario.emoji,
    status: scenario.status,
    message: scenario.message,
    event: scenario.id,
    client: batch ? "香港节点-01, 东京节点-02" : "香港节点-01",
    client_count: batch ? "2" : "1",
    client_list: batch
      ? "• 香港节点-01 — 192.0.2.18（中国香港）\n• 东京节点-02 — 2001:db8::24（日本东京）"
      : "• 香港节点-01 — 192.0.2.18（中国香港）",
    ip: batch ? "192.0.2.18, 2001:db8::24" : "192.0.2.18",
    ipv4: "192.0.2.18",
    ipv6: "2001:db8::24",
    region: batch ? "中国香港, 日本东京" : "中国香港",
    os: batch ? "Debian 12, Ubuntu 24.04" : "Debian 12",
    arch: batch ? "amd64, arm64" : "amd64",
    group: "生产节点",
    site_name: "Komari Retro",
    time: "2026-10-16 09:41:00",
    date: "2026-10-16",
    raw_time: "2026-10-16T09:41:00+08:00",
    timezone: "UTC+08:00",
  };
}
// Only emit tags and attributes created here. Raw HTML from variables stays
// escaped; unsupported tags/links are displayed as text and never executed.
export function previewNotification(
  template: string,
  data: Record<string, string>,
) {
  const expanded = substituteNotificationTemplate(template, data);
  const errors: string[] = [];
  const known = new Set([
    ...NOTIFICATION_VARIABLES.map((v) => v.key),
    ...Object.keys(VARIABLE_ALIASES),
    "raw_time",
  ]);
  const unknown = [
    ...template.matchAll(/\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}/g),
  ]
    .map((m) => m[1])
    .filter((key) => !known.has(key));
  const stack: string[] = [];
  const basic = new Set([
    "b",
    "strong",
    "i",
    "em",
    "u",
    "ins",
    "s",
    "strike",
    "del",
    "code",
    "pre",
    "blockquote",
    "tg-spoiler",
    "span",
    "a",
  ]);
  const html = expanded.replace(/<[^>]*(?:>|$)/g, (raw) => {
    const parsed = raw.match(/^<(\/?)([a-z][a-z0-9-]*)([^>]*)>$/i);
    if (!parsed) {
      errors.push("存在未闭合的 HTML 标签");
      return escapeNotificationHTML(raw);
    }
    const [, closing, original, attributes] = parsed,
      tag = original.toLowerCase();
    if (!basic.has(tag)) {
      errors.push(`不支持的标签：${tag}`);
      return escapeNotificationHTML(raw);
    }
    if (closing) {
      if (attributes.trim() || stack.pop() !== tag)
        errors.push(`标签闭合不匹配：${tag}`);
      return `</${tag === "tg-spoiler" ? "span" : tag}>`;
    }
    const attrs = attributes.trim();
    let safeAttrs = "";
    let outputTag = tag;
    if (tag === "a") {
      const link = attrs.match(/^href\s*=\s*(?:"([^"]*)"|'([^']*)')$/i);
      if (!link) {
        errors.push("链接需要使用带引号的 href 地址");
        return escapeNotificationHTML(raw);
      }
      const decoder = document.createElement("textarea");
      decoder.innerHTML = link[1] ?? link[2];
      const href = decoder.value;
      if (!/^(https?:\/\/|tg:\/\/user\?id=\d+$)/i.test(href)) {
        errors.push("链接仅支持 http、https 或 Telegram 用户链接");
        return escapeNotificationHTML(raw);
      }
      safeAttrs = ` href="${escapeNotificationHTML(href)}" target="_blank" rel="noopener noreferrer" class="text-sky-600 underline"`;
    } else if (tag === "span" || tag === "tg-spoiler") {
      if (tag === "span" && !/^class\s*=\s*["']tg-spoiler["']$/i.test(attrs))
        errors.push("span 仅支持 tg-spoiler 隐藏文本");
      if (tag === "tg-spoiler" && attrs)
        errors.push("隐藏文本标签不支持额外属性");
      outputTag = "span";
      safeAttrs =
        ' class="rounded bg-slate-500/20 px-1 blur-[3px] hover:blur-none transition-[filter]" title="悬停查看隐藏内容"';
    } else if (tag === "blockquote") {
      if (attrs && attrs !== "expandable")
        errors.push("引用块仅支持 expandable 属性");
      if (stack.includes("blockquote")) errors.push("引用块不能嵌套");
      safeAttrs =
        ' class="border-l-3 border-sky-500/60 bg-sky-500/5 pl-3 py-1 my-2 rounded-r"';
    } else if (
      tag === "code" &&
      /^class\s*=\s*["']language-[a-z0-9_-]+["']$/i.test(attrs) &&
      stack[stack.length - 1] === "pre"
    ) {
      safeAttrs = ' class="font-mono text-[13px]"';
    } else {
      if (attrs) errors.push(`标签 ${tag} 不支持额外属性`);
      if (tag === "code")
        safeAttrs =
          ' class="font-mono text-[13px] bg-slate-500/10 px-1 rounded"';
      if (tag === "pre")
        safeAttrs =
          ' class="font-mono text-xs bg-slate-500/10 p-2 rounded whitespace-pre-wrap"';
    }
    if (
      stack.some((t) => t === "code" || t === "pre") &&
      !(tag === "code" && stack[stack.length - 1] === "pre")
    )
      errors.push("代码块内不能嵌套其他格式");
    stack.push(tag);
    return `<${outputTag}${safeAttrs}>`;
  });
  if (stack.length) errors.push(`标签未闭合：${stack.join("、")}`);
  if (!template.trim()) errors.push("模板不能为空");
  if (/&(?!(?:amp|lt|gt|quot|#\d+|#x[\da-f]+);)/i.test(expanded))
    errors.push("普通文本中的 & 需要写成 &amp;");
  const doc = new DOMParser().parseFromString(html, "text/html");
  const length = [...(doc.body.textContent || "")].length;
  const warnings = [...new Set(unknown)].map(
    (key) => `未识别变量 {{${key}}}，发送时会原样保留`,
  );
  if (length > 4096)
    warnings.push(
      "示例正文超过 Telegram 单条消息 4096 字符限制，请精简模板；实际长度取决于事件内容。",
    );
  return { html, errors: [...new Set(errors)], warnings, length };
}
