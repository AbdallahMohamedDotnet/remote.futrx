// Remote's theme tokens the page needs to look like the rest of the app.
const TOKENS = [
  "--fg-bright",
  "--fg",
  "--fg-dim",
  "--fg-muted",
  "--fg-subtle",
  "--fg-faint",
  "--bg-surface-rgb",
  "--bg-inset-rgb",
  "--line",
  "--tint",
  "--tint-strong",
  "--accent-blue",
  "--accent-green",
  "--accent-red",
  "--accent-yellow",
  "--accent-orange",
  "--accent-purple",
  "--scrollbar-thumb",
  "--scrollbar-thumb-hover",
  "--selection-bg",
];
// Remote pads its panes less below its `md` breakpoint.
const COMPACT = "(max-width: 767px)";

export function syncDrawerTheme(frame, send) {
  const compact = window.matchMedia(COMPACT);
  const theme = () => {
    const styles = getComputedStyle(document.documentElement);
    send({
      type: "theme",
      tokens: Object.fromEntries(
        TOKENS.map((name) => [name, styles.getPropertyValue(name).trim()]),
      ),
      compact: compact.matches,
    });
  };
  frame.addEventListener("load", theme);
  compact.addEventListener("change", theme);
  const observer = new MutationObserver(theme);
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class", "data-theme", "style"],
  });
  return () => {
    observer.disconnect();
    compact.removeEventListener("change", theme);
    frame.removeEventListener("load", theme);
  };
}
