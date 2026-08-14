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
      const keyboardOpen = keyboardLikelyOpen();
      const visualViewport = window.visualViewport;
      const height = keyboardOpen && visualViewport?.height ? visualViewport.height : window.innerHeight;
      const offsetTop = keyboardOpen ? visualViewport?.offsetTop ?? 0 : 0;
      document.documentElement.style.setProperty("--app-height", `${Math.round(height)}px`);
      document.documentElement.style.setProperty("--app-offset-top", `${Math.round(offsetTop)}px`);
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
