import { render } from "preact";
import { App } from "./app/App";
import "./index.css";

function installViewportHeightFix() {
  let raf = 0;
  const keyboardLikelyOpen = () => {
    const active = document.activeElement;
    const tag = active?.tagName.toLowerCase();
    return tag === "input" || tag === "textarea" || active?.getAttribute("contenteditable") === "true";
  };

  const sync = () => {
    cancelAnimationFrame(raf);
    raf = requestAnimationFrame(() => {
      const visualHeight = window.visualViewport?.height;
      const height = keyboardLikelyOpen() && visualHeight ? visualHeight : window.innerHeight;
      document.documentElement.style.setProperty("--app-height", `${Math.round(height)}px`);
    });
  };

  sync();
  window.addEventListener("resize", sync);
  window.addEventListener("orientationchange", sync);
  window.addEventListener("focusin", sync);
  window.addEventListener("focusout", () => window.setTimeout(sync, 120));
  window.visualViewport?.addEventListener("resize", sync);
}

installViewportHeightFix();

const root = document.getElementById("root")!;
render(<App />, root);
