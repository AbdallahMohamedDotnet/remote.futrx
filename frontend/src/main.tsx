import { render } from "preact";
import { App } from "./App";
import "./index.css";
import "@xterm/xterm/css/xterm.css";

function installViewportHeightFix() {
  let raf = 0;
  const sync = () => {
    cancelAnimationFrame(raf);
    raf = requestAnimationFrame(() => {
      const height = window.visualViewport?.height ?? window.innerHeight;
      document.documentElement.style.setProperty("--app-height", `${Math.round(height)}px`);
    });
  };

  sync();
  window.addEventListener("resize", sync);
  window.addEventListener("orientationchange", sync);
  window.visualViewport?.addEventListener("resize", sync);
  window.visualViewport?.addEventListener("scroll", sync);
}

installViewportHeightFix();

const root = document.getElementById("root")!;
render(<App />, root);
