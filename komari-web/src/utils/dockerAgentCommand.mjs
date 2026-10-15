export function createDockerAgentCommand(args, quoteArgs, autoDiscovery = false) {
  const installFlags = new Set([
    "--install-ghproxy", "--install-dir", "--install-service-name", "--install-version",
  ]);
  const runtimeArgs = [];
  let version = "latest";
  for (let index = 0; index < args.length; index++) {
    if (installFlags.has(args[index])) {
      if (args[index] === "--install-version") version = args[index + 1] || "latest";
      index++;
      continue;
    }
    runtimeArgs.push(args[index]);
  }
  const prepare = autoDiscovery
    ? "umask 077 && touch .komari-auto-discovery.json && chmod 600 .komari-auto-discovery.json && "
    : "";
  const mount = autoDiscovery
    ? '-v "$(pwd)/.komari-auto-discovery.json:/app/auto-discovery.json" '
    : "";
  return prepare + "docker run -d --name komari-agent --restart=always " + mount +
    quoteArgs([`ghcr.io/0xdabiaoge/komari-retro-agent:${version}`, ...runtimeArgs]);
}
