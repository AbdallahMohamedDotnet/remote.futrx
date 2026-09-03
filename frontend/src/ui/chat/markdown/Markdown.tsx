import { useMemo } from "preact/hooks";
import { parseMarkdown } from "./blockParser";
import { highlightCode } from "./highlight";
import { renderInline } from "./inlineParser";
import type { MarkdownBlock } from "./types";
import { getTextAlignClass, isRtlText } from "./bidi";

export function Markdown({ children, chatId, cwd }: { children: string; chatId?: string; cwd?: string }) {
  const docIsRtl = isRtlText(children);
  const blocks = useMemo(() => parseMarkdown(children), [children]);
  const context: MarkdownRenderContext = { chatId, cwd, docIsRtl };
  return <>{blocks.map((block, index) => renderBlock(block, `md-${index}`, context))}</>;
}

interface MarkdownRenderContext {
  chatId?: string;
  cwd?: string;
  docIsRtl?: boolean;
  isRtl?: boolean;
}

function renderBlock(block: MarkdownBlock, key: string, context: MarkdownRenderContext) {
  switch (block.type) {
    case "paragraph": {
      const isRtl = isRtlText(block.text);
      return <p key={key} dir={isRtl ? "rtl" : "ltr"} class={`my-1.5 leading-relaxed break-words [overflow-wrap:anywhere] ${getTextAlignClass(block.text)}`}>{renderInline(block.text, key, { ...context, isRtl })}</p>;
    }
    case "heading":
      return renderHeading(block.level, block.text, key, context);
    case "code":
      return (
        <div key={key} dir="ltr" class="relative my-3 rounded-lg border border-line bg-surface overflow-hidden text-left">
          {block.lang && <div dir="ltr" class="px-3 py-1 text-[11px] text-ink-300 border-b border-line bg-tint text-left">{block.lang}</div>}
          <pre dir="ltr" class="md-code overflow-x-auto touch-scroll p-3 text-[12.5px] leading-relaxed font-mono text-left">
            <code dir="ltr">{highlightCode(block.text, block.lang)}</code>
          </pre>
        </div>
      );
    case "blockquote": {
      const hasRtl = block.children.some((child) => "text" in child && typeof child.text === "string" && isRtlText(child.text));
      const borderClass = hasRtl ? "border-r-2 border-accent-blue/45 pr-3 text-right" : "border-l-2 border-accent-blue/45 pl-3 text-left";
      return (
        <blockquote key={key} dir={hasRtl ? "rtl" : "ltr"} class={`${borderClass} my-2 text-ink-200 italic min-w-0 break-words [overflow-wrap:anywhere]`}>
          {block.children.map((child, index) => renderBlock(child, `${key}-q-${index}`, { ...context, isRtl: hasRtl }))}
        </blockquote>
      );
    }
    case "list":
      return renderList(block, key, context);
    case "table":
      return (
        <div key={key} class="overflow-x-auto touch-scroll my-3 border border-line rounded-lg">
          <table class="w-full text-sm border-collapse">
            <thead class="bg-tint">
              <tr>{block.header.map((cell, index) => { const rtl = isRtlText(cell); return <th key={index} dir={rtl ? "rtl" : "ltr"} class={`${getTextAlignClass(cell)} px-3 py-1.5 font-semibold border-b border-line text-ink-100 [overflow-wrap:anywhere]`}>{renderInline(cell, `${key}-h-${index}`, { ...context, isRtl: rtl })}</th>; })}</tr>
            </thead>
            <tbody>{block.rows.map((row, rowIndex) => <tr key={rowIndex}>{row.map((cell, cellIndex) => { const rtl = isRtlText(cell); return <td key={cellIndex} dir={rtl ? "rtl" : "ltr"} class={`${getTextAlignClass(cell)} px-3 py-1.5 border-b border-line [overflow-wrap:anywhere]`}>{renderInline(cell, `${key}-r-${rowIndex}-${cellIndex}`, { ...context, isRtl: rtl })}</td>; })}</tr>)}</tbody>
          </table>
        </div>
      );
    case "hr":
      return <hr key={key} class="my-3 border-line" />;
  }
}

function renderHeading(level: 1 | 2 | 3 | 4 | 5 | 6, text: string, key: string, context: MarkdownRenderContext) {
  const rtl = isRtlText(text);
  const dir = rtl ? "rtl" : "ltr";
  const align = getTextAlignClass(text);
  const c = { ...context, isRtl: rtl };
  if (level === 1) return <h1 key={key} dir={dir} class={`text-xl font-bold mt-3 mb-1.5 break-words [overflow-wrap:anywhere] ${align}`}>{renderInline(text, key, c)}</h1>;
  if (level === 2) return <h2 key={key} dir={dir} class={`text-lg font-bold mt-3 mb-1.5 break-words [overflow-wrap:anywhere] ${align}`}>{renderInline(text, key, c)}</h2>;
  return <h3 key={key} dir={dir} class={`text-base font-bold mt-2 mb-1 break-words [overflow-wrap:anywhere] ${align}`}>{renderInline(text, key, c)}</h3>;
}

function renderList(block: Extract<MarkdownBlock, { type: "list" }>, key: string, context: MarkdownRenderContext) {
  const rtl = !!context.docIsRtl || block.items.some((item) => isRtlText(item.text));
  const className = `${block.ordered ? "list-decimal" : "list-disc"} list-outside ${rtl ? "pr-5 pl-0 text-right" : "pl-5 pr-0 text-left"} my-2 space-y-0.5`;
  const items = block.items.map((item, index) => {
    const content = renderInline(item.text, `${key}-li-${index}`, { ...context, isRtl: rtl });
    if (item.checked === undefined) return <li key={index} dir={rtl ? "rtl" : "ltr"} class={rtl ? "text-right" : "text-left"}>{content}</li>;
    return (
      <li key={index} dir={rtl ? "rtl" : "ltr"} class={`list-none ${rtl ? "-mr-5" : "-ml-5"} flex items-start gap-2 ${rtl ? "text-right" : "text-left"}`}>
        <input type="checkbox" checked={item.checked} disabled class="mt-1 h-3.5 w-3.5 flex-none" />
        <span>{content}</span>
      </li>
    );
  });
  return block.ordered ? <ol key={key} dir={rtl ? "rtl" : "ltr"} class={className} start={block.start}>{items}</ol> : <ul key={key} dir={rtl ? "rtl" : "ltr"} class={className}>{items}</ul>;
}
