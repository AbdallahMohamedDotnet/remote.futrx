// The Files drawer body. Remote draws the header (title, subtitle, Refresh,
// close); this page draws the search bar and the tree beneath it, and keeps
// the header in sync through messages to the drawer script.
const search = document.querySelector("#search");
const clear = document.querySelector("#clear");
const entries = document.querySelector("#entries");
const parentOrigin = document.referrer
  ? new URL(document.referrer).origin
  : null;
const embedded = Boolean(parentOrigin) && window.parent !== window;
const SEARCH_DEBOUNCE_MS = 250;

const ICONS = {
  chevron: '<path d="m9 6 6 6-6 6"/>',
  loader:
    '<path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"/>',
  x: '<path d="M18 6 6 18M6 6l12 12"/>',
  folder:
    '<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>',
  folderOpen:
    '<path d="m6 14 1.45-2.9A2 2 0 0 1 9.24 10H21a2 2 0 0 1 1.94 2.5l-1.55 6A2 2 0 0 1 19.5 20H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.93a2 2 0 0 1 1.66.9l.82 1.2a2 2 0 0 0 1.66.9H18a2 2 0 0 1 2 2v2"/>',
  download:
    '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>',
  image:
    '<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.1-3.1a2 2 0 0 0-2.8 0L6 21"/>',
  film: '<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 3v18M17 3v18M3 7.5h4M3 12h18M3 16.5h4M17 7.5h4M17 16.5h4"/>',
  music:
    '<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>',
  archive:
    '<rect x="2" y="4" width="20" height="5" rx="1"/><path d="M4 9v9a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9M10 13h4"/>',
  code: '<path d="m16 18 6-6-6-6M8 6l-6 6 6 6"/>',
  fileText:
    '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6M9 13h6M9 17h6"/>',
};

const CATEGORY_META = {
  image: { icon: "image", tone: "green" },
  video: { icon: "film", tone: "purple" },
  audio: { icon: "music", tone: "orange" },
  pdf: { icon: "fileText", tone: "red" },
  archive: { icon: "archive", tone: "yellow" },
  code: { icon: "code", tone: "blue" },
  data: { icon: "fileText", tone: "green" },
  text: { icon: "fileText", tone: "muted" },
};

const CATEGORY_BY_EXTENSION = {
  png: "image", jpg: "image", jpeg: "image", gif: "image", webp: "image",
  svg: "image", avif: "image", bmp: "image", ico: "image", heic: "image",
  mp4: "video", mov: "video", webm: "video", mkv: "video", avi: "video", m4v: "video",
  mp3: "audio", wav: "audio", flac: "audio", ogg: "audio", m4a: "audio", aac: "audio",
  pdf: "pdf",
  zip: "archive", tar: "archive", gz: "archive", tgz: "archive", rar: "archive", "7z": "archive",
  ts: "code", tsx: "code", js: "code", jsx: "code", go: "code", py: "code", rs: "code",
  java: "code", c: "code", cpp: "code", h: "code", css: "code", html: "code", sh: "code", rb: "code",
  json: "data", csv: "data", yaml: "data", yml: "data", xml: "data", toml: "data",
  sql: "data", db: "data", sqlite: "data",
  txt: "text", md: "text", log: "text",
};

// Extensions Remote's media viewer renders inline.
const VIEWABLE = new Set([
  "avif", "bmp", "gif", "ico", "jpeg", "jpg", "png", "svg", "tif", "tiff", "webp",
  "m4v", "mov", "mp4", "ogv", "webm",
  "aac", "flac", "m4a", "mp3", "oga", "ogg", "opus", "wav",
  "pdf",
]);

// Tree state: survives searching, and is thrown away by Refresh.
let revision = 0;
let controller = new AbortController();
let rootLoading = false;
let rootEntries = [];
let rootError = "";
const expanded = new Set();
const loadingDirs = new Set();
const childrenByDir = new Map();
const errorByDir = new Map();
const truncatedDirs = new Set();

// A file whose editor tab the browser refused, offered as a link instead.
let blocked = null;

// Search state.
let searchTimer;
let searchController;
let searchResults = null;
let searchTruncated = false;
let searching = false;
let searchError = "";

function svg(name, className) {
  const element = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  element.setAttribute("viewBox", "0 0 24 24");
  element.setAttribute("aria-hidden", "true");
  element.setAttribute("class", className);
  element.innerHTML = ICONS[name];
  return element;
}

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function extension(name) {
  const dot = name.lastIndexOf(".");
  return dot < 0 ? "" : name.slice(dot + 1).toLowerCase();
}

function categoryMeta(name) {
  return CATEGORY_META[CATEGORY_BY_EXTENSION[extension(name)] ?? "text"];
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

function parentDir(path) {
  const slash = path.lastIndexOf("/");
  return slash < 0 ? "" : path.slice(0, slash);
}

function downloadUrl(node) {
  return `/api/${node.isDir ? "archive" : "download"}?path=${encodeURIComponent(node.path)}`;
}

function openTitle(name) {
  return VIEWABLE.has(extension(name)) ? `View ${name}` : `Open ${name}`;
}

async function get(route, signal) {
  const response = await fetch(route, { signal });
  if (!response.ok)
    throw new Error(`Unable to load files (${response.status}). Try Refresh.`);
  return response.json();
}

// Header ---------------------------------------------------------------------

function subtitle() {
  if (searchResults) {
    const count = searchResults.length;
    return `${count} result${count === 1 ? "" : "s"}${searchTruncated ? "+" : ""}`;
  }
  return rootLoading ? "Loading..." : "workspace";
}

function publishState() {
  if (!embedded) return;
  window.parent.postMessage(
    {
      source: "remote-file-manager",
      type: "state",
      subtitle: subtitle(),
      loading: rootLoading,
    },
    parentOrigin,
  );
}

// Rows -----------------------------------------------------------------------

function activate(row, name, action) {
  row.setAttribute("role", "button");
  row.setAttribute("aria-label", name);
  row.tabIndex = 0;
  row.addEventListener("click", action);
  row.addEventListener("keydown", (event) => {
    if (event.target !== row) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      action();
    }
  });
}

function downloadLink(node) {
  const link = element("a", "download");
  link.href = downloadUrl(node);
  if (!node.isDir) link.download = node.name;
  const label = node.isDir
    ? `Download ${node.name} as zip`
    : `Download ${node.name}`;
  link.title = label;
  link.setAttribute("aria-label", label);
  link.addEventListener("click", (event) => event.stopPropagation());
  link.append(svg("download"));
  return link;
}

function openFile(node) {
  if (!embedded) {
    window.location.assign(downloadUrl(node));
    return;
  }
  if (blocked) {
    blocked = null;
    render();
  }
  window.parent.postMessage(
    { source: "remote-file-manager", type: "open", path: node.path },
    parentOrigin,
  );
}

function fileIcon(node) {
  const { icon, tone } = categoryMeta(node.name);
  return svg(icon, `icon tone-${tone}`);
}

function item(node) {
  const li = element("li");
  li.dataset.path = node.path;
  return li;
}

function folderRow(node) {
  const isOpen = expanded.has(node.path);
  const children = childrenByDir.get(node.path);
  const error = errorByDir.get(node.path);
  const li = item(node);
  const row = element("div", "row");
  row.setAttribute("aria-expanded", String(isOpen));
  activate(row, node.name, () => toggle(node.path));
  row.append(
    loadingDirs.has(node.path) ? svg("loader", "spinner") : svg("chevron", "chevron"),
    svg(isOpen ? "folderOpen" : "folder", "icon tone-blue"),
    element("span", "name", node.name),
  );
  if (children) row.append(element("span", "meta", String(children.length)));
  row.append(downloadLink(node));
  li.append(row);
  if (isOpen && error) li.append(element("div", "inline-note error", error));
  if (isOpen && children && children.length === 0 && !error)
    li.append(element("div", "inline-note", "Empty folder."));
  if (isOpen && children && children.length > 0)
    li.append(treeList(children, true));
  return li;
}

function fileRow(node) {
  const li = item(node);
  const row = element("div", "row");
  row.title = openTitle(node.name);
  activate(row, node.name, () => openFile(node));
  row.append(
    element("span", "indent"),
    fileIcon(node),
    element("span", "name", node.name),
  );
  if (node.size != null)
    row.append(element("span", "meta", formatBytes(node.size)));
  row.append(downloadLink(node));
  li.append(row);
  return li;
}

function resultRow(node) {
  const li = item(node);
  const row = element("div", "row");
  if (!node.isDir) {
    row.title = openTitle(node.name);
    activate(row, node.name, () => openFile(node));
  }
  const detail = element("div", "detail");
  detail.append(element("span", "name", node.name));
  const dir = parentDir(node.path);
  if (dir) detail.append(element("div", "dir", `${dir}/`));
  row.append(node.isDir ? svg("folder", "icon tone-blue") : fileIcon(node), detail);
  if (!node.isDir && node.size != null)
    row.append(element("span", "meta", formatBytes(node.size)));
  row.append(downloadLink(node));
  li.append(row);
  return li;
}

function treeList(nodes, nested) {
  const ul = element("ul", nested ? "nested" : "");
  for (const node of nodes) ul.append(node.isDir ? folderRow(node) : fileRow(node));
  return ul;
}

// Views ----------------------------------------------------------------------

function banner(kind, text) {
  return element("div", `banner ${kind}`, text);
}

function blockedNotice({ path, url }) {
  const notice = banner("warning", "Your browser blocked the editor tab. ");
  const link = element("a", "", `Open ${path.slice(path.lastIndexOf("/") + 1)}`);
  link.href = url;
  link.target = "_blank";
  link.rel = "noopener";
  link.addEventListener("click", () => {
    blocked = null;
    queueMicrotask(render);
  });
  notice.append(link);
  return notice;
}

function render() {
  // Rebuilding the list loses focus; put it back on the same row.
  const focused = document.activeElement?.closest?.("li")?.dataset.path;
  const scroll = entries.scrollTop;
  const view = [];
  if (blocked) view.push(blockedNotice(blocked));
  if (searchResults !== null) {
    if (searchError) view.push(banner("error", searchError));
    if (searchTruncated)
      view.push(
        banner(
          "warning",
          "Showing the first matches only — refine your search to narrow it down.",
        ),
      );
    if (!searching && !searchError && searchResults.length === 0)
      view.push(element("div", "empty", "No matches."));
    else {
      const ul = element("ul");
      for (const node of searchResults) ul.append(resultRow(node));
      view.push(ul);
    }
  } else {
    if (rootError) view.push(banner("error", rootError));
    if (truncatedDirs.size > 0)
      view.push(
        banner(
          "warning",
          "A folder was large and its listing was truncated. Use search or download the folder as a zip to get everything.",
        ),
      );
    if (!rootLoading && !rootError && rootEntries.length === 0)
      view.push(element("div", "empty", "This workspace is empty."));
    else view.push(treeList(rootEntries, false));
  }
  entries.replaceChildren(...view);
  entries.scrollTop = scroll;
  if (focused)
    entries
      .querySelector(`li[data-path="${CSS.escape(focused)}"] > .row`)
      ?.focus();
  clear.hidden = !(search.value || searching);
  clear.replaceChildren(svg(searching ? "loader" : "x", searching ? "spinner" : ""));
  publishState();
}

// Loading --------------------------------------------------------------------

async function loadDirectory(path, token) {
  loadingDirs.add(path);
  if (path === "") rootLoading = true;
  render();
  try {
    const result = await get(
      path ? `/api/list?path=${encodeURIComponent(path)}` : "/api/list",
      controller.signal,
    );
    if (token !== revision) return;
    const list = result.entries || [];
    if (path === "") rootEntries = list;
    childrenByDir.set(path, list);
    errorByDir.delete(path);
    if (path === "") rootError = "";
    if (result.truncated) truncatedDirs.add(path);
    else truncatedDirs.delete(path);
  } catch (error) {
    if (error.name === "AbortError" || token !== revision) return;
    errorByDir.set(path, error.message);
    if (path === "") rootError = error.message;
  } finally {
    if (token === revision) {
      loadingDirs.delete(path);
      if (path === "") rootLoading = false;
      render();
    }
  }
}

function toggle(path) {
  const opening = !expanded.has(path);
  if (opening) expanded.add(path);
  else expanded.delete(path);
  if (opening && !childrenByDir.has(path) && !loadingDirs.has(path))
    void loadDirectory(path, revision);
  else render();
}

function reset() {
  controller.abort();
  controller = new AbortController();
  const token = ++revision;
  rootEntries = [];
  rootError = "";
  expanded.clear();
  loadingDirs.clear();
  childrenByDir.clear();
  errorByDir.clear();
  truncatedDirs.clear();
  search.value = "";
  runSearch();
  void loadDirectory("", token);
}

function runSearch() {
  clearTimeout(searchTimer);
  searchController?.abort();
  const query = search.value.trim();
  if (query.length < 2) {
    searchResults = null;
    searchError = "";
    searching = false;
    render();
    return;
  }
  searching = true;
  render();
  searchController = new AbortController();
  const { signal } = searchController;
  searchTimer = setTimeout(async () => {
    try {
      const result = await get(`/api/search?q=${encodeURIComponent(query)}`, signal);
      searchResults = result.entries || [];
      searchTruncated = Boolean(result.truncated);
      searchError = "";
    } catch (error) {
      if (error.name === "AbortError") return;
      searchResults = [];
      searchError = error.message;
    }
    searching = false;
    render();
  }, SEARCH_DEBOUNCE_MS);
}

search.addEventListener("input", runSearch);
clear.addEventListener("click", () => {
  search.value = "";
  runSearch();
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
    reset();
    return;
  }
  if (
    event.data.type === "blocked" &&
    typeof event.data.path === "string" &&
    typeof event.data.url === "string" &&
    /^https?:\/\//i.test(event.data.url)
  ) {
    blocked = { path: event.data.path, url: event.data.url };
    render();
    return;
  }
  if (event.data.type === "download" && typeof event.data.path === "string") {
    window.location.assign(downloadUrl({ path: event.data.path, isDir: false }));
    return;
  }
  if (event.data.type !== "theme") return;
  for (const [name, value] of Object.entries(event.data.tokens || {}))
    if (/^--[a-z-]+$/.test(name) && typeof value === "string" && value)
      document.documentElement.style.setProperty(name, value);
  document.documentElement.classList.toggle("compact", Boolean(event.data.compact));
});

reset();
