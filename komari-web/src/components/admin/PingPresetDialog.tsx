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
  groupPingPresets,
  normalizePingPresetEdit,
  pingPresetTargetKey,
  togglePingPresetSelection,
  type PingPreset,
  type PingPresetCatalog,
  type PingPresetOverride,
} from "@/utils/pingPresets";

function PresetTarget({
  preset,
  checked,
  existing,
  customized,
  editing,
  onCheck,
  onEdit,
  onApply,
  onReset,
}: {
  preset: PingPreset;
  checked: boolean;
  existing: boolean;
  customized: boolean;
  editing: boolean;
  onCheck: (checked: boolean) => void;
  onEdit: (editing: boolean) => void;
  onApply: (edit: PingPresetOverride) => void;
  onReset: () => void;
}) {
  const [draft, setDraft] = useState<PingPresetOverride>(preset);
  const [error, setError] = useState("");
  return (
    <div className="min-w-0" data-testid={`preset-${preset.id}`}>
      <div className="flex items-start gap-2">
        <label
          className="flex flex-1 min-w-0 items-start gap-2 text-xs cursor-pointer"
          title={preset.note}
        >
          <Checkbox
            aria-label={preset.name}
            className="mt-0.5 shrink-0"
            checked={checked}
            onCheckedChange={(v) => onCheck(v === true)}
          />
          <span className="min-w-0">
            <span className="font-medium break-words">{preset.name}</span>{" "}
            <span className="text-muted-foreground">
              {preset.type.toUpperCase()}
            </span>
            {customized && (
              <span className="ml-1 text-amber-600">已自定义</span>
            )}
            {existing && <span className="ml-1 text-sky-600">已存在</span>}
            <code className="block break-all text-[10px] text-muted-foreground mt-0.5">
              {preset.target}
            </code>
          </span>
        </label>
        <Button
          type="button"
          variant="ghost"
          size="1"
          aria-label={`自定义编辑${preset.name}`}
          onClick={() => {
            setDraft(preset);
            setError("");
            onEdit(!editing);
          }}
        >
          编辑
        </Button>
      </div>
      {editing && (
        <div
          className="mt-2 rounded-md border bg-muted/30 p-2 flex flex-col gap-2"
          role="group"
          aria-label="自定义探测目标"
        >
          <label className="text-xs">
            探测名称
            <TextField.Root
              aria-label="探测名称"
              value={draft.name}
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            />
          </label>
          <Select.Root
            value={draft.type}
            onValueChange={(type) =>
              setDraft({ ...draft, type: type as PingPreset["type"] })
            }
          >
            <Select.Trigger aria-label="探测方式" />
            <Select.Content position="popper">
              <Select.Item value="icmp">ICMP</Select.Item>
              <Select.Item value="tcp">TCP</Select.Item>
              <Select.Item value="http">HTTP</Select.Item>
            </Select.Content>
          </Select.Root>
          <label className="text-xs">
            探测地址
            <TextField.Root
              aria-label="探测地址"
              value={draft.target}
              onChange={(e) => setDraft({ ...draft, target: e.target.value })}
              placeholder={
                draft.type === "tcp"
                  ? "域名:端口 或 [IPv6]:端口"
                  : draft.type === "http"
                    ? "https://example.com"
                    : "IP 或域名"
              }
            />
          </label>
          {error && (
            <p role="alert" className="text-xs text-red-600">
              {error}
            </p>
          )}
          <div className="flex flex-wrap justify-end gap-2">
            {customized && (
              <Button
                type="button"
                variant="ghost"
                size="1"
                onClick={() => {
                  onReset();
                  onEdit(false);
                }}
              >
                恢复默认
              </Button>
            )}
            <Button
              type="button"
              variant="soft"
              size="1"
              onClick={() => onEdit(false)}
            >
              取消编辑
            </Button>
            <Button
              type="button"
              size="1"
              onClick={() => {
                try {
                  onApply(normalizePingPresetEdit(draft));
                  onEdit(false);
                } catch (e) {
                  setError(e instanceof Error ? e.message : "无效的探测目标");
                }
              }}
            >
              应用修改并选中
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

function selectionState(
  targets: PingPreset[],
  selected: string[],
): boolean | "indeterminate" {
  const count = targets.filter((p) => selected.includes(p.id)).length;
  return count === targets.length && count > 0
    ? true
    : count > 0
      ? "indeterminate"
      : false;
}

export default function PingPresetDialog() {
  const { callViaHTTP } = useRPC2Call();
  const { refresh, pingTasks } = usePingTask();
  const [open, setOpen] = useState(false),
    [saving, setSaving] = useState(false),
    [loading, setLoading] = useState(false);
  const [catalog, setCatalog] = useState<PingPresetCatalog | null>(null),
    [error, setError] = useState("");
  const [group, setGroup] = useState("mainland");
  const [overrides, setOverrides] = useState<
    Record<string, PingPresetOverride>
  >({});
  const [editing, setEditing] = useState<string | null>(null);
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
  const regions = useMemo(
    () =>
      groupPingPresets(
        (catalog?.entries ?? []).map((p) => ({ ...p, ...overrides[p.id] })),
        group,
      ),
    [catalog, overrides, group],
  );
  const visible = regions.flatMap((r) => r.targets);
  const seconds = Number(interval),
    validInterval =
      Number.isInteger(seconds) && seconds >= 10 && seconds <= 86400;
  const canSave =
    selected.length > 0 &&
    (clients.length > 0 || defaultOn) &&
    validInterval &&
    !saving &&
    !editing;
  const estimatedDaily = validInterval
    ? Math.round((selected.length * clients.length * 86400) / seconds)
    : 0;
  const existing = new Set(
    (pingTasks ?? []).map((p) => pingPresetTargetKey(p.type, p.target)),
  );
  function changeOpen(value: boolean) {
    if (saving) return;
    if (!value) {
      setSelected([]);
      setOverrides({});
      setEditing(null);
    }
    setOpen(value);
  }
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
          overrides: PingPresetOverride[];
        },
        { created: number; updated: number; unchanged: number }
      >("admin:importPingPresets", {
        preset_ids: selected,
        clients,
        default_on: defaultOn,
        interval: seconds,
        overrides: selected.flatMap((id) =>
          overrides[id] ? [overrides[id]] : [],
        ),
      });
      toast.success(
        `已保存：新增 ${result.created}，合并 ${result.updated}，保持不变 ${result.unchanged}`,
      );
      refresh();
      setSelected([]);
      setOverrides({});
      setEditing(null);
      setOpen(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "预设保存失败");
    } finally {
      setSaving(false);
    }
  }
  return (
    <Dialog.Root open={open} onOpenChange={changeOpen}>
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
          在各地区卡片中勾选 IPv4、IPv6
          目标，可逐项编辑名称、地址和探测方式，再统一选择服务器和间隔保存。
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
                  setEditing(null);
                }}
              >
                {label}
              </Button>
            ))}
          </div>
          <div className="rounded-lg bg-muted/40 p-3 text-xs leading-relaxed text-muted-foreground">
            大陆内置目标采用 TCP 80 端口；IPv6 需服务器具备可用 IPv6。Google 与
            Cloudflare 默认按 ICMP 探测，其 Anycast
            地址不代表固定城市。海外地区使用服务商公开测试地址。目录不会自动启用。
          </div>
          <Flex justify="between" align="center" gap="2" wrap="wrap">
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                aria-label="全选当前地区目标"
                checked={selectionState(visible, selected)}
                onCheckedChange={(v) =>
                  setSelected((s) =>
                    togglePingPresetSelection(
                      s,
                      visible.map((p) => p.id),
                      v === true,
                    ),
                  )
                }
              />
              全选当前地区（{visible.length}）
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
            className="max-h-[48dvh] overflow-y-auto rounded-lg border p-3"
            aria-label="预设目标列表"
          >
            {loading ? (
              <p className="text-sm p-4">正在读取预设…</p>
            ) : error ? (
              <p role="alert" className="text-sm text-red-600">
                {error}
              </p>
            ) : !regions.length ? (
              <p className="text-sm text-muted-foreground p-4">暂无预设</p>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3 items-start">
                {regions.map(({ region, targets, families }) => (
                  <section
                    key={region}
                    className="min-w-0 rounded-lg border p-3"
                    aria-label={`${region}探测预设`}
                  >
                    <label className="flex items-center gap-2 font-semibold text-sm mb-3">
                      <Checkbox
                        aria-label={`选择${region}全部目标`}
                        checked={selectionState(targets, selected)}
                        onCheckedChange={(v) =>
                          setSelected((s) =>
                            togglePingPresetSelection(
                              s,
                              targets.map((p) => p.id),
                              v === true,
                            ),
                          )
                        }
                      />
                      {region}
                    </label>
                    <div className="flex flex-col gap-3">
                      {families.map(({ family, targets }) => (
                        <div
                          key={family}
                          className="min-w-0 flex flex-col gap-2"
                        >
                          <label className="flex items-center gap-2 text-xs font-semibold rounded bg-muted/40 p-1.5">
                            <Checkbox
                              aria-label={`选择${region}${family === "ipv4" ? "IPv4" : "IPv6"}全部目标`}
                              checked={selectionState(targets, selected)}
                              onCheckedChange={(v) =>
                                setSelected((s) =>
                                  togglePingPresetSelection(
                                    s,
                                    targets.map((p) => p.id),
                                    v === true,
                                  ),
                                )
                              }
                            />
                            {family === "ipv4" ? "IPv4" : "IPv6"}
                          </label>
                          {targets.map((p) => (
                            <PresetTarget
                              key={p.id}
                              preset={p}
                              checked={selected.includes(p.id)}
                              existing={existing.has(
                                pingPresetTargetKey(p.type, p.target),
                              )}
                              customized={!!overrides[p.id]}
                              editing={editing === p.id}
                              onCheck={(v) =>
                                setSelected((s) =>
                                  togglePingPresetSelection(s, [p.id], v),
                                )
                              }
                              onEdit={(v) => setEditing(v ? p.id : null)}
                              onApply={(edit) => {
                                setOverrides((s) => ({ ...s, [p.id]: edit }));
                                setSelected((s) =>
                                  togglePingPresetSelection(s, [p.id], true),
                                );
                              }}
                              onReset={() =>
                                setOverrides((s) => {
                                  const next = { ...s };
                                  delete next[p.id];
                                  return next;
                                })
                              }
                            />
                          ))}
                        </div>
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
            同类型、同目标的任务会合并服务器并保留原名称、间隔和历史数据，不会重复创建；已开启的默认启用设置保持不变。自定义编辑只应用到本次导入的任务，不会改写内置目录。目录日期：
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
