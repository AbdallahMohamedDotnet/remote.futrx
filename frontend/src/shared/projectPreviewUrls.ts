const projectPreviewHostSuffix = ".dev.remote.futrx.com";

export function buildProjectPreviewUrl(slug: string, port: number | null): string {
  if (!slug || !port) return "";
  return `https://${slug}--${port}${projectPreviewHostSuffix}`;
}

export function projectPreviewUrlsInText(text: string): string[] {
  return [...text.matchAll(/https:\/\/[a-z0-9][a-z0-9-]*--\d{4,5}\.dev\.remote\.futrx\.com[^\s<>)\]]*/g)]
    .map((match) => match[0].replace(/[.,;:!?]+$/, ""));
}

export function isProjectPreviewUrl(raw: string, slug: string): boolean {
  try {
    const url = new URL(raw);
    const portStart = `${slug}--`;
    return (
      url.protocol === "https:" &&
      url.hostname.startsWith(portStart) &&
      url.hostname.endsWith(projectPreviewHostSuffix) &&
      isValidProjectPreviewPort(
        url.hostname.slice(portStart.length, -projectPreviewHostSuffix.length),
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
