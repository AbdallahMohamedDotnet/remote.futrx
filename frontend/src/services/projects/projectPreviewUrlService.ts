// Every project port is published at `<slug>--<port>.dev.<public hostname>`.
// This service is the only place that shape is written down: it builds those
// URLs, finds them in agent output, and checks that a candidate is really one
// of ours before the browser drawer follows it.
class ProjectPreviewUrlService {
  build(slug: string, port: number | null, publicHostname: string): string {
    const hostSuffix = this.hostSuffix(publicHostname);
    if (!slug || !port || !hostSuffix) return "";
    return `https://${slug}--${port}${hostSuffix}`;
  }

  /** Preview URLs mentioned in a block of text, trailing punctuation trimmed. */
  findInText(text: string, publicHostname: string): string[] {
    const hostname = this.normalizeHostname(publicHostname);
    if (!hostname) return [];
    const pattern = new RegExp(
      `https:\\/\\/[a-z0-9][a-z0-9-]*--\\d{4,5}\\.dev\\.${this.escapeRegExp(hostname)}[^\\s<>)\\]]*`,
      "g",
    );
    return [...text.matchAll(pattern)]
      .map((match) => match[0].replace(/[.,;:!?]+$/, ""));
  }

  /** Whether `raw` is a preview URL for this project on this deployment —
   *  right scheme, right host suffix, right slug, and a port in range. */
  belongsToProject(raw: string, slug: string, publicHostname: string): boolean {
    try {
      const url = new URL(raw);
      const hostSuffix = this.hostSuffix(publicHostname);
      const portStart = `${slug}--`;
      return (
        hostSuffix !== "" &&
        url.protocol === "https:" &&
        url.hostname.startsWith(portStart) &&
        url.hostname.endsWith(hostSuffix) &&
        this.isValidPort(url.hostname.slice(portStart.length, -hostSuffix.length))
      );
    } catch {
      return false;
    }
  }

  port(url: string): number | null {
    const match = /--(\d{4,5})\./.exec(url);
    return match ? Number(match[1]) : null;
  }

  private isValidPort(port: string): boolean {
    const value = Number(port);
    return Number.isInteger(value) && value >= 1024 && value <= 65535;
  }

  private hostSuffix(publicHostname: string): string {
    const hostname = this.normalizeHostname(publicHostname);
    return hostname ? `.dev.${hostname}` : "";
  }

  private normalizeHostname(hostname: string): string {
    return hostname.trim().toLowerCase().replace(/\.$/, "");
  }

  private escapeRegExp(value: string): string {
    return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  }
}

export const projectPreviewUrlService = new ProjectPreviewUrlService();
