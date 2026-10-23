import type { ITheme, ITerminalOptions } from "@xterm/xterm";

export type TerminalThemeName = "dark" | "light" | "contrast";
export interface SavedCommand {
  id: string;
  name: string;
  command: string;
  description: string;
}
export interface TerminalPreferences {
  terminalOptions: Partial<ITerminalOptions>;
  terminalPadding: number;
  themeName: TerminalThemeName;
  savedCommands: SavedCommand[];
}
// Original Komari palettes. No remote fonts, runtime scripts or terminal recordings.
export const terminalThemes: Record<TerminalThemeName, ITheme> = {
  dark: {
    background: "#101318",
    foreground: "#e2e8f0",
    cursor: "#67e8f9",
    selectionBackground: "#334155",
    selectionForeground: "#ffffff",
    black: "#111827",
    red: "#fb7185",
    green: "#86efac",
    yellow: "#fde68a",
    blue: "#93c5fd",
    magenta: "#d8b4fe",
    cyan: "#67e8f9",
    white: "#e2e8f0",
    brightBlack: "#64748b",
  },
  light: {
    background: "#f8fafc",
    foreground: "#172033",
    cursor: "#0369a1",
    selectionBackground: "#bae6fd",
    selectionForeground: "#0f172a",
    black: "#172033",
    red: "#be123c",
    green: "#15803d",
    yellow: "#a16207",
    blue: "#1d4ed8",
    magenta: "#7e22ce",
    cyan: "#0e7490",
    white: "#cbd5e1",
    brightBlack: "#64748b",
  },
  contrast: {
    background: "#000000",
    foreground: "#ffffff",
    cursor: "#ffff00",
    selectionBackground: "#ffff00",
    selectionForeground: "#000000",
    red: "#ff8080",
    green: "#80ff80",
    yellow: "#ffff80",
    blue: "#80c0ff",
    magenta: "#ff80ff",
    cyan: "#80ffff",
    brightBlack: "#aaaaaa",
  },
};
export const defaultPreferences: TerminalPreferences = {
  terminalOptions: {
    fontSize: 14,
    fontFamily: "'Cascadia Mono', Consolas, 'Noto Sans Mono CJK SC', monospace",
    scrollback: 5000,
    cursorBlink: true,
    convertEol: false,
    macOptionIsMeta: true,
  },
  terminalPadding: 12,
  themeName: "dark",
  savedCommands: [],
};

export function normalizePreferences(
  value: Partial<TerminalPreferences>,
): TerminalPreferences {
  const o = { ...defaultPreferences.terminalOptions, ...value.terminalOptions };
  return {
    ...value,
    terminalOptions: {
      ...o,
      fontSize: Math.max(10, Math.min(32, Number(o.fontSize) || 14)),
      scrollback: Math.max(0, Math.min(10000, Number(o.scrollback) || 0)),
    },
    terminalPadding: Math.max(
      0,
      Math.min(32, Number(value.terminalPadding ?? 12)),
    ),
    themeName:
      value.themeName === "light" || value.themeName === "contrast"
        ? value.themeName
        : "dark",
    savedCommands: Array.isArray(value.savedCommands)
      ? value.savedCommands
      : [],
  };
}
