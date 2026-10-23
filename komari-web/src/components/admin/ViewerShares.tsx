import { useCallback, useEffect, useState } from "react";
import { Button, Checkbox, Flex, Select, Text, TextField } from "@radix-ui/themes";
import { useRPC2Call } from "@/contexts/RPC2Context";
import { toast } from "sonner";
import { SettingCardCollapse } from "./SettingCard";

interface Grant {
  id: string;
  name: string;
  all_nodes: boolean;
  nodes: string[];
  history_hours: number;
  max_connections: number;
  expires_at: number;
  revoked: boolean;
}

export default function ViewerShares() {
  const { call } = useRPC2Call();
  const [nodeList, setNodeList] = useState<Array<{uuid:string;name:string;hidden:boolean}>>([]);
  const [grants, setGrants] = useState<Grant[]>([]);
  const [name, setName] = useState("");
  const [nodes, setNodes] = useState<string[]>([]);
  const [allNodes, setAllNodes] = useState(false);
  const [hours, setHours] = useState("24");
  const [expiry, setExpiry] = useState("24");
  const [price, setPrice] = useState(false);
  const [expiredAt, setExpiredAt] = useState(false);
  const [link, setLink] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const refresh = useCallback(async () => {
    try {
      setGrants(await call<undefined, Grant[]>("admin:listViewerShares"));
      setNodeList((await call<undefined, Array<{uuid:string;name:string;hidden:boolean}>>("admin:listClients")).filter(node => !node.hidden));
      setError("");
    } catch {
      setError("分享列表加载失败，请刷新重试。");
    }
  }, [call]);
  useEffect(() => { void refresh(); }, [refresh]);
  const create = async () => {
    setBusy(true);
    try {
      const result = await call<Record<string, unknown>, { path: string }>("admin:createViewerShare", {
        name: name.trim() || "只读分享", all_nodes: allNodes, nodes,
        history_hours: Number(hours), max_connections: 8,
        show_price: price, show_expiry: expiredAt,
        expires_at: expiry === "0" ? 0 : Math.floor(Date.now() / 1000) + Number(expiry) * 3600,
      });
      setLink(window.location.origin + result.path);
      await refresh();
      toast.success("分享已生成，请复制链接；密钥只显示这一次。");
    } catch {
      toast.error("生成失败，请检查节点范围并重试。");
    } finally { setBusy(false); }
  };
  return (
    <SettingCardCollapse title="独立授权分享" description="每条链接单独控制节点、历史和有效期，可随时撤销；默认不公开价格及到期时间。">
      <Flex direction="column" gap="3">
        {error && <Text color="red">{error}</Text>}
        <label>分享名称<TextField.Root maxLength={100} value={name} onChange={e => setName(e.target.value)} /></label>
        <Text as="label"><Checkbox checked={allNodes} onCheckedChange={v => setAllNodes(v === true)} /> 分享所有非隐藏节点（包括以后加入的节点）</Text>
        {!allNodes && <Flex direction="column" gap="2" style={{ maxHeight: 220, overflowY: "auto" }}>
          {nodeList?.map(node => <Text as="label" key={node.uuid}>
            <Checkbox checked={nodes.includes(node.uuid)} onCheckedChange={v => setNodes(old => v === true ? [...old, node.uuid] : old.filter(id => id !== node.uuid))} /> {node.name}
          </Text>)}
        </Flex>}
        <Flex gap="4" wrap="wrap">
          <label>历史范围 <Select.Root value={hours} onValueChange={setHours}><Select.Trigger /><Select.Content>
            <Select.Item value="0">仅实时状态</Select.Item><Select.Item value="1">最近 1 小时</Select.Item>
            <Select.Item value="24">最近 24 小时</Select.Item><Select.Item value="168">最近 7 天</Select.Item>
            <Select.Item value="744">最近 31 天</Select.Item>
          </Select.Content></Select.Root></label>
          <label>有效期 <Select.Root value={expiry} onValueChange={setExpiry}><Select.Trigger /><Select.Content>
            <Select.Item value="1">1 小时</Select.Item><Select.Item value="24">24 小时</Select.Item>
            <Select.Item value="168">7 天</Select.Item><Select.Item value="0">长期有效</Select.Item>
          </Select.Content></Select.Root></label>
        </Flex>
        <Text as="label"><Checkbox checked={price} onCheckedChange={v => setPrice(v === true)} /> 允许价格（仍须主题游客显示设置允许）</Text>
        <Text as="label"><Checkbox checked={expiredAt} onCheckedChange={v => setExpiredAt(v === true)} /> 允许到期时间（仍须主题游客显示设置允许）</Text>
        <Button disabled={busy || (!allNodes && nodes.length === 0)} onClick={() => void create()}>生成独立分享链接</Button>
        {link && <Flex direction="column" gap="2"><Text size="2">请保存下面的链接，关闭或刷新页面后不能再次读取密钥。</Text>
          <TextField.Root aria-label="新分享链接" value={link} readOnly />
          <Button onClick={() => void navigator.clipboard.writeText(link).then(() => toast.success("已复制")).catch(() => toast.error("请手动复制链接"))}>复制链接</Button>
        </Flex>}
        {grants.map(grant => <Flex key={grant.id} align="center" justify="between" gap="3" wrap="wrap">
          <Text size="2">{grant.name} · {grant.all_nodes ? "全部可见节点" : `${grant.nodes.length} 个节点`} · {grant.history_hours === 0 ? "仅实时" : `历史 ${grant.history_hours} 小时`} · {grant.revoked ? "已撤销" : grant.expires_at ? new Date(grant.expires_at * 1000).toLocaleString() : "长期有效"}</Text>
          <Button color="red" disabled={busy || grant.revoked} onClick={async () => {
            setBusy(true);
            try { await call("admin:revokeViewerShare", { id: grant.id }); setLink(""); await refresh(); toast.success("已撤销"); }
            catch { toast.error("撤销失败，请重试"); }
            finally { setBusy(false); }
          }}>撤销</Button>
        </Flex>)}
      </Flex>
    </SettingCardCollapse>
  );
}
