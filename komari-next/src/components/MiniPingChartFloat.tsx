import React, { useState, useRef, useCallback } from "react";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import MiniPingChart from "./MiniPingChart";

interface FloatMiniPingChartProps {
  uuid: string;
  trigger: React.ReactNode;
  chartWidth?: string | number;
  chartHeight?: string | number;
  hours?: number;
}

const MiniPingChartFloat: React.FC<FloatMiniPingChartProps> = ({
  uuid,
  trigger,
  chartWidth = 600,
  chartHeight = 300,
  hours = 12,
}) => {
  const [open, setOpen] = useState(false);
  const hoverTimeoutRef = useRef<number | null>(null);

  const handleMouseEnter = useCallback(() => {
    if (hoverTimeoutRef.current) {
      clearTimeout(hoverTimeoutRef.current);
    }
    hoverTimeoutRef.current = window.setTimeout(() => {
      setOpen(true);
    }, 3000);
  }, []);

  const handleMouseLeave = useCallback(() => {
    if (hoverTimeoutRef.current) {
      clearTimeout(hoverTimeoutRef.current);
    }
    hoverTimeoutRef.current = window.setTimeout(() => {
      setOpen(false);
    }, 200);
  }, []);

  const handleClick = useCallback(() => {
    if (hoverTimeoutRef.current) {
      clearTimeout(hoverTimeoutRef.current);
    }
    setOpen((prev) => !prev);
  }, []);

  const responsiveWidth = typeof chartWidth === 'number' ? `${chartWidth}px` : chartWidth;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <span
          onMouseEnter={handleMouseEnter}
          onMouseLeave={handleMouseLeave}
          onClick={handleClick}
          style={{ cursor: "pointer" }}
          className="flex items-center justify-center"
        >
          {trigger}
        </span>
      </PopoverTrigger>
      <PopoverContent
        sideOffset={5}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        className="z-50 mx-auto w-auto max-w-[calc(100vw-1rem)] overflow-y-auto overscroll-contain rounded-lg border bg-card p-2 shadow-lg touch-pan-y"
        style={{
          width: responsiveWidth,
          maxHeight: "min(var(--radix-popover-content-available-height, calc(100dvh - 1rem)), calc(100dvh - 1rem))",
        }}
        align="center"
      >
        <MiniPingChart hours={hours} uuid={uuid} width="100%" height={chartHeight} />
      </PopoverContent>
    </Popover>
  );
};

export default MiniPingChartFloat;
