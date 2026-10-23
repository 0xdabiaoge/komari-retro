/** Fill only; callers must never append Enter to a command derived from a path. */
export function directoryCommand(path: string): string {
  if (!path || [...path].some(c => c.charCodeAt(0) < 32 || c.charCodeAt(0) === 127)) throw new Error("目录包含不支持的控制字符");
  if (/^(?:[a-z]:[\\/]|\\\\)/i.test(path)) return `Set-Location -LiteralPath '${path.replace(/'/g, "''")}'`;
  return `cd -- '${path.replace(/'/g, () => "'\\''")}'`;
}
