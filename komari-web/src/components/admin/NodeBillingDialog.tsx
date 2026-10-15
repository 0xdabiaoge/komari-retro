import { useState } from "react";
import {
  Button,
  Dialog,
  Flex,
  IconButton,
  Select,
  Switch,
  TextField,
} from "@radix-ui/themes";
import { CircleDollarSign } from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { useNodeDetails, type NodeDetail } from "@/contexts/NodeDetailsContext";
import { SelectOrInput } from "@/components/ui/select-or-input";
import {
  BILLING_CURRENCIES,
  billingPrice,
  normalizeBillingCurrency,
  priceModeFromPrice,
  type PriceMode,
} from "../../../../shared/billing";

export default function BillingButton({ node }: { node: NodeDetail }) {
  const { t } = useTranslation();
  const { refresh } = useNodeDetails();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [mode, setMode] = useState<PriceMode>("hidden");
  const [amount, setAmount] = useState("");
  const [currency, setCurrency] = useState("USD");
  const [cycle, setCycle] = useState("30");
  const [expiry, setExpiry] = useState("");
  const [renewal, setRenewal] = useState(false);
  function changeOpen(next: boolean) {
    if (saving) return;
    if (next) {
      setMode(priceModeFromPrice(node.price));
      setAmount(node.price > 0 ? String(node.price) : "");
      setCurrency(normalizeBillingCurrency(node.currency || "$") || "USD");
      setCycle(String(node.billing_cycle || 30));
      const date = node.expired_at ? new Date(node.expired_at) : null;
      setExpiry(
        date && Number.isFinite(date.getTime()) && date.getUTCFullYear() > 1
          ? date.toISOString().slice(0, 10)
          : "",
      );
      setRenewal(!!node.auto_renewal);
    }
    setOpen(next);
  }
  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return;
    setSaving(true);
    try {
      const price = billingPrice(mode, amount);
      const billingCycle = Number(cycle);
      if (
        !Number.isInteger(billingCycle) ||
        (billingCycle <= 0 && billingCycle !== -1)
      )
        throw new Error("请选择有效计费周期");
      const response = await fetch(`/api/admin/client/${node.uuid}/edit`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          price,
          currency,
          billing_cycle: billingCycle,
          expired_at: expiry
            ? new Date(`${expiry}T00:00:00Z`).toISOString()
            : null,
          auto_renewal: renewal,
        }),
      });
      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        throw new Error(error.message || "服务器未能保存账单");
      }
      refresh();
      setOpen(false);
      toast.success("账单已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "账单保存失败");
    } finally {
      setSaving(false);
    }
  }
  return (
    <Dialog.Root open={open} onOpenChange={changeOpen}>
      <Dialog.Trigger>
        <IconButton
          variant="ghost"
          title={t("admin.nodeTable.billing", "账单")}
        >
          <CircleDollarSign size="18" />
        </IconButton>
      </Dialog.Trigger>
      <Dialog.Content maxWidth="560px" data-testid="node-billing-dialog">
        <Dialog.Title>{t("admin.nodeTable.billing", "账单")}</Dialog.Title>
        <Dialog.Description size="2" color="gray" className="mb-4">
          设置节点价格、货币和计费周期。
        </Dialog.Description>
        <form onSubmit={save}>
          <Flex direction="column" gap="3">
            <label htmlFor="billing-price-mode" className="font-bold">
              价格
            </label>
            <Select.Root
              value={mode}
              onValueChange={(v) => setMode(v as PriceMode)}
            >
              <Select.Trigger id="billing-price-mode" aria-label="价格模式" />
              <Select.Content>
                <Select.Item value="hidden">隐藏价格</Select.Item>
                <Select.Item value="free">免费</Select.Item>
                <Select.Item value="paid">自定义付费金额</Select.Item>
              </Select.Content>
            </Select.Root>
            {mode === "paid" && (
              <TextField.Root
                aria-label="付费金额"
                type="number"
                min="0"
                step="any"
                required
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="例如：19.99"
              />
            )}
            <label htmlFor="billing-currency" className="font-bold">
              货币
            </label>
            <Select.Root value={currency} onValueChange={setCurrency}>
              <Select.Trigger id="billing-currency" aria-label="货币" />
              <Select.Content position="popper">
                {!BILLING_CURRENCIES.some((c) => c.code === currency) && (
                  <Select.Item value={currency}>
                    现有货币：{currency}（保留）
                  </Select.Item>
                )}
                {BILLING_CURRENCIES.map((c) => (
                  <Select.Item key={c.code} value={c.code}>
                    {c.name} · {c.code}（{c.symbol.trim()}）
                  </Select.Item>
                ))}
              </Select.Content>
            </Select.Root>
            <span className="text-xs text-muted-foreground">
              按币种代码保存，区分人民币与日元、美元与其他元。
            </span>
            <label className="font-bold">计费周期</label>
            <SelectOrInput
              options={[
                { label: "每月", value: "30" },
                { label: "每季度", value: "92" },
                { label: "每半年", value: "184" },
                { label: "每年", value: "365" },
                { label: "每两年", value: "730" },
                { label: "每三年", value: "1095" },
                { label: "每五年", value: "1825" },
                { label: "一次性", value: "-1" },
              ]}
              type="number"
              name="billingCycle"
              value={cycle}
              onChange={setCycle}
            />
            <label htmlFor="billing-expiry" className="font-bold">
              到期时间
            </label>
            <TextField.Root
              id="billing-expiry"
              type="date"
              value={expiry}
              onChange={(e) => setExpiry(e.target.value)}
            >
              <TextField.Slot side="right">
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => {
                    const date = new Date();
                    date.setUTCFullYear(date.getUTCFullYear() + 200);
                    setExpiry(date.toISOString().slice(0, 10));
                  }}
                >
                  设置为长期
                </Button>
              </TextField.Slot>
            </TextField.Root>
            <label className="rounded-lg border p-3 flex items-center justify-between gap-3">
              <span>
                <span className="font-bold block">自动续期</span>
                <span className="text-xs text-muted-foreground">
                  到期时节点在线，自动顺延到期日；不会自动付款。
                </span>
              </span>
              <Switch
                checked={renewal}
                onCheckedChange={setRenewal}
                aria-label="自动续期"
              />
            </label>
            <Flex justify="end" gap="2">
              <Dialog.Close>
                <Button type="button" variant="soft" disabled={saving}>
                  取消
                </Button>
              </Dialog.Close>
              <Button type="submit" disabled={saving}>
                {saving ? "正在保存…" : "保存"}
              </Button>
            </Flex>
          </Flex>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
}
