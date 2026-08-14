import type { ComponentChildren, JSX } from "preact";

type IconComponent = (
  props: JSX.SVGAttributes<SVGSVGElement>
) => JSX.Element;

interface WorkspaceActionContentProps {
  Icon: IconComponent;
  label: string;
}

interface WorkspaceActionButtonProps extends WorkspaceActionContentProps {
  ariaLabel: string;
  emphasized?: boolean;
  onClick: () => void;
  title: string;
}

interface WorkspaceActionLinkProps extends WorkspaceActionContentProps {
  ariaLabel: string;
  href: string;
  title: string;
}

const ACTION_CLASS = `workspace-action h-8 inline-flex flex-none items-center gap-1.5 rounded-md px-2
                      text-left text-ink-300 transition hover:bg-white/[0.08] hover:text-ink-100`;

export function WorkspaceActionGroup({ children }: { children: ComponentChildren }) {
  return (
    <div class="workspace-action-group inline-flex items-center gap-0.5 rounded-lg border border-white/10 bg-white/[0.035] p-0.5">
      {children}
    </div>
  );
}

export function WorkspaceActionButton({
  Icon,
  label,
  ariaLabel,
  emphasized = false,
  onClick,
  title,
}: WorkspaceActionButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      class={`${ACTION_CLASS}${emphasized ? " bg-accent-blue/[0.08] text-ink-100" : ""}`}
      title={title}
      aria-label={ariaLabel}
    >
      <WorkspaceActionContent Icon={Icon} label={label} />
    </button>
  );
}

export function WorkspaceActionLink({
  Icon,
  label,
  ariaLabel,
  href,
  title,
}: WorkspaceActionLinkProps) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      class={ACTION_CLASS}
      title={title}
      aria-label={ariaLabel}
    >
      <WorkspaceActionContent Icon={Icon} label={label} />
    </a>
  );
}

function WorkspaceActionContent({ Icon, label }: WorkspaceActionContentProps) {
  return (
    <>
      <Icon class="w-3.5 h-3.5 text-accent-blue flex-none" />
      <span class="workspace-action-label text-[11.5px] font-medium">{label}</span>
    </>
  );
}
