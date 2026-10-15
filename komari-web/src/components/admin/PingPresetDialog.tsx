import { useEffect, useMemo, useState } from "react";
import {
  Button,
  Checkbox,
  Dialog,
  Flex,
  Select,
  TextField,
} from "@radix-ui/themes";
import { toast } from "sonner";
import NodeSelectorDialog from "@/components/NodeSelectorDialog";
import { usePingTask } from "@/contexts/PingTaskContext";
import { useRPC2Call } from "@/contexts/RPC2Context";
import {
  filterPingPresets,
  togglePingPresetSelection,
  type PingPresetCatalog,
} from "@/utils/pingPresets";

export default function PingPresetDialog() {
  const { callViaHTTP } = useRPC2Call();
  const { refresh, pingTasks } = usePingTask();
  const [open, setOpen] = useState(false),
    [saving, setSaving] = useState(false),
    [loading, setLoading] = useState(false);
  const [catalog, setCatalog] = useState<PingPresetCatalog | null>(null),
    [error, setError] = useState("");
  const [group, setGroup] = useState("mainland"),
    [family, setFamily] = useState("ipv4"),
    [carrier, setCarrier] = useState("all"),
    [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string[]>([]),
    [clients, setClients] = useState<string[]>([]),
    [defaultOn, setDefaultOn] = useState(false),
    [interval, setInterval] = useState("60");
  useEffect(() => {
    if (!open || catalog) return;
    let cancelled = false;
    setLoading(true);
    setError("");
    callViaHTTP<Record<string, never>, PingPresetCatalog>(
      "admin:getPingPresets",
      {},
    )
      .then((data) => {
        if (!cancelled) setCatalog(data);
      })
      .catch((e) => {
        if (!cancelled)
          setError(e instanceof Error ? e.message : "读取预设失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, catalog, callViaHTTP]);
  const visible = useMemo(
    () =>
      filterPingPresets(
        catalog?.entries ?? [],
        group,
        family,
        group === "mainland" ? carrier : "all",
        search,
      ),
    [catalog, group, family, carrier, search],
  );
  const regions = [...new Set(visible.map((p) => p.region))];
  const allChecked =
    visible.length > 0 && visible.every((p) => selected.includes(p.id));
  const someChecked = visible.some((p) => selected.includes(p.id));
  const seconds = Number(interval),
    validInterval =
      Number.isInteger(seconds) && seconds >= 10 && seconds <= 86400;
  const canSave =
    selected.length > 0 &&
    (clients.length > 0 || defaultOn) &&
    validInterval &&
    !saving;
  const estimatedDaily = validInterval
    ? Math.round((selected.length * clients.length * 86400) / seconds)
    : 0;
  const existing = new Set(
    (pingTasks ?? []).map((p) => `${p.type}|${p.target}`.toLowerCase()),
  );
  async function save() {
    if (!canSave) return;
    setSaving(true);
    try {
      const result = await callViaHTTP<
        {
          preset_ids: string[];
          clients: string[];
          default_on: boolean;
          interval: number;
        },
        { created: number; updated: number; unchanged: number }
      >("admin:importPingPresets", {
        preset_ids: selected,
        clients,
        default_on: defaultOn,
        interval: seconds,
      });
      toast.success(
        `已保存：新增 ${result.created}，合并 ${result.updated}，保持不变 ${result.unchanged}`,
      );
      refresh();
      setSelected([]);
      setOpen(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "预设保存失败");
    } finally {
      setSaving(false);
    }
  }
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(v) => {
        if (!saving) setOpen(v);
      }}
    >
      <Dialog.Trigger>
        <Button variant="soft">批量添加预设</Button>
      </Dialog.Trigger>
      <Dialog.Content
        maxWidth="1000px"
        data-testid="ping-preset-dialog"
        className="max-h-[92dvh] overflow-y-auto"
      >
        <Dialog.Title>选择延迟探测预设</Dialog.Title>
        <Dialog.Description size="2" color="gray">
          勾选目标后统一选择服务器和间隔，再保存。目录不会自动启用。
        </Dialog.Description>
        <Flex direction="column" gap="3" className="mt-4">
          <div
            role="group"
            aria-label="预设地区"
            className="flex flex-wrap gap-2"
          >
            {[
              ["mainland", "中国大陆"],
              ["global", "Google / Cloudflare"],
              ["overseas", "海外地区"],
            ].map(([id, label]) => (
              <Button
                key={id}
                type="button"
                variant={group === id ? "solid" : "soft"}
                onClick={() => {
                  setGroup(id);
                  setFamily("ipv4");
                  setSearch("");
                }}
              >
                {label}
              </Button>
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <TextField.Root
              aria-label="搜索探测预设"
              placeholder="搜索省份、地区、运营商或地址"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="flex-1 min-w-40"
            />
            <Select.Root value={family} onValueChange={setFamily}>
              <Select.Trigger aria-label="IP 协议" />
              <Select.Content>
                <Select.Item value="ipv4">IPv4</Select.Item>
                <Select.Item value="ipv6">IPv6</Select.Item>
                <Select.Item value="all">IPv4 + IPv6</Select.Item>
              </Select.Content>
            </Select.Root>
            {group === "mainland" && (
              <Select.Root value={carrier} onValueChange={setCarrier}>
                <Select.Trigger aria-label="运营商" />
                <Select.Content>
                  <Select.Item value="all">全部运营商</Select.Item>
                  {["电信", "联通", "移动"].map((v) => (
                    <Select.Item key={v} value={v}>
                      {v}
                    </Select.Item>
                  ))}
                </Select.Content>
              </Select.Root>
            )}
          </div>
          <div className="rounded-lg bg-muted/40 p-3 text-xs leading-relaxed text-muted-foreground">
            大陆预设采用 TCP 80 端口；IPv6 需服务器具备可用 IPv6。Google 与
            Cloudflare 按 ICMP 探测，其 Anycast
            地址不代表固定城市。海外地区使用服务商公开测试地址。
          </div>
          <Flex justify="between" align="center" gap="2" wrap="wrap">
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                aria-label="全选当前筛选结果"
                checked={
                  allChecked ? true : someChecked ? "indeterminate" : false
                }
                onCheckedChange={(checked) =>
                  setSelected((s) =>
                    togglePingPresetSelection(
                      s,
                      visible.map((p) => p.id),
                      checked === true,
                    ),
                  )
                }
              />
              全选当前结果（{visible.length}）
            </label>
            <span className="text-sm">
              已选 {selected.length} 个目标{" "}
              <Button
                size="1"
                variant="ghost"
                onClick={() => setSelected([])}
                disabled={!selected.length}
              >
                清空选择
              </Button>
            </span>
          </Flex>
          <div
            className="max-h-[42dvh] overflow-y-auto rounded-lg border p-3"
            aria-label="预设目标列表"
          >
            {loading ? (
              <p className="text-sm p-4">正在读取预设…</p>
            ) : error ? (
              <p role="alert" className="text-sm text-red-600">
                {error}
              </p>
            ) : !visible.length ? (
              <p className="text-sm text-muted-foreground p-4">
                没有匹配的预设
              </p>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
                {regions.map((region) => (
                  <section
                    key={region}
                    className="min-w-0 rounded-lg border p-3"
                  >
                    <label className="flex items-center gap-2 font-semibold text-sm mb-2">
                      <Checkbox
                        aria-label={`选择${region}全部目标`}
                        checked={visible
                          .filter((p) => p.region === region)
                          .every((p) => selected.includes(p.id))}
                        onCheckedChange={(checked) =>
                          setSelected((s) =>
                            togglePingPresetSelection(
                              s,
                              visible
                                .filter((p) => p.region === region)
                                .map((p) => p.id),
                              checked === true,
                            ),
                          )
                        }
                      />
                      {region}
                    </label>
                    <div className="flex flex-col gap-2">
                      {visible
                        .filter((p) => p.region === region)
                        .map((p) => (
                          <label
                            key={p.id}
                            className="flex items-start gap-2 text-xs cursor-pointer"
                            title={p.note}
                          >
                            <Checkbox
                              aria-label={p.name}
                              className="mt-0.5 shrink-0"
                              checked={selected.includes(p.id)}
                              onCheckedChange={(checked) =>
                                setSelected((s) =>
                                  togglePingPresetSelection(
                                    s,
                                    [p.id],
                                    checked === true,
                                  ),
                                )
                              }
                            />
                            <span className="min-w-0">
                              <span className="font-medium">{p.name}</span>{" "}
                              <span className="text-muted-foreground">
                                {p.type.toUpperCase()}
                              </span>
                              {existing.has(
                                `${p.type}|${p.target}`.toLowerCase(),
                              ) && (
                                <span className="text-sky-600 ml-1">
                                  已存在
                                </span>
                              )}
                              <code className="block break-all text-[10px] text-muted-foreground mt-0.5">
                                {p.target}
                              </code>
                            </span>
                          </label>
                        ))}
                    </div>
                  </section>
                ))}
              </div>
            )}
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 rounded-lg border p-3">
            <div>
              <div className="font-semibold text-sm mb-2">应用到服务器</div>
              <Flex align="center" gap="2">
                <NodeSelectorDialog
                  value={clients}
                  onChange={setClients}
                  title="选择服务器"
                />
                <span className="text-sm">已选 {clients.length} 台</span>
              </Flex>
              <label className="mt-3 flex gap-2 items-center text-xs">
                <Checkbox
                  checked={defaultOn}
                  onCheckedChange={(v) => setDefaultOn(v === true)}
                />
                新加入的服务器默认启用这些目标
              </label>
            </div>
            <div>
              <label
                htmlFor="preset-interval"
                className="font-semibold text-sm block mb-2"
              >
                探测间隔（秒）
              </label>
              <TextField.Root
                id="preset-interval"
                type="number"
                min="10"
                max="86400"
                value={interval}
                onChange={(e) => setInterval(e.target.value)}
              />
              <p className="text-xs text-muted-foreground mt-2">
                本次选择预计每天产生约 {estimatedDaily.toLocaleString()}{" "}
                条探测记录；相同任务合并时实际增量更少。
              </p>
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            同类型、同目标的任务会合并服务器并保留原名称、间隔和历史数据，不会重复创建；已开启的默认启用设置保持不变。目录日期：
            {catalog?.updated_at ?? "—"}。
          </p>
          {!validInterval && (
            <p role="alert" className="text-xs text-red-600">
              间隔必须是 10 至 86400 秒的整数。
            </p>
          )}
          <Flex justify="end" gap="2">
            <Dialog.Close>
              <Button type="button" variant="soft" disabled={saving}>
                取消
              </Button>
            </Dialog.Close>
            <Button type="button" disabled={!canSave} onClick={save}>
              {saving ? "正在保存…" : `保存所选预设（${selected.length}）`}
            </Button>
          </Flex>
        </Flex>
      </Dialog.Content>
    </Dialog.Root>
  );
}
