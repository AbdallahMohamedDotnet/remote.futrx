import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import type { ChatProvider } from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";
import { skillService } from "../../../services/skillService";
import { ChevronDown, Code, Search } from "../../ui/icons";

export function SkillPicker({
  provider,
  projectId,
  onSelect,
}: {
  provider: ChatProvider;
  projectId?: string;
  onSelect: (skill: RegisteredSkill) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [skills, setSkills] = useState<RegisteredSkill[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const rootRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    let cancelled = false;
    setOpen(false);
    setQuery("");
    setError("");
    setLoading(true);
    skillService.list(provider, projectId)
      .then((items) => {
        if (!cancelled) setSkills(items);
      })
      .catch((err) => {
        if (!cancelled) {
          setSkills([]);
          setError((err as Error).message || "Could not load skills");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [provider, projectId]);

  useEffect(() => {
    if (!open) return;
    const id = window.setTimeout(() => searchRef.current?.focus(), 0);
    function closeOnOutsideClick(event: MouseEvent) {
      const target = event.target as Node | null;
      if (target && !rootRef.current?.contains(target)) setOpen(false);
    }
    window.addEventListener("mousedown", closeOnOutsideClick);
    return () => {
      window.clearTimeout(id);
      window.removeEventListener("mousedown", closeOnOutsideClick);
    };
  }, [open]);

  const filteredSkills = useMemo(() => {
    const term = query.trim().toLowerCase();
    if (!term) return skills;
    return skills.filter((skill) =>
      `${skill.name} ${skill.description || ""} ${skill.source || ""}`.toLowerCase().includes(term)
    );
  }, [query, skills]);

  function choose(skill: RegisteredSkill) {
    onSelect(skill);
    setOpen(false);
    setQuery("");
  }

  const providerLabel = provider === "codex" ? "Codex" : "Claude";

  return (
    <div ref={rootRef} class="relative w-full flex-none sm:w-[300px] lg:w-[320px]">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        class={`codex-skill-control flex h-[58px] w-full items-center justify-between gap-3 rounded-md border px-3 text-left transition
                ${open ? "bg-accent-blue/[0.14] border-accent-blue/30 text-accent-blue" : "bg-white/[0.05] border-white/10 text-ink-200 hover:bg-white/[0.08]"}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        title={`${providerLabel} skills`}
      >
        <span class="flex min-w-0 items-center gap-2.5">
          <span class="inline-flex h-8 w-8 flex-none items-center justify-center rounded-md bg-white/[0.08] text-ink-100">
            <Code class="h-4 w-4" />
          </span>
          <span class="min-w-0">
            <span class="block truncate text-[13px] font-semibold text-ink-100">Skill set</span>
            <span class="block truncate text-[11px] text-ink-400">
              {loading ? "Loading skills" : `${skills.length} ${providerLabel} available`}
            </span>
          </span>
        </span>
        <span class="inline-flex flex-none items-center gap-2">
          <span class="rounded bg-white/10 px-1.5 py-0.5 text-[11px] leading-none text-ink-300">
            {loading ? "..." : skills.length}
          </span>
          <ChevronDown class="h-3.5 w-3.5" />
        </span>
      </button>

      {open && (
        <div
          class="absolute left-0 bottom-full z-40 mb-2 w-[calc(100vw-1.5rem)] sm:left-auto sm:right-0 sm:w-[460px]
                 rounded-lg border border-white/10 bg-[#14161d] shadow-2xl overflow-hidden"
        >
          <div class="p-2 border-b border-white/10 bg-[#191a1f]">
            <div class="h-9 rounded-md bg-[#0b0d11] border border-white/10 px-2.5 flex items-center gap-2">
              <Search class="w-3.5 h-3.5 text-ink-400 flex-none" />
              <input
                ref={searchRef}
                value={query}
                onInput={(event) => setQuery((event.currentTarget as HTMLInputElement).value)}
                class="min-w-0 flex-1 bg-transparent text-[13px] text-ink-100 placeholder:text-ink-500 focus:outline-none"
                placeholder={`Search ${providerLabel} skills`}
              />
            </div>
          </div>

          <div class="max-h-[300px] overflow-y-auto py-1">
            {error ? (
              <div class="px-3 py-3 text-[12px] text-red-300">{error}</div>
            ) : loading ? (
              <div class="px-3 py-3 text-[12px] text-ink-400">Loading skills...</div>
            ) : filteredSkills.length === 0 ? (
              <div class="px-3 py-3 text-[12px] text-ink-400">
                {skills.length === 0 ? `No ${providerLabel} skills registered` : "No matching skills"}
              </div>
            ) : (
              filteredSkills.map((skill) => (
                <button
                  key={`${skill.source || "skill"}:${skill.name}`}
                  type="button"
                  onClick={() => choose(skill)}
                  class="w-full text-left px-3 py-2.5 hover:bg-white/[0.07] focus:bg-white/[0.07] focus:outline-none"
                  role="option"
                >
                  <div class="flex items-center gap-2 min-w-0">
                    <span class="truncate text-[13px] font-medium text-ink-100">{skill.name}</span>
                    {skill.source && (
                      <span class="flex-none rounded bg-white/[0.08] px-1.5 py-0.5 text-[10px] uppercase text-ink-400">
                        {skill.source}
                      </span>
                    )}
                  </div>
                  {skill.description && (
                    <div class="mt-1 max-h-8 overflow-hidden text-[12px] leading-4 text-ink-400">
                      {skill.description}
                    </div>
                  )}
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
