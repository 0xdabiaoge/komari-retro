"use client";

import React from "react";
import { useTheme } from "@/contexts/ThemeContext";

interface CircleChartProps {
  value: number; // 0-100
  label: string;
  subLabel?: string;
  color?: string; // Optional override
  compact?: boolean; // Compact mode for table views
  displayValue?: number; // Optional static text value when the arc is animated
  animationDuration?: number;
}

export default function CircleChart({
  value,
  label,
  subLabel,
  color,
  compact = false,
  displayValue,
  animationDuration = 800,
}: CircleChartProps) {
  const { themeConfig } = useTheme();

  // Clamp value
  const chartValue = Math.min(Math.max(value, 0), 100);
  const textValue = Math.min(Math.max(displayValue ?? value, 0), 100);

  // Get theme color based on selected color theme and value
  const getThemeColor = () => {
    if (color) return color; // Use override if provided

    const getColorForTheme = () => {
      switch (themeConfig.colorTheme) {
        case 'ocean':
          return chartValue >= 80 ? '#0284c7' : chartValue >= 60 ? '#06b6d4' : '#22d3ee';
        case 'sunset':
          return chartValue >= 80 ? '#ec4899' : chartValue >= 60 ? '#f97316' : '#fb923c';
        case 'forest':
          return chartValue >= 80 ? '#059669' : chartValue >= 60 ? '#10b981' : '#4ade80';
        case 'midnight':
          return chartValue >= 80 ? '#7c3aed' : chartValue >= 60 ? '#6366f1' : '#818cf8';
        case 'rose':
          return chartValue >= 80 ? '#e11d48' : chartValue >= 60 ? '#ec4899' : '#f472b6';
        default: // 'default'
          return chartValue >= 80 ? '#9333ea' : chartValue >= 60 ? '#3b82f6' : '#60a5fa';
      }
    };

    return getColorForTheme();
  };

  const fillColor = getThemeColor();

  const renderSVG = (radius: number, strokeWidth: number) => {
    const circumference = 2 * Math.PI * radius;
    const strokeDashoffset = circumference - (chartValue / 100) * circumference;
    
    // Fallback animation duration if needed
    const duration = animationDuration > 0 ? animationDuration : 0;
    const transitionStyle = duration > 0 
      ? { transition: `stroke-dashoffset ${duration}ms ease-out, stroke ${duration}ms ease-out` } 
      : {};

    return (
      <svg 
        viewBox="0 0 100 100" 
        className="w-full h-full transform -rotate-90 overflow-visible"
      >
        {/* Background Track */}
        <circle
          cx="50"
          cy="50"
          r={radius}
          fill="transparent"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          className="text-muted/20"
        />
        {/* Progress Arc */}
        <circle
          cx="50"
          cy="50"
          r={radius}
          fill="transparent"
          stroke={fillColor}
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeDashoffset={strokeDashoffset}
          strokeLinecap="round"
          style={transitionStyle}
        />
      </svg>
    );
  };

  // Compact mode for table views
  if (compact) {
    return (
      <div className="flex items-center justify-center">
        <div className="h-[40px] w-[40px] relative">
          {renderSVG(36, 16)}
          {/* Centered Percentage for compact mode */}
          <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
            <span className="text-[11px] font-bold text-foreground">
              {Math.round(textValue)}%
            </span>
          </div>
        </div>
      </div>
    );
  }

  // Default mode with labels
  return (
    <div className="flex flex-col items-center justify-center p-2">
      <div className="h-[90px] w-[90px] relative">
        {renderSVG(40, 9)}
        {/* Centered Percentage */}
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <span className="text-base font-bold text-foreground drop-shadow-sm tracking-tight">
            {Math.round(textValue)}%
          </span>
        </div>
      </div>

      {/* Labels */}
      <div className="text-center mt-2">
        <div className="text-xs font-semibold text-foreground/90">{label}</div>
        {subLabel && (
          <div className="text-[10px] text-muted-foreground/60 mt-0.5 whitespace-nowrap">{subLabel}</div>
        )}
      </div>
    </div>
  );
}
