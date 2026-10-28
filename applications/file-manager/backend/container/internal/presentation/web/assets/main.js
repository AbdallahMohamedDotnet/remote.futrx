const search = document.querySelector("#search");
const entries = document.querySelector("#entries");
const status = document.querySelector("#status");
const refresh = document.querySelector("#refresh");
const parentOrigin = document.referrer
  ? new URL(document.referrer).origin
  : null;
let revision = 0;
let timer;
let controller;
function message(text, error = false) {
  status.textContent = text;
  status.className = error ? "error" : "";
}
async function get(route, signal) {
  const response = await fetch(route, { signal });
  if (!response.ok)
    throw new Error(`Unable to load files (${response.status}). Try Refresh.`);
  return response.json();
}
function download(node) {
  return `/api/${node.isDir ? "archive" : "download"}?path=${encodeURIComponent(node.path)}`;
}
function row(node, searching, token) {
  const li = document.createElement("li");
  const line = document.createElement("div");
  line.className = "row";
  const open = document.createElement("button");
  open.type = "button";
  open.className = "open";
  const label = (prefix = "") => {
    open.textContent = prefix + node.name;
    if (searching) {
      const path = document.createElement("small");
      path.textContent = node.path;
      open.append(path);
    }
  };
  label();
  open.title = node.path;
  const link = document.createElement("a");
  link.href = download(node);
  link.textContent = "↓";
  link.setAttribute(
    "aria-label",
    `Download ${node.name}${node.isDir ? " as ZIP" : ""}`,
  );
  link.title = link.getAttribute("aria-label");
  line.append(open);
  if (!node.isDir && node.size != null) {
    const size = document.createElement("span");
    size.className = "size";
    size.textContent = formatBytes(node.size);
    line.append(size);
  }
  line.append(link);
  li.append(line);
  if (node.isDir) {
    open.classList.add("folder");
    label("▸ ");
    open.setAttribute("aria-expanded", "false");
    const children = document.createElement("div");
    children.hidden = true;
    li.append(children);
    let loaded = false;
    open.onclick = async () => {
      children.hidden = !children.hidden;
      open.setAttribute("aria-expanded", String(!children.hidden));
      label(children.hidden ? "▸ " : "▾ ");
      if (children.hidden || loaded) return;
      children.textContent = "Loading…";
      try {
        const result = await get(
          `/api/list?path=${encodeURIComponent(node.path)}`,
          controller.signal,
        );
        if (token !== revision) return;
        children.replaceChildren(list(result.entries, false, token));
        loaded = true;
        if (result.truncated)
          children.append(
            note(
              "Folder listing truncated. Search or download its ZIP for more.",
            ),
          );
      } catch (error) {
        if (error.name !== "AbortError" && token === revision)
          children.replaceChildren(note(error.message, true));
      }
    };
  } else {
    open.onclick = () => {
      if (parentOrigin && window.parent !== window)
        window.parent.postMessage(
          { source: "remote-file-manager", type: "open", path: node.path },
          parentOrigin,
        );
      else window.location.assign(download(node));
    };
  }
  return li;
}
function formatBytes(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index++;
  }
  return `${value.toFixed(value < 10 ? 1 : 0)} ${units[index]}`;
}
function note(text, error = false) {
  const p = document.createElement("p");
  p.className = error ? "error" : "message";
  p.textContent = text;
  return p;
}
function list(nodes, searching, token) {
  const ul = document.createElement("ul");
  for (const node of nodes || []) ul.append(row(node, searching, token));
  if (!nodes?.length)
    ul.append(note(searching ? "No matches." : "Empty folder."));
  return ul;
}
async function load() {
  const token = ++revision;
  controller?.abort();
  controller = new AbortController();
  const query = search.value.trim();
  const searching = Array.from(query).length >= 2;
  message("Loading…");
  refresh.disabled = true;
  try {
    const result = await get(
      searching ? `/api/search?q=${encodeURIComponent(query)}` : "/api/list",
      controller.signal,
    );
    if (token !== revision) return;
    entries.replaceChildren(list(result.entries, searching, token));
    message(
      result.truncated
        ? "Results truncated. Refine your search or download a folder as ZIP."
        : searching
          ? `${result.entries.length} result${result.entries.length === 1 ? "" : "s"}`
          : "Workspace",
    );
  } catch (error) {
    if (error.name !== "AbortError" && token === revision) {
      entries.replaceChildren();
      message(error.message, true);
    }
  } finally {
    if (token === revision) refresh.disabled = false;
  }
}
search.oninput = () => {
  clearTimeout(timer);
  revision++;
  controller?.abort();
  timer = setTimeout(load, 250);
};
refresh.onclick = () => {
  clearTimeout(timer);
  void load();
};
window.addEventListener("message", (event) => {
  if (
    event.source !== window.parent ||
    event.origin !== parentOrigin ||
    event.data?.source !== "remote-file-manager-theme"
  )
    return;
  for (const [name, value] of Object.entries(event.data.tokens || {}))
    if (/^--[a-z-]+$/.test(name) && typeof value === "string")
      document.documentElement.style.setProperty(name, value);
});
void load();
