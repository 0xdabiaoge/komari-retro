import { Box } from '@/components/ui/box';
import { Flex } from '@/components/ui/flex';
import { Text } from '@/components/ui/text';
import { useTheme } from '@/contexts/ThemeContext';

interface UsageBarProps {
  value: number; // Utilization percentage (0–100)
  label: string; // Label for the bar (e.g., "CPU", "Memory", "Disk")
  compact?: boolean; // Whether to show in compact mode (for tables)
  max?: number; // Maximum value for the bar (e.g., total RAM, total disk space)
}

const UsageBar = ({ value, label, compact = false, max = 100 }: UsageBarProps) => {
  const { themeConfig } = useTheme();

  // Ensure value is between 0 and 100
  const clampedValue = Math.min(Math.max(value, 0), max);

  // Determine color based on thresholds
  const getColor = (val: number) => {
    if (val >= 80) return 'red';
    if (val >= 60) return 'orange';
    return 'green';
  };

  const barColor = getColor(clampedValue);

  // Render based on graph design preference
  if (themeConfig.graphDesign === 'minimal') {
    return (
      <Flex justify="between" align="center" style={{ width: '100%' }}>
        <Text size="2" color="gray">
          {label}
        </Text>
        <Text size="2" weight="medium">
          {clampedValue.toFixed(1)}%
        </Text>
      </Flex>
    );
  }

  if (themeConfig.graphDesign === 'circle') {
    const radius = compact ? 16 : 20;
    const circumference = 2 * Math.PI * radius;
    const offset = circumference - (clampedValue / 100) * circumference;

    return (
      <Flex direction="column" gap="1" align="center" style={{ width: '100%' }}>
        <svg width={compact ? 40 : 50} height={compact ? 40 : 50}>
          <circle
            cx={compact ? 20 : 25}
            cy={compact ? 20 : 25}
            r={radius}
            fill="none"
            stroke="var(--gray-5)"
            strokeWidth="3"
          />
          <circle
            cx={compact ? 20 : 25}
            cy={compact ? 20 : 25}
            r={radius}
            fill="none"
            stroke={`var(--${barColor}-9)`}
            strokeWidth="3"
            strokeDasharray={circumference}
            strokeDashoffset={offset}
            strokeLinecap="round"
            transform={`rotate(-90 ${compact ? 20 : 25} ${compact ? 20 : 25})`}
            style={{ transition: 'stroke-dashoffset 0.5s ease-out' }}
          />
          <text
            x={compact ? 20 : 25}
            y={compact ? 20 : 25}
            textAnchor="middle"
            dominantBaseline="middle"
            fill="currentColor"
            fontSize={compact ? "10" : "11"}
            fontWeight="500"
          >
            {clampedValue.toFixed(0)}%
          </text>
        </svg>
        {!compact && (
          <Text size="2" color="gray">
            {label}
          </Text>
        )}
      </Flex>
    );
  }

  if (themeConfig.graphDesign === 'bar') {
    return (
      <Flex direction="column" gap="1" style={{ width: '100%' }}>
        <Flex justify="between" align="center">
          <Text size="2" color="gray">
            {label}
          </Text>
          <Text size="2" weight="medium">
            {clampedValue.toFixed(1)}%
          </Text>
        </Flex>
        <div style={{ display: 'flex', gap: '2px', height: compact ? '20px' : '30px', alignItems: 'flex-end' }}>
          {Array.from({ length: 10 }).map((_, i) => {
            const threshold = (i + 1) * 10;
            const isActive = clampedValue >= threshold;
            return (
              <div
                key={i}
                style={{
                  flex: 1,
                  height: `${(i + 1) * 10}%`,
                  backgroundColor: isActive ? `var(--${barColor}-9)` : 'var(--gray-5)',
                  borderRadius: '2px',
                  transition: 'background-color 0.3s ease-out',
                }}
              />
            );
          })}
        </div>
      </Flex>
    );
  }

  // Default: progress bar
  if (compact) {
    return (
      <Box style={{ width: '100%' }}>
        <Box
          style={{
            width: '100%',
            height: '6px',
            backgroundColor: 'var(--gray-5)',
            borderRadius: '3px',
            overflow: 'hidden',
            marginBottom: '2px',
          }}
        >
          <div
            style={{
              height: '100%',
              backgroundColor: `var(--${barColor}-9)`,
              borderRadius: '3px',
              width: `${clampedValue}%`,
              transition: 'width 0.5s ease-out',
            }}
          />
        </Box>
        <label color="gray" className='text-sm'>
          {clampedValue.toFixed(1)}%
        </label>
      </Box>
    );
  }

  return (
    <Flex direction="column" gap="1" style={{ width: '100%' }}>
      <Flex justify="between" align="center">
        <Text size="2" color="gray">
          {label}
        </Text>
        <Text size="2" weight="medium">
          {clampedValue.toFixed(1)}%
        </Text>
      </Flex>
      <Box
        style={{
          width: '100%',
          height: '8px',
          backgroundColor: 'var(--gray-5)',
          borderRadius: '4px',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            height: '100%',
            backgroundColor: `var(--${barColor}-9)`,
            borderRadius: '4px',
            width: `${clampedValue}%`,
            transition: 'width 0.5s ease-out',
          }}
        />
      </Box>
    </Flex>
  );
};

export default UsageBar;
