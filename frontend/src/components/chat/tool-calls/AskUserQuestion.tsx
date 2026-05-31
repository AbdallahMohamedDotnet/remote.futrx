// Render claude's AskUserQuestion tool calls as a paginated wizard.
// One question per page; Next disabled until current question is answered.

import { useEffect, useMemo, useState } from "preact/hooks";
import { Check, ChevronLeft, ChevronRight } from "../../ui/icons";

interface Question {
  question: string;
  header?: string;
  multiSelect?: boolean;
  options: Array<{ label: string; description?: string }>;
}

interface Input {
  questions?: Question[];
}

interface Props {
  toolUseId: string;
  chatId: string;
  input: Input;
  onSubmit: (text: string) => void;
}

const LS_KEY_PREFIX = "askq-answered:";
function readAnswered(toolUseId: string): string | null {
  try { return localStorage.getItem(LS_KEY_PREFIX + toolUseId); } catch { return null; }
}
function writeAnswered(toolUseId: string, summary: string) {
  try { localStorage.setItem(LS_KEY_PREFIX + toolUseId, summary); } catch {}
}

export function AskUserQuestion({ toolUseId, input, onSubmit }: Props) {
  const questions = input.questions ?? [];
  const total = questions.length;
  const initialAnswered = useMemo(() => readAnswered(toolUseId), [toolUseId]);
  const [answered, setAnswered] = useState<string | null>(initialAnswered);
  const [page, setPage] = useState(0);
  const [selections, setSelections] = useState<Record<number, Set<number>>>(() => {
    const init: Record<number, Set<number>> = {};
    questions.forEach((_, i) => { init[i] = new Set(); });
    return init;
  });
  const [other, setOther] = useState<Record<number, string>>({});
  const [otherActive, setOtherActive] = useState<Record<number, boolean>>({});

  useEffect(() => { setAnswered(initialAnswered); }, [initialAnswered]);

  if (!questions.length) return null;

  if (answered) {
    return (
      <div class="my-2 rounded-lg border border-white/10 bg-white/[0.04] p-3 text-sm">
        <div class="flex items-center gap-2 text-ink-300 text-[11px] mb-1.5">
          <Check class="w-3 h-3 text-accent-green" /> Answered
        </div>
        <div class="text-ink-200 whitespace-pre-wrap text-[13px]">{answered}</div>
      </div>
    );
  }

  function toggle(qi: number, oi: number, multi: boolean) {
    setSelections((prev) => {
      const next = { ...prev };
      const set = new Set(next[qi]);
      if (multi) {
        if (set.has(oi)) set.delete(oi); else set.add(oi);
      } else {
        set.clear();
        set.add(oi);
      }
      next[qi] = set;
      return next;
    });
    setOtherActive((p) => ({ ...p, [qi]: false }));
  }

  function activateOther(qi: number, multi: boolean) {
    setOtherActive((p) => ({ ...p, [qi]: true }));
    if (!multi) {
      setSelections((prev) => ({ ...prev, [qi]: new Set() }));
    }
  }

  function questionAnswered(qi: number): boolean {
    const sel = selections[qi];
    const otherText = otherActive[qi] ? (other[qi] || "").trim() : "";
    if (otherText.length > 0) return true;
    return sel.size > 0;
  }

  function chosenLabels(qi: number): string[] {
    const q = questions[qi];
    const chosen: string[] = [];
    selections[qi].forEach((oi) => chosen.push(q.options[oi].label));
    if (otherActive[qi]) {
      const t = (other[qi] || "").trim();
      if (t) chosen.push(t);
    }
    return chosen;
  }

  function summarize(): { text: string; preview: string } {
    const parts: string[] = [];
    const preview: string[] = [];
    for (let qi = 0; qi < questions.length; qi++) {
      const q = questions[qi];
      const chosen = chosenLabels(qi);
      parts.push(`Q: ${q.question}\nA: ${chosen.join("; ")}`);
      preview.push(`${q.header ?? "Answer"}: ${chosen.join(", ")}`);
    }
    return { text: parts.join("\n\n"), preview: preview.join(" · ") };
  }

  function submit() {
    const s = summarize();
    writeAnswered(toolUseId, s.preview);
    setAnswered(s.preview);
    onSubmit(s.text);
  }

  const q = questions[page];
  const multi = !!q.multiSelect;
  const sel = selections[page] ?? new Set<number>();
  const isOtherActive = !!otherActive[page];
  const canAdvance = questionAnswered(page);
  const isLast = page === total - 1;

  return (
    <div class="my-2 rounded-lg border border-accent-blue/40 bg-accent-blue/5 overflow-hidden">
      <div class="px-3 py-2 bg-accent-blue/10 text-[11px] text-accent-blue
                  flex items-center justify-between gap-2 border-b border-accent-blue/20">
        <span>Agent is asking</span>
        {total > 1 && (
            <span class="flex items-center gap-1.5 text-accent-blue/80">
            <span class="font-mono text-[10.5px]">{page + 1} / {total}</span>
            <span class="flex gap-1">
              {questions.map((_, i) => (
                <span
                  key={i}
                  class={`w-1.5 h-1.5 rounded-full
                    ${i === page ? "bg-accent-blue"
                      : questionAnswered(i) ? "bg-accent-green" : "bg-accent-blue/30"}`}
                />
              ))}
            </span>
          </span>
        )}
      </div>

      <div class="p-3 space-y-3">
        {q.header && (
        <div class="inline-block text-[10px] font-mono
                      px-1.5 py-0.5 rounded bg-white/[0.06] text-ink-200 border border-white/10">
            {q.header}
          </div>
        )}
        <div class="text-[14px] text-ink-100 font-medium leading-snug">{q.question}</div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          {q.options.map((opt, oi) => {
            const active = sel.has(oi) && !isOtherActive;
            return (
              <button
                key={oi}
                type="button"
                onClick={() => toggle(page, oi, multi)}
                class={`text-left rounded-md border px-3 py-2.5 min-h-12 transition-colors
                        ${active
                          ? "border-accent-blue bg-accent-blue/15"
                          : "border-white/10 bg-white/[0.04] hover:bg-white/[0.07]"}`}
              >
                <div class="flex items-start gap-2">
                  <div class={`flex-none mt-0.5 w-4 h-4 ${multi ? "rounded-sm" : "rounded-full"}
                               border ${active ? "bg-accent-blue border-accent-blue" : "border-ink-300"}
                               grid place-items-center`}>
                    {active && <Check class="w-3 h-3 text-white" />}
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class={`text-[13px] font-medium ${active ? "text-accent-blue" : "text-ink-100"}`}>
                      {opt.label}
                    </div>
                    {opt.description && (
                      <div class="text-[11.5px] text-ink-300 mt-0.5 leading-snug">
                        {opt.description}
                      </div>
                    )}
                  </div>
                </div>
              </button>
            );
          })}
          <button
            type="button"
            onClick={() => activateOther(page, multi)}
            class={`text-left rounded-md border px-3 py-2.5 min-h-12 transition-colors
                    ${isOtherActive
                      ? "border-accent-blue bg-accent-blue/15"
                      : "border-white/10 border-dashed bg-white/[0.03] hover:bg-white/[0.07]"} sm:col-span-2`}
          >
            <div class="flex items-start gap-2">
              <div class={`flex-none mt-0.5 w-4 h-4 ${multi ? "rounded-sm" : "rounded-full"}
                           border ${isOtherActive ? "bg-accent-blue border-accent-blue" : "border-ink-300"}
                           grid place-items-center`}>
                {isOtherActive && <Check class="w-3 h-3 text-white" />}
              </div>
              <div class="flex-1 min-w-0">
                <div class={`text-[13px] font-medium ${isOtherActive ? "text-accent-blue" : "text-ink-200"}`}>
                  Other (write your own)
                </div>
                {isOtherActive && (
                  <textarea
                    autofocus
                    rows={2}
                    value={other[page] || ""}
                    onInput={(e) => setOther((p) => ({ ...p, [page]: (e.currentTarget as HTMLTextAreaElement).value }))}
                    onClick={(e) => e.stopPropagation()}
                    placeholder="Your custom answer"
                    class="mt-2 w-full resize-none bg-[#0b0d11] border border-white/10 rounded-md
                           text-ink-100 text-[13px] px-2 py-1 focus:outline-none focus:border-accent-blue"
                  />
                )}
              </div>
            </div>
          </button>
        </div>

        <div class="flex items-center justify-between pt-1">
          <button
            type="button"
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
            class="flex items-center gap-1 text-sm px-3 h-10 rounded-md
                   text-ink-200 hover:text-ink-100 hover:bg-white/[0.08]
                   disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent"
          >
            <ChevronLeft class="w-3.5 h-3.5" /> Back
          </button>
          {isLast ? (
            <button
              type="button"
              onClick={submit}
              disabled={!canAdvance}
              class="flex items-center gap-1.5 bg-accent-blue hover:bg-accent-blue/85 h-10
                     disabled:bg-ink-500 disabled:cursor-not-allowed
                     text-white text-sm font-medium px-3.5 rounded-md"
            >
              Send answer <ChevronRight class="w-3.5 h-3.5" />
            </button>
          ) : (
            <button
              type="button"
              onClick={() => setPage((p) => Math.min(total - 1, p + 1))}
              disabled={!canAdvance}
              class="flex items-center gap-1.5 bg-accent-blue/90 hover:bg-accent-blue h-10
                     disabled:bg-ink-500 disabled:cursor-not-allowed
                     text-white text-sm font-medium px-3.5 rounded-md"
            >
              Next <ChevronRight class="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
