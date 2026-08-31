import { render } from "preact";
import { App } from "./app/App";
import "./index.css";
import { keyboardViewportOverride } from "./services/platform/viewportHeight";

function installViewportHeightFix() {
  let raf = 0;
  const inputFocused = () => {
    const active = document.activeElement;
    const tag = active?.tagName.toLowerCase();
    return tag === "input" || tag === "textarea" || active?.getAttribute("contenteditable") === "true";
  };

  const sync = () => {
    cancelAnimationFrame(raf);
    raf = requestAnimationFrame(() => {
      const html = document.documentElement;
      const visualViewport = window.visualViewport;
      const override = keyboardViewportOverride({
        layoutHeight: html.clientHeight,
        visual: visualViewport
          ? { height: visualViewport.height, offsetTop: visualViewport.offsetTop }
          : null,
        inputFocused: inputFocused(),
      });

      if (!override) {
        html.style.removeProperty("--app-height");
        html.style.removeProperty("--app-offset-top");
        return;
      }
      html.style.setProperty("--app-height", `${override.height}px`);
      html.style.setProperty("--app-offset-top", `${override.offsetTop}px`);
    });
  };

  sync();
  window.addEventListener("resize", sync);
  window.addEventListener("orientationchange", sync);
  window.addEventListener("focusin", sync);
  window.addEventListener("focusout", () => window.setTimeout(sync, 120));
  window.visualViewport?.addEventListener("resize", sync);
  window.visualViewport?.addEventListener("scroll", sync);
}

installViewportHeightFix();

const root = document.getElementById("root")!;
render(<App />, root);
