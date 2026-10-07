export type UninstallPlatform = "linux" | "windows" | "macos" | "docker";

export interface UninstallCommandOptions {
  /** Agent service name or Docker container name. Defaults to komari-agent. */
  serviceName?: string;
  /** Exact installation directory; blank uses the platform's known default directories. */
  installDir?: string;
}

export function createUninstallCommand(
  platform: UninstallPlatform,
  options?: UninstallCommandOptions,
): string;
