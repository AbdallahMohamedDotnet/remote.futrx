import type { ComponentChildren } from "preact";

type TokenKind = "comment" | "string" | "keyword" | "number" | "literal";

const sharedKeywords = new Set([
  "async", "await", "break", "case", "catch", "class", "const", "continue", "default", "do",
  "else", "export", "extends", "finally", "for", "from", "function", "if", "import", "in",
  "interface", "let", "new", "of", "return", "switch", "throw", "try", "type", "var", "while",
]);

const keywordSets: Record<string, Set<string>> = {
  bash: new Set(["case", "do", "done", "elif", "else", "esac", "fi", "for", "function", "if", "in", "then", "while"]),
  css: new Set(["important", "media", "supports", "keyframes", "from", "to"]),
  go: new Set(["break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var"]),
  js: sharedKeywords,
  json: new Set(),
  py: new Set(["and", "as", "async", "await", "break", "class", "continue", "def", "elif", "else", "except", "finally", "for", "from", "if", "import", "in", "is", "lambda", "not", "or", "pass", "raise", "return", "try", "while", "with", "yield"]),
  ts: sharedKeywords,
  yaml: new Set(["true", "false", "null"]),
};

const literals = new Set(["true", "false", "null", "undefined", "nil", "None", "True", "False"]);

export function highlightCode(code: string, lang?: string): ComponentChildren {
  const normalized = normalizeLang(lang);
  if (normalized === "diff") return renderDiff(code);
  if (!normalized || normalized === "markdown") return code;
  return tokenize(code, normalized).map((token, index) => {
    if (typeof token === "string") return token;
    return <span key={index} class={`md-code-${token.kind}`}>{token.text}</span>;
  });
}

function renderDiff(code: string): ComponentChildren[] {
  const lines = code.split("\n");
  return lines.map((line, index) => {
    const kind = line.startsWith("+")
      ? "addition"
      : line.startsWith("-")
        ? "deletion"
        : line.startsWith("@@") || line.startsWith("diff ") || line.startsWith("index ")
          ? "meta"
          : "";
    return (
      <span key={index} class={kind ? `md-code-${kind}` : undefined}>
        {line}
        {index < lines.length - 1 ? "\n" : ""}
      </span>
    );
  });
}

function tokenize(code: string, lang: string): Array<string | { kind: TokenKind; text: string }> {
  const tokens: Array<string | { kind: TokenKind; text: string }> = [];
  const keywords = keywordSets[lang] ?? sharedKeywords;
  let index = 0;
  let plain = "";

  const flush = () => {
    if (plain) {
      tokens.push(plain);
      plain = "";
    }
  };

  while (index < code.length) {
    const comment = matchAt(code, index, commentPattern(lang));
    if (comment) {
      flush();
      tokens.push({ kind: "comment", text: comment });
      index += comment.length;
      continue;
    }

    const string = matchAt(code, index, stringPattern(lang));
    if (string) {
      flush();
      tokens.push({ kind: "string", text: string });
      index += string.length;
      continue;
    }

    const number = matchAt(code, index, /\b(?:0x[\da-fA-F]+|\d+(?:\.\d+)?)\b/y);
    if (number) {
      flush();
      tokens.push({ kind: "number", text: number });
      index += number.length;
      continue;
    }

    const word = matchAt(code, index, /[A-Za-z_$][\w$-]*/y);
    if (word) {
      if (keywords.has(word) || literals.has(word)) {
        flush();
        tokens.push({ kind: literals.has(word) ? "literal" : "keyword", text: word });
      } else {
        plain += word;
      }
      index += word.length;
      continue;
    }

    plain += code[index];
    index++;
  }

  flush();
  return tokens;
}

function normalizeLang(lang?: string): string | undefined {
  const lower = lang?.toLowerCase();
  if (!lower) return undefined;
  if (lower === "shell" || lower === "sh") return "bash";
  if (lower === "javascript" || lower === "jsx") return "js";
  if (lower === "typescript" || lower === "tsx") return "ts";
  if (lower === "python") return "py";
  if (lower === "yml") return "yaml";
  if (lower === "md") return "markdown";
  if (lower === "html" || lower === "xml") return "html";
  return lower;
}

function commentPattern(lang: string): RegExp {
  if (lang === "bash" || lang === "py" || lang === "yaml") return /#[^\n]*/y;
  if (lang === "html") return /<!--[\s\S]*?-->/y;
  if (lang === "css") return /\/\*[\s\S]*?\*\//y;
  return /\/\/[^\n]*|\/\*[\s\S]*?\*\//y;
}

function stringPattern(lang: string): RegExp {
  if (lang === "html") return /"[^"]*"|'[^']*'/y;
  return /`(?:\\[\s\S]|[^`\\])*`|'(?:\\[\s\S]|[^'\\])*'|"(?:\\[\s\S]|[^"\\])*"/y;
}

function matchAt(text: string, index: number, pattern: RegExp): string | null {
  pattern.lastIndex = index;
  return pattern.exec(text)?.[0] ?? null;
}
