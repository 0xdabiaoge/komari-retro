import {
  Card,
  Flex,
  Text,
} from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import React, { useMemo, Suspense } from "react";
const NodeDisplay = React.lazy(() => import("../../components/NodeDisplay"));
import { formatBytes } from "@/utils/unitHelper";
import { useLiveData } from "@/contexts/LiveDataContext";
import { useNodeList } from "@/contexts/NodeListContext";
import Loading from "@/components/loading";
import type { LiveData } from "@/types/LiveData";

const formatSpeed = (bytes: number): string => {
  if (bytes === 0) return "0 B/s";
  const units = ["B/s", "KB/s", "MB/s", "GB/s", "TB/s"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const size = bytes / Math.pow(1024, i);

  let decimals = 2;
  if (i >= 3) decimals = 1;
  if (i <= 1) decimals = 0;
  if (size >= 100) decimals = 0;

  return `${size.toFixed(decimals)} ${units[i]}`;
};

const EMPTY_LIVE_DATA: LiveData = { online: [], data: {} };

const ShareIndex = () => {
  const [t] = useTranslation();
  const { live_data } = useLiveData();
  const { nodeList, isLoading, error } = useNodeList();
  const liveData = live_data?.data ?? EMPTY_LIVE_DATA;
  const onlineSet = useMemo(
    () => new Set(liveData.online),
    [liveData.online],
  );

  const summaryStats = useMemo(() => {
    const regions = new Set<string>();
    let totalUp = 0;
    let totalDown = 0;
    let speedUp = 0;
    let speedDown = 0;

    for (const node of nodeList ?? []) {
      if (!onlineSet.has(node.uuid)) continue;

      regions.add(node.region);
      const record = liveData.data[node.uuid];
      if (!record) continue;

      totalUp += record.network.totalUp || 0;
      totalDown += record.network.totalDown || 0;
      speedUp += record.network.up || 0;
      speedDown += record.network.down || 0;
    }

    return {
      regionCount: regions.size,
      trafficText: `↑ ${formatBytes(totalUp)} / ↓ ${formatBytes(totalDown)}`,
      speedText: `↑ ${formatSpeed(speedUp)} / ↓ ${formatSpeed(speedDown)}`,
    };
  }, [liveData.data, nodeList, onlineSet]);

  if (isLoading) {
    return <Loading />;
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center p-8 text-center gap-4 min-h-[40vh]">
        <Text color="red" size="3" weight="bold">
          {error}
        </Text>
      </div>
    );
  }

  return (
    <>
      <Card className="km-page-index km-summary-card summary-card mx-4 md:text-base text-sm relative">
        <div
          className="km-summary-card-grid grid gap-2"
          style={{
            gridTemplateColumns: `repeat(auto-fit, minmax(230px, 1fr))`,
            gridAutoRows: "min-content",
          }}
        >
          <TopCard
            title={t("current_time")}
            value={<CurrentTimeValue />}
          />
          <TopCard
            title={t("current_online")}
            value={`${onlineSet.size} / ${nodeList?.length ?? 0}`}
          />
          <TopCard
            title={t("region_overview")}
            value={`${summaryStats.regionCount} ${t("regions")}`}
          />
          <TopCard
            title={t("traffic_overview")}
            value={summaryStats.trafficText}
          />
          <TopCard
            title={t("network_speed")}
            value={summaryStats.speedText}
          />
        </div>
      </Card>
      <Suspense fallback={<div style={{ padding: 16 }}>Loading…</div>}>
        <NodeDisplay nodes={nodeList ?? []} liveData={liveData} />
      </Suspense>
    </>
  );
};

export default ShareIndex;

type TopCardProps = {
  title: string;
  value: React.ReactNode;
  description?: string;
};

const TopCard: React.FC<TopCardProps> = React.memo(
  ({ title, value, description }) => {
    return (
      <div className="km-top-card min-w-52 md:max-w-72 w-full">
        <Flex direction="column" gap="1">
          <label className="text-muted-foreground text-sm">{title}</label>
          <label className="font-medium -mt-2 text-md">{value}</label>
          {description && (
            <Text size="2" color="gray">
              {description}
            </Text>
          )}
        </Flex>
      </div>
    );
  },
);

const CurrentTimeValue = React.memo(() => {
  const [currentTime, setCurrentTime] = React.useState(() =>
    new Date().toLocaleTimeString(),
  );

  React.useEffect(() => {
    const timer = window.setInterval(() => {
      setCurrentTime(new Date().toLocaleTimeString());
    }, 1000);
    return () => window.clearInterval(timer);
  }, []);

  return <>{currentTime}</>;
});
