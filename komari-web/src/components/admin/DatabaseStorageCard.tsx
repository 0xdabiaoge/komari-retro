import { useCallback, useEffect, useState } from "react";
import { Button, Flex, Text } from "@radix-ui/themes";
import { useRPC2Call } from "@/contexts/RPC2Context";
import { formatBytes } from "@/utils/unitHelper";
import { SettingCard } from "./SettingCard";

type Storage = {
  allocated_bytes: number; reusable_bytes: number; wal_bytes: number;
  incremental_reclamation: boolean; load_retention_hours: number; ping_retention_hours: number;
};

export function DatabaseStorageCard() {
  const { callViaHTTP } = useRPC2Call();
  const [data, setData] = useState<Storage | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const refresh = useCallback(async () => {
    setLoading(true); setError(null);
    try { setData(await callViaHTTP<unknown, Storage>("admin:getDatabaseStorage")); }
    catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setLoading(false); }
  }, [callViaHTTP]);
  useEffect(() => { void refresh(); }, [refresh]);
  return <SettingCard title="数据库空间与历史保留" description="过期记录分批删除，空闲页优先复用；不会自动清空保留期内的历史。">
    <Flex direction="column" gap="2" className="w-full">
      {error && <Text color="red" role="alert">{error}</Text>}
      {data && <>
        <Text>数据库文件：{formatBytes(data.allocated_bytes)}；可复用空闲页：{formatBytes(data.reusable_bytes)}；写入日志：{formatBytes(data.wal_bytes)}</Text>
        <Text size="2">延迟记录保留：{data.ping_retention_hours > 0 ? `${data.ping_retention_hours} 小时` : "按服务端长期保留策略"}。负载记录超过 4 小时后聚合为 15 分钟记录，长期记录默认保留 30 天。</Text>
        <Text size="2">{data.incremental_reclamation ? "已开启增量回收：后台定期、小批量释放空闲页。" : "当前数据库未开启增量回收：空闲页会被后续写入复用，但文件不会自动缩小。"}</Text>
        {!data.incremental_reclamation && <Text size="2">如需缩小现有文件，请先备份并安排维护窗口，停止 Komari 后执行 compact-database。此操作不会在在线服务中自动运行。</Text>}
      </>}
      <Button size="1" variant="soft" disabled={loading} onClick={() => void refresh()}>{loading ? "读取中…" : "刷新空间信息"}</Button>
    </Flex>
  </SettingCard>;
}
