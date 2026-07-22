export function buildProjectPreviewUrl(
  slug: string,
  port: number | null,
  publicHostname: string,
): string {
  const hostSuffix = projectPreviewHostSuffix(publicHostname);
  if (!slug || !port || !hostSuffix) return "";
  return `https://${slug}--${port}${hostSuffix}`;
}

export function projectPreviewUrlsInText(
  text: string,
  publicHostname: string,
): string[] {
  const hostname = normalizeHostname(publicHostname);
  if (!hostname) return [];
  const pattern = new RegExp(
    `https:\\/\\/[a-z0-9][a-z0-9-]*--\\d{4,5}\\.dev\\.${escapeRegExp(hostname)}[^\\s<>)\\]]*`,
    "g",
  );
  return [...text.matchAll(pattern)]
    .map((match) => match[0].replace(/[.,;:!?]+$/, ""));
}

export function isProjectPreviewUrl(
  raw: string,
  slug: string,
  publicHostname: string,
): boolean {
  try {
    const url = new URL(raw);
    const hostSuffix = projectPreviewHostSuffix(publicHostname);
    const portStart = `${slug}--`;
    return (
      hostSuffix !== "" &&
      url.protocol === "https:" &&
      url.hostname.startsWith(portStart) &&
      url.hostname.endsWith(hostSuffix) &&
      isValidProjectPreviewPort(
        url.hostname.slice(portStart.length, -hostSuffix.length),
      )
    );
  } catch {
    return false;
  }
}

export function projectPreviewPort(url: string): number | null {
  const match = /--(\d{4,5})\./.exec(url);
  return match ? Number(match[1]) : null;
}

function isValidProjectPreviewPort(port: string): boolean {
  const value = Number(port);
  return Number.isInteger(value) && value >= 1024 && value <= 65535;
}

function projectPreviewHostSuffix(publicHostname: string): string {
  const hostname = normalizeHostname(publicHostname);
  return hostname ? `.dev.${hostname}` : "";
}

function normalizeHostname(hostname: string): string {
  return hostname.trim().toLowerCase().replace(/\.$/, "");
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
