// BiDi and text direction helpers for chat and markdown rendering.
// Detects Arabic and other RTL scripts to apply proper text direction,
// text alignment, and Unicode BiDi isolation without altering global app layout.

const RTL_REGEX = /[\u0590-\u08FF\uFB50-\uFDFF\uFE70-\uFEFF]/;
const LTR_REGEX = /[A-Za-z\u00C0-\u024F]/;

/**
 * Returns true if the provided text contains any Arabic or RTL characters.
 */
export function isRtlText(text?: string | null): boolean {
  if (!text) return false;
  return RTL_REGEX.test(text);
}

/**
 * Returns true if the provided text contains any Latin/LTR characters.
 */
export function hasLtrText(text?: string | null): boolean {
  if (!text) return false;
  return LTR_REGEX.test(text);
}

/**
 * Resolves the appropriate HTML `dir` attribute ("rtl" | "ltr") based on content.
 */
export function getTextDirection(text?: string | null): "rtl" | "ltr" {
  return isRtlText(text) ? "rtl" : "ltr";
}

/**
 * Returns the matching Tailwind text alignment utility class.
 */
export function getTextAlignClass(text?: string | null): "text-right" | "text-left" {
  return isRtlText(text) ? "text-right" : "text-left";
}

export interface BidiSegment {
  text: string;
  isLtr: boolean;
}

/**
 * Splits text into alternating RTL/neutral and isolated coherent LTR segments.
 * Coherent LTR phrases (e.g. "OAuth 2.0 / OpenID Connect (OIDC)", "Django REST Framework",
 * "Single Page Application - SPA", "Angular", "(Frontend / Client-side UI)") are isolated
 * as coherent LTR units with their internal punctuation intact.
 */
export function splitBidiSegments(text: string): BidiSegment[] {
  if (!text) return [];
  // If the text does not contain both RTL and LTR characters, no splitting is needed.
  if (!isRtlText(text) || !hasLtrText(text)) {
    return [{ text, isLtr: !isRtlText(text) && hasLtrText(text) }];
  }

  // A coherent LTR run:
  // Starts with optional opening delimiter attached to Latin: ( [ { " ' “ ‘
  // Followed by Latin letter or digit
  // Followed greedily by non-RTL characters (Latin letters, digits, symbols, internal spaces/slashes/dashes/dots/parens)
  // Ending at the last Latin letter, digit, or closing delimiter ) ] } " ' ” ’
  const ltrPattern = /(?:[\(\[\{"'“‘]*)?[A-Za-z0-9\u00C0-\u024F](?:[^\u0590-\u08FF\uFB50-\uFDFF\uFE70-\uFEFF،؛؟]*[A-Za-z0-9\u00C0-\u024F\)\}\]"'”’])?/g;

  const segments: BidiSegment[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = ltrPattern.exec(text)) !== null) {
    const matchStart = match.index;
    let matchText = match[0];

    // Refine match: if match has a trailing dot/comma/colon/semicolon that is sentence punctuation
    // (i.e. not balanced by a leading bracket/quote and not an internal decimal like 2.0), strip it.
    while (matchText.length > 1 && /[.,;:!?،؛؟]$/.test(matchText)) {
      const lastChar = matchText[matchText.length - 1];
      const openParens = (matchText.match(/\(/g) || []).length;
      const closeParens = (matchText.match(/\)/g) || []).length;
      const openBrackets = (matchText.match(/\[/g) || []).length;
      const closeBrackets = (matchText.match(/\]/g) || []).length;
      if (lastChar === ")" && openParens > closeParens) break;
      if (lastChar === "]" && openBrackets > closeBrackets) break;
      matchText = matchText.slice(0, -1);
    }

    if (matchStart > lastIndex) {
      segments.push({
        text: text.slice(lastIndex, matchStart),
        isLtr: false,
      });
    }

    segments.push({
      text: matchText,
      isLtr: true,
    });

    lastIndex = matchStart + matchText.length;
    ltrPattern.lastIndex = lastIndex;
  }

  if (lastIndex < text.length) {
    segments.push({
      text: text.slice(lastIndex),
      isLtr: false,
    });
  }

  return segments;
}
