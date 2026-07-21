import { ArrowDown } from "../../primitives/icons";

export function JumpToLatestButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      class="absolute right-4 bottom-4 h-10 w-10 rounded-md bg-[#151922] border border-white/[0.12]
             text-ink-100 shadow-xl grid place-items-center hover:bg-[#1b202b] active:scale-[0.98] transition"
      aria-label="Jump to latest message"
      title="Jump to latest"
    >
      <ArrowDown class="w-4 h-4" />
    </button>
  );
}
