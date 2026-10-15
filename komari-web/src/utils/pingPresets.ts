export interface PingPreset {
  id: string;
  name: string;
  group: "mainland" | "global" | "overseas";
  region: string;
  carrier: string;
  family: "ipv4" | "ipv6";
  type: "tcp" | "icmp";
  target: string;
  source_url: string;
  note: string;
}
export interface PingPresetCatalog {
  updated_at: string;
  description: string;
  entries: PingPreset[];
}
export function filterPingPresets(
  entries: PingPreset[],
  group: string,
  family: string,
  carrier: string,
  search: string,
) {
  const query = search.trim().toLowerCase();
  return entries.filter(
    (p) =>
      p.group === group &&
      (family === "all" || p.family === family) &&
      (carrier === "all" || p.carrier === carrier) &&
      (!query ||
        `${p.name} ${p.region} ${p.carrier} ${p.target}`
          .toLowerCase()
          .includes(query)),
  );
}
export function togglePingPresetSelection(
  selected: string[],
  ids: string[],
  checked: boolean,
): string[] {
  const next = new Set(selected);
  for (const id of ids) {
    if (checked) next.add(id);
    else next.delete(id);
  }
  return [...next];
}
