import { Loader } from "../../primitives/icons";

export function ThinkingIndicator() {
  return (
    <div class="inline-flex items-center gap-2 text-ink-300 text-xs pt-1 rounded-full bg-white/5 px-2.5 py-1">
      <Loader class="w-3 h-3 animate-spin" />
      thinking
    </div>
  );
}
