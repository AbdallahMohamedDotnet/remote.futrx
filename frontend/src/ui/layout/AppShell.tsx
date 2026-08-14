import type { ComponentChildren } from "preact";

export function AppShell({
  sidebar,
  children,
}: {
  sidebar: ComponentChildren;
  children: ComponentChildren;
}) {
  return (
    <div class="codex-app app-shell relative flex bg-[#090b0f] text-ink-100 overflow-hidden">
      {sidebar}
      <main class="codex-main codex-window-frame relative flex-1 flex flex-col min-w-0 h-full overflow-hidden bg-[#0b0d11]">
        {children}
      </main>
    </div>
  );
}
