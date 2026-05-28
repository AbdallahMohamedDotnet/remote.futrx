import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import hljs from "highlight.js/lib/core";
import "highlight.js/styles/github-dark.css";

// Register only the languages we expect to see in claude's outputs.
// Keeps the bundle ~80KB lighter than the default "all languages" build.
import bash from "highlight.js/lib/languages/bash";
import typescript from "highlight.js/lib/languages/typescript";
import javascript from "highlight.js/lib/languages/javascript";
import python from "highlight.js/lib/languages/python";
import json from "highlight.js/lib/languages/json";
import yaml from "highlight.js/lib/languages/yaml";
import css from "highlight.js/lib/languages/css";
import xml from "highlight.js/lib/languages/xml";
import go from "highlight.js/lib/languages/go";
import diff from "highlight.js/lib/languages/diff";
import markdown from "highlight.js/lib/languages/markdown";

hljs.registerLanguage("bash", bash);
hljs.registerLanguage("sh", bash);
hljs.registerLanguage("shell", bash);
hljs.registerLanguage("ts", typescript);
hljs.registerLanguage("typescript", typescript);
hljs.registerLanguage("tsx", typescript);
hljs.registerLanguage("js", javascript);
hljs.registerLanguage("javascript", javascript);
hljs.registerLanguage("jsx", javascript);
hljs.registerLanguage("py", python);
hljs.registerLanguage("python", python);
hljs.registerLanguage("json", json);
hljs.registerLanguage("yaml", yaml);
hljs.registerLanguage("yml", yaml);
hljs.registerLanguage("css", css);
hljs.registerLanguage("html", xml);
hljs.registerLanguage("xml", xml);
hljs.registerLanguage("go", go);
hljs.registerLanguage("diff", diff);
hljs.registerLanguage("md", markdown);
hljs.registerLanguage("markdown", markdown);

function highlight(code: string, lang?: string): string {
  if (lang && hljs.getLanguage(lang)) {
    try {
      return hljs.highlight(code, { language: lang, ignoreIllegals: true }).value;
    } catch {}
  }
  try {
    return hljs.highlightAuto(code).value;
  } catch {
    return escapeHtml(code);
  }
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]!)
  );
}

export function Markdown({ children }: { children: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        code({ inline, className, children, ...props }: any) {
          const match = /language-(\w+)/.exec(className || "");
          const text = String(children).replace(/\n$/, "");
          if (inline) {
            return (
              <code class="bg-ink-700 text-ink-100 px-1 py-0.5 rounded text-[12.5px] font-mono" {...props}>
                {text}
              </code>
            );
          }
          const html = highlight(text, match?.[1]);
          return (
            <div class="relative my-2 rounded-md border border-ink-500 bg-ink-700 overflow-hidden">
              {match && (
                <div class="px-3 py-1 text-[11px] uppercase tracking-wider text-ink-300 border-b border-ink-500 bg-ink-600/50">
                  {match[1]}
                </div>
              )}
              <pre class="overflow-x-auto p-3 text-[12.5px] leading-relaxed font-mono">
                <code dangerouslySetInnerHTML={{ __html: html }} />
              </pre>
            </div>
          );
        },
        a({ href, children, ...props }: any) {
          return (
            <a href={href} target="_blank" rel="noopener noreferrer"
              class="text-accent-blue hover:underline" {...props}>{children}</a>
          );
        },
        table({ children }: any) {
          return (
            <div class="overflow-x-auto my-3 border border-ink-500 rounded">
              <table class="w-full text-sm border-collapse">{children}</table>
            </div>
          );
        },
        thead({ children }: any) {
          return <thead class="bg-ink-700">{children}</thead>;
        },
        th({ children }: any) {
          return <th class="text-left px-3 py-1.5 font-semibold border-b border-ink-500 text-ink-100">{children}</th>;
        },
        td({ children }: any) {
          return <td class="px-3 py-1.5 border-b border-ink-500/70">{children}</td>;
        },
        ul({ children }: any) {
          return <ul class="list-disc list-outside pl-5 my-2 space-y-0.5">{children}</ul>;
        },
        ol({ children }: any) {
          return <ol class="list-decimal list-outside pl-5 my-2 space-y-0.5">{children}</ol>;
        },
        blockquote({ children }: any) {
          return <blockquote class="border-l-2 border-ink-500 pl-3 my-2 text-ink-200 italic">{children}</blockquote>;
        },
        h1: ({ children }: any) => <h1 class="text-xl font-bold mt-3 mb-1.5">{children}</h1>,
        h2: ({ children }: any) => <h2 class="text-lg font-bold mt-3 mb-1.5">{children}</h2>,
        h3: ({ children }: any) => <h3 class="text-base font-bold mt-2 mb-1">{children}</h3>,
        p:  ({ children }: any) => <p class="my-1.5 leading-relaxed">{children}</p>,
        hr: () => <hr class="my-3 border-ink-500" />,
      }}
    >
      {children}
    </ReactMarkdown>
  );
}
