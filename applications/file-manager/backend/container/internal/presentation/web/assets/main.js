// Remote owns the header; this entrypoint connects the page to its drawer.
import { filesApi, downloadUrl } from "./filesApi.js";
import { createFileBrowser } from "./fileBrowser.js";
import { createFileBrowserView } from "./fileBrowserView.js";

const search = document.querySelector("#search");
const clear = document.querySelector("#clear");
const entries = document.querySelector("#entries");
const parentOrigin = document.referrer
  ? new URL(document.referrer).origin
  : null;
const embedded = Boolean(parentOrigin) && window.parent !== window;

const browser = createFileBrowser(filesApi, render);
const view = createFileBrowserView(
  { search, clear, entries },
  {
    toggle: browser.toggle,
    openFile,
    dismissBlocked: browser.dismissBlocked,
    downloadUrl,
  },
);

function render() {
  view.render(browser.state);
  publishState();
}

function publishState() {
  if (!embedded) return;
  window.parent.postMessage(
    {
      source: "remote-file-manager",
      type: "state",
      subtitle: browser.subtitle(),
      loading: browser.state.rootLoading,
    },
    parentOrigin,
  );
}

function openFile(node) {
  if (!embedded) {
    window.location.assign(downloadUrl(node));
    return;
  }
  browser.clearBlocked();
  window.parent.postMessage(
    { source: "remote-file-manager", type: "open", path: node.path },
    parentOrigin,
  );
}

search.addEventListener("input", () => browser.search(search.value));
clear.addEventListener("click", () => {
  search.value = "";
  browser.search(search.value);
  search.focus();
});

window.addEventListener("message", (event) => {
  if (
    !embedded ||
    event.source !== window.parent ||
    event.origin !== parentOrigin ||
    event.data?.source !== "remote-file-manager-host"
  )
    return;
  if (event.data.type === "refresh") {
    browser.reset();
    return;
  }
  if (
    event.data.type === "blocked" &&
    typeof event.data.path === "string" &&
    typeof event.data.url === "string" &&
    /^https?:\/\//i.test(event.data.url)
  ) {
    browser.showBlocked(event.data.path, event.data.url);
    return;
  }
  if (event.data.type !== "theme") return;
  for (const [name, value] of Object.entries(event.data.tokens || {}))
    if (/^--[a-z-]+$/.test(name) && typeof value === "string" && value)
      document.documentElement.style.setProperty(name, value);
  document.documentElement.classList.toggle("compact", Boolean(event.data.compact));
});

browser.reset();
