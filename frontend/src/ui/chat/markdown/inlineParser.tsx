import type { ComponentChildren } from "preact";
import { mediaViewerState } from "../../../state/chat/mediaViewerState";
import { viewableMediaKind } from "../files/fileMeta";
import { internalPathOpenUrl } from "../ideLinks";
import { getTextDirection, hasLtrText, isRtlText, splitBidiSegments } from "./bidi";

const urlPattern = /^https?:\/\/[^\s<]+/;

export interface InlineRenderContext {
  chatId?: string;
  cwd?: string;
  isRtl?: boolean;
}

export function renderInline(text: string, keyPrefix: string, context: InlineRenderContext = {}): ComponentChildren[] {
  const nodes: ComponentChildren[] = [];
  let plain = "";
  let index = 0;

  const flush = () => {
    if (plain) {
      if (context.isRtl || isRtlText(plain)) {
        const segments = splitBidiSegments(plain);
        for (const seg of segments) {
          if (seg.isLtr) {
            nodes.push(
              <span
                key={`${keyPrefix}-bidi-${nodes.length}`}
                dir="ltr"
                class="bidi-isolate"
              >
                {seg.text}
              </span>
            );
          } else if (seg.text) {
            nodes.push(seg.text);
          }
        }
      } else {
        nodes.push(plain);
      }
      plain = "";
    }
  };

  const addWrapped = (tag: "strong" | "em" | "del", content: string, markerLength: number, end: number) => {
    flush();
    const key = `${keyPrefix}-${nodes.length}`;
    const isLtrWrap = !isRtlText(content) && hasLtrText(content);
    const wrapDir = isLtrWrap ? "ltr" : undefined;
    const childContext = isLtrWrap ? { ...context, isRtl: false } : context;
    const children = renderInline(content, key, childContext);
    const wrapClass = "bidi-isolate";
    if (tag === "strong") nodes.push(<strong key={key} dir={wrapDir} class={wrapClass}>{children}</strong>);
    if (tag === "em") nodes.push(<em key={key} dir={wrapDir} class={wrapClass}>{children}</em>);
    if (tag === "del") nodes.push(<del key={key} dir={wrapDir} class={wrapClass}>{children}</del>);
    index = end + markerLength;
  };

  while (index < text.length) {
    if (text[index] === "`") {
      const end = text.indexOf("`", index + 1);
      if (end > index + 1) {
        flush();
        nodes.push(
          <code
            key={`${keyPrefix}-${nodes.length}`}
            dir="ltr"
            class="bg-white/[0.08] text-ink-100 px-1 py-0.5 rounded text-[12.5px] font-mono md-inline-code"
          >
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
          const href = safeHref(text.slice(hrefStart + 1, hrefEnd), context);
          if (href) {
            flush();
            const key = `${keyPrefix}-${nodes.length}`;
            const labelText = text.slice(index + 1, labelEnd);
            const labelDir = getTextDirection(labelText);
            const labelContext = { ...context, isRtl: labelDir === "rtl" };
            nodes.push(
              <a
                key={key}
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                dir={labelDir}
                class="text-accent-blue hover:underline bidi-isolate"
                onClick={(event) => maybeOpenMediaViewer(event, href)}
              >
                {renderInline(labelText, key, labelContext)}
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
        <a
          key={`${keyPrefix}-${nodes.length}`}
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          dir="ltr"
          class="text-accent-blue hover:underline bidi-isolate"
        >
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

function safeHref(raw: string, context: InlineRenderContext): string | null {
  const href = raw.trim();
  const internalHref = internalPathOpenUrl(href, context);
  if (internalHref) return internalHref;
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

// Media links produced by internalPathOpenUrl point at the inline media-open
// endpoint; render those in the in-app viewer instead of a new tab. Modified
// clicks (cmd/ctrl/shift/middle) keep the browser's default behavior.
function maybeOpenMediaViewer(event: MouseEvent, href: string): void {
  if (event.defaultPrevented) return;
  if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
  if (!href.includes("/media-open?")) return;
  const name = mediaOpenFileName(href);
  const kind = name ? viewableMediaKind(name) : null;
  if (!name || !kind) return;
  event.preventDefault();
  mediaViewerState.open({ url: href, name, kind });
}

function mediaOpenFileName(href: string): string {
  try {
    const url = new URL(href, window.location.origin);
    const path = url.searchParams.get("path") || "";
    const base = path.split("/").pop() || "";
    // Paths may carry :line or :line:column suffixes — strip them for display.
    return base.replace(/(:\d+){1,2}$/, "");
  } catch {
    return "";
  }
}
