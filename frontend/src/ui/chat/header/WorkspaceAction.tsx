import type { JSX } from "preact";

type IconComponent = (
  props: JSX.SVGAttributes<SVGSVGElement>
) => JSX.Element;

interface WorkspaceActionBaseProps {
  Icon: IconComponent;
  ariaLabel: string;
  emphasized?: boolean;
  label: string;
  title: string;
}

type WorkspaceActionProps = WorkspaceActionBaseProps & (
  | {
      href: string;
      onClick?: never;
    }
  | {
      href?: never;
      onClick: () => void;
    }
);

const ACTION_CLASS = `workspace-action h-8 inline-flex flex-none items-center gap-1.5 rounded-md px-2
                      text-left text-ink-300 transition hover:bg-white/[0.08] hover:text-ink-100`;

export function WorkspaceAction({
  Icon,
  ariaLabel,
  emphasized = false,
  href,
  label,
  onClick,
  title,
}: WorkspaceActionProps) {
  const content = (
    <>
      <Icon class="w-3.5 h-3.5 text-accent-blue flex-none" />
      <span class="workspace-action-label text-[11.5px] font-medium">{label}</span>
    </>
  );

  if (href) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        class={ACTION_CLASS}
        title={title}
        aria-label={ariaLabel}
      >
        {content}
      </a>
    );
  }

  return (
    <button
      type="button"
      onClick={onClick}
      class={`${ACTION_CLASS}${emphasized ? " bg-accent-blue/[0.08] text-ink-100" : ""}`}
      title={title}
      aria-label={ariaLabel}
    >
      {content}
    </button>
  );
}
