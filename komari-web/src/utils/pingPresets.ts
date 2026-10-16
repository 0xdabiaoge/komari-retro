export interface PingPreset {
  id: string;
  name: string;
  group: "mainland" | "global" | "overseas";
  region: string;
  carrier: string;
  family: "ipv4" | "ipv6";
  type: "tcp" | "icmp" | "http";
  target: string;
  source_url: string;
  note: string;
}
export interface PingPresetCatalog {
  updated_at: string;
  description: string;
  entries: PingPreset[];
}
export type PingPresetOverride = Pick<
  PingPreset,
  "id" | "name" | "type" | "target"
>;

export function pingPresetTargetKey(type = "", target = ""): string {
  const kind = type.trim().toLowerCase(),
    value = target.trim();
  if (kind === "http") {
    return (
      kind +
      "|" +
      value.replace(
        /^([a-z]+:\/\/)([^/?#]+)/i,
        (_, scheme: string, host: string) =>
          scheme.toLowerCase() + host.toLowerCase(),
      )
    );
  }
  return kind + "|" + value.toLowerCase();
}

export function groupPingPresets(entries: PingPreset[], group: string) {
  const regions = new Map<string, PingPreset[]>();
  for (const entry of entries) {
    if (entry.group !== group) continue;
    if (!regions.has(entry.region)) regions.set(entry.region, []);
    regions.get(entry.region)!.push(entry);
  }
  return [...regions].map(([region, targets]) => ({
    region,
    targets,
    families: (["ipv4", "ipv6"] as const)
      .map((family) => ({
        family,
        targets: targets.filter((p) => p.family === family),
      }))
      .filter((section) => section.targets.length > 0),
  }));
}

export function normalizePingPresetEdit(
  edit: PingPresetOverride,
): PingPresetOverride {
  const name = edit.name.trim(),
    target = edit.target.trim();
  const control = (s: string) =>
    [...s].some((c) => {
      const code = c.codePointAt(0)!;
      return code < 32 || (code >= 127 && code <= 159);
    });
  if (!name || [...name].length > 128 || control(name))
    throw new Error("探测名称须为 1 至 128 个字符，且不能包含控制字符");
  if (
    !target ||
    new TextEncoder().encode(target).length > 2048 ||
    /\s/.test(target) ||
    control(target)
  )
    throw new Error("探测地址不能为空、不能包含空白，且不能超过 2048 字节");
  try {
    if (edit.type === "icmp") {
      if (["/", "\\", "?", "#", "@", "[", "]"].some((c) => target.includes(c)))
        throw new Error();
      if (target.includes(":")) new URL(`http://[${target}]/`);
    } else if (edit.type === "tcp") {
      const parts = target.match(/^(\[[^\]]+\]|[^:]+):(\d+)$/);
      if (!parts || Number(parts[2]) < 1 || Number(parts[2]) > 65535)
        throw new Error();
      const url = new URL(`http://${target}`);
      if (
        url.username ||
        url.password ||
        url.pathname !== "/" ||
        url.search ||
        url.hash
      )
        throw new Error();
    } else if (edit.type === "http") {
      const url = new URL(target);
      if (
        !["http:", "https:"].includes(url.protocol) ||
        !url.hostname ||
        url.username ||
        url.password ||
        url.port === "0"
      )
        throw new Error();
    } else throw new Error();
  } catch {
    throw new Error(
      edit.type === "icmp"
        ? "ICMP 地址请填写 IP 或域名，不要包含协议或端口"
        : edit.type === "tcp"
          ? "TCP 地址请使用 域名:端口、IPv4:端口 或 [IPv6]:端口"
          : "HTTP 地址请填写完整的 http:// 或 https:// 网址，不要包含登录凭据",
    );
  }
  return { id: edit.id, name, type: edit.type, target };
}
export function togglePingPresetSelection(
  selected: string[],
  ids: string[],
  checked: boolean,
): string[] {
  const next = new Set(selected);
  for (const id of ids) {
    if (checked) next.add(id);
    else next.delete(id);
  }
  return [...next];
}
