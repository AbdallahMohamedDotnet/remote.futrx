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

export function createFileBrowserView(
  { search, clear, entries },
  { toggle, openFile, dismissBlocked, downloadUrl },
) {
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

  function openTitle(name) {
    return VIEWABLE.has(extension(name)) ? `View ${name}` : `Open ${name}`;
  }

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

  function fileIcon(node) {
    const { icon, tone } = categoryMeta(node.name);
    return svg(icon, `icon tone-${tone}`);
  }

  function item(node) {
    const li = element("li");
    li.dataset.path = node.path;
    return li;
  }

  function folderRow(node, state) {
    const isOpen = state.expanded.has(node.path);
    const children = state.childrenByDir.get(node.path);
    const error = state.errorByDir.get(node.path);
    const li = item(node);
    const row = element("div", "row");
    row.setAttribute("aria-expanded", String(isOpen));
    activate(row, node.name, () => toggle(node.path));
    row.append(
      state.loadingDirs.has(node.path) ? svg("loader", "spinner") : svg("chevron", "chevron"),
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
      li.append(treeList(children, true, state));
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

  function treeList(nodes, nested, state) {
    const ul = element("ul", nested ? "nested" : "");
    for (const node of nodes) ul.append(node.isDir ? folderRow(node, state) : fileRow(node));
    return ul;
  }

  // Views ----------------------------------------------------------------------

  function banner(kind, text) {
    return element("div", `banner ${kind}`, text);
  }

  function blockedNotice({ path, url }) {
    const notice = banner("warning", "Your browser blocked the new tab. ");
    const link = element("a", "", `Open ${path.slice(path.lastIndexOf("/") + 1)}`);
    link.href = url;
    link.target = "_blank";
    link.rel = "noopener";
    link.addEventListener("click", dismissBlocked);
    notice.append(link);
    return notice;
  }

  function render(state) {
    // Rebuilding the list loses focus; put it back on the same row.
    const focused = document.activeElement?.closest?.("li")?.dataset.path;
    const scroll = entries.scrollTop;
    const view = [];
    if (state.blocked) view.push(blockedNotice(state.blocked));
    if (state.searchResults !== null) {
      if (state.searchError) view.push(banner("error", state.searchError));
      if (state.searchTruncated)
        view.push(
          banner(
            "warning",
            "Showing the first matches only — refine your search to narrow it down.",
          ),
        );
      if (!state.searching && !state.searchError && state.searchResults.length === 0)
        view.push(element("div", "empty", "No matches."));
      else {
        const ul = element("ul");
        for (const node of state.searchResults) ul.append(resultRow(node));
        view.push(ul);
      }
    } else {
      if (state.rootError) view.push(banner("error", state.rootError));
      if (state.truncatedDirs.size > 0)
        view.push(
          banner(
            "warning",
            "A folder was large and its listing was truncated. Use search or download the folder as a zip to get everything.",
          ),
        );
      if (!state.rootLoading && !state.rootError && state.rootEntries.length === 0)
        view.push(element("div", "empty", "This workspace is empty."));
      else view.push(treeList(state.rootEntries, false, state));
    }
    entries.replaceChildren(...view);
    entries.scrollTop = scroll;
    if (focused)
      entries
        .querySelector(`li[data-path="${CSS.escape(focused)}"] > .row`)
        ?.focus();
    if (search.value !== state.query) search.value = state.query;
    clear.hidden = !(state.query || state.searching);
    clear.replaceChildren(svg(state.searching ? "loader" : "x", state.searching ? "spinner" : ""));
  }

  return { render };
}
