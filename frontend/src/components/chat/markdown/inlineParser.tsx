import type { ComponentChildren } from "preact";

const urlPattern = /^https?:\/\/[^\s<]+/;

export function renderInline(text: string, keyPrefix: string): ComponentChildren[] {
  const nodes: ComponentChildren[] = [];
  let plain = "";
  let index = 0;

  const flush = () => {
    if (plain) {
      nodes.push(plain);
      plain = "";
    }
  };

  const addWrapped = (tag: "strong" | "em" | "del", content: string, markerLength: number, end: number) => {
    flush();
    const key = `${keyPrefix}-${nodes.length}`;
    const children = renderInline(content, key);
    if (tag === "strong") nodes.push(<strong key={key}>{children}</strong>);
    if (tag === "em") nodes.push(<em key={key}>{children}</em>);
    if (tag === "del") nodes.push(<del key={key}>{children}</del>);
    index = end + markerLength;
  };

  while (index < text.length) {
    if (text[index] === "`") {
      const end = text.indexOf("`", index + 1);
      if (end > index + 1) {
        flush();
        nodes.push(
          <code key={`${keyPrefix}-${nodes.length}`} class="bg-white/[0.08] text-ink-100 px-1 py-0.5 rounded text-[12.5px] font-mono">
            {text.slice(index + 1, end)}
          </code>
        );
        index = end + 1;
        continue;
      }
    }

    if (text.startsWith("**", index)) {
      const end = text.indexOf("**", index + 2);
      if (end > index + 2) {
        addWrapped("strong", text.slice(index + 2, end), 2, end);
        continue;
      }
    }

    if (text.startsWith("~~", index)) {
      const end = text.indexOf("~~", index + 2);
      if (end > index + 2) {
        addWrapped("del", text.slice(index + 2, end), 2, end);
        continue;
      }
    }

    if (text[index] === "*" && text[index + 1] !== "*") {
      const end = text.indexOf("*", index + 1);
      if (end > index + 1) {
        addWrapped("em", text.slice(index + 1, end), 1, end);
        continue;
      }
    }

    if (text[index] === "[") {
      const labelEnd = text.indexOf("]", index + 1);
      const hrefStart = labelEnd >= 0 ? labelEnd + 1 : -1;
      if (hrefStart >= 0 && text[hrefStart] === "(") {
        const hrefEnd = text.indexOf(")", hrefStart + 1);
        if (hrefEnd > hrefStart + 1) {
          const href = safeHref(text.slice(hrefStart + 1, hrefEnd));
          if (href) {
            flush();
            const key = `${keyPrefix}-${nodes.length}`;
            nodes.push(
              <a key={key} href={href} target="_blank" rel="noopener noreferrer" class="text-accent-blue hover:underline">
                {renderInline(text.slice(index + 1, labelEnd), key)}
              </a>
            );
            index = hrefEnd + 1;
            continue;
          }
        }
      }
    }

    const url = text.slice(index).match(urlPattern)?.[0];
    if (url) {
      const href = trimTrailingUrlPunctuation(url);
      flush();
      nodes.push(
        <a key={`${keyPrefix}-${nodes.length}`} href={href} target="_blank" rel="noopener noreferrer" class="text-accent-blue hover:underline">
          {href}
        </a>
      );
      index += href.length;
      continue;
    }

    plain += text[index];
    index++;
  }

  flush();
  return nodes;
}

function safeHref(raw: string): string | null {
  const href = raw.trim();
  if (
    href.startsWith("https://") ||
    href.startsWith("http://") ||
    href.startsWith("mailto:") ||
    href.startsWith("/") ||
    href.startsWith("#")
  ) {
    return href;
  }
  return null;
}

function trimTrailingUrlPunctuation(url: string): string {
  return url.replace(/[),.;:!?]+$/, "");
}
