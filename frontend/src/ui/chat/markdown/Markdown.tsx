import { useMemo } from "preact/hooks";
import { parseMarkdown } from "./blockParser";
import { highlightCode } from "./highlight";
import { renderInline } from "./inlineParser";
import type { MarkdownBlock } from "./types";
import { getTextAlignClass, getTextDirection, isRtlText } from "./bidi";

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
      const dir = isRtl ? "rtl" : "ltr";
      const align = getTextAlignClass(block.text);
      return <p key={key} dir={dir} class={`my-1.5 leading-relaxed ${align}`}>{renderInline(block.text, key, { ...context, isRtl })}</p>;
    }
    case "heading":
      return renderHeading(block.level, block.text, key, context);
    case "code":
      return (
        <div key={key} dir="ltr" class="relative my-3 rounded-lg border border-white/10 bg-[#101318] overflow-hidden text-left">
          {block.lang && (
            <div dir="ltr" class="px-3 py-1 text-[11px] text-ink-300 border-b border-white/10 bg-white/[0.04] text-left">
              {block.lang}
            </div>
          )}
          <pre dir="ltr" class="md-code overflow-x-auto touch-scroll p-3 text-[12.5px] leading-relaxed font-mono text-left">
            <code dir="ltr">{highlightCode(block.text, block.lang)}</code>
          </pre>
        </div>
      );
    case "blockquote": {
      const hasRtl = block.children.some((child) => {
        if ("text" in child && typeof child.text === "string") return isRtlText(child.text);
        return false;
      });
      const dir = hasRtl ? "rtl" : "ltr";
      const borderClass = dir === "rtl"
        ? "border-r-2 border-accent-blue/45 pr-3 text-right"
        : "border-l-2 border-accent-blue/45 pl-3 text-left";
      const blockquoteContext = { ...context, isRtl: hasRtl };
      return (
        <blockquote key={key} dir={dir} class={`${borderClass} my-2 text-ink-200 italic`}>
          {block.children.map((child, index) => renderBlock(child, `${key}-q-${index}`, blockquoteContext))}
        </blockquote>
      );
    }
    case "list":
      return renderList(block, key, context);
    case "table":
      return (
        <div key={key} class="overflow-x-auto touch-scroll my-3 border border-white/10 rounded-lg">
          <table class="w-full text-sm border-collapse">
            <thead class="bg-white/[0.04]">
              <tr>
                {block.header.map((cell, index) => {
                  const cellIsRtl = isRtlText(cell);
                  const cellDir = cellIsRtl ? "rtl" : "ltr";
                  const align = getTextAlignClass(cell);
                  return (
                    <th key={index} dir={cellDir} class={`${align} px-3 py-1.5 font-semibold border-b border-white/10 text-ink-100`}>
                      {renderInline(cell, `${key}-h-${index}`, { ...context, isRtl: cellIsRtl })}
                    </th>
                  );
                })}
              </tr>
            </thead>
            <tbody>
              {block.rows.map((row, rowIndex) => (
                <tr key={rowIndex}>
                  {row.map((cell, cellIndex) => {
                    const cellIsRtl = isRtlText(cell);
                    const cellDir = cellIsRtl ? "rtl" : "ltr";
                    const align = getTextAlignClass(cell);
                    return (
                      <td key={cellIndex} dir={cellDir} class={`${align} px-3 py-1.5 border-b border-white/10`}>
                        {renderInline(cell, `${key}-r-${rowIndex}-${cellIndex}`, { ...context, isRtl: cellIsRtl })}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
    case "hr":
      return <hr key={key} class="my-3 border-white/10" />;
  }
}

function renderHeading(level: 1 | 2 | 3 | 4 | 5 | 6, text: string, key: string, context: MarkdownRenderContext) {
  const isRtl = isRtlText(text);
  const dir = isRtl ? "rtl" : "ltr";
  const align = getTextAlignClass(text);
  const headingContext = { ...context, isRtl };
  if (level === 1) return <h1 key={key} dir={dir} class={`text-xl font-bold mt-3 mb-1.5 ${align}`}>{renderInline(text, key, headingContext)}</h1>;
  if (level === 2) return <h2 key={key} dir={dir} class={`text-lg font-bold mt-3 mb-1.5 ${align}`}>{renderInline(text, key, headingContext)}</h2>;
  return <h3 key={key} dir={dir} class={`text-base font-bold mt-2 mb-1 ${align}`}>{renderInline(text, key, headingContext)}</h3>;
}

function renderList(block: Extract<MarkdownBlock, { type: "list" }>, key: string, context: MarkdownRenderContext) {
  const isRtl = !!context.docIsRtl || block.items.some((item) => isRtlText(item.text));
  const dir = isRtl ? "rtl" : "ltr";
  const paddingClass = isRtl ? "pr-5 pl-0 text-right" : "pl-5 pr-0 text-left";
  const className = `${block.ordered ? "list-decimal" : "list-disc"} list-outside ${paddingClass} my-2 space-y-0.5`;
  const items = block.items.map((item, index) => {
    const itemDir = isRtl ? "rtl" : "ltr";
    const itemAlign = isRtl ? "text-right" : "text-left";
    const itemContext = { ...context, isRtl };
    const content = renderInline(item.text, `${key}-li-${index}`, itemContext);
    if (item.checked === undefined) return <li key={index} dir={itemDir} class={itemAlign}>{content}</li>;
    return (
      <li key={index} dir={itemDir} class={`list-none ${isRtl ? "-mr-5" : "-ml-5"} flex items-start gap-2 ${itemAlign}`}>
        <input
          type="checkbox"
          checked={item.checked}
          disabled
          class="mt-1 h-3.5 w-3.5 flex-none"
        />
        <span>{content}</span>
      </li>
    );
  });

  if (block.ordered) {
    return <ol key={key} dir={dir} class={className} start={block.start}>{items}</ol>;
  }
  return <ul key={key} dir={dir} class={className}>{items}</ul>;
}
