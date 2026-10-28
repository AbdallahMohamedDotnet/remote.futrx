// Opening a clicked file on Remote versions without `remote.files.open`.
//
// Those versions give applications no way to reach the editors other
// applications register, so this reproduces what Remote's former built-in
// Files drawer did from Remote's page:
// - Scratch Tab opens files by intercepting clicks on links to them, so the
//   file is offered as a link click and Scratch Tab may claim it;
// - otherwise Code Server, when it runs in the project, opens in a new tab;
// - media opens in the browser and everything else downloads.
// Remote versions with `remote.files.open` do all of this in core instead.

import { applicationOrigin } from "./workspacePath.js";

const CODE_SERVER = { id: "code-server", subdomain: "code" };

const VIEWABLE = new Set([
  "avif", "bmp", "gif", "ico", "jpeg", "jpg", "png", "svg", "tif", "tiff", "webp",
  "m4v", "mov", "mp4", "ogv", "webm",
  "aac", "flac", "m4a", "mp3", "oga", "ogg", "opus", "wav",
  "pdf",
]);
// Archives and media the browser cannot play: no editor opens these.
const DOWNLOADED = new Set([
  "zip", "tar", "gz", "tgz", "rar", "7z",
  "heic", "mkv", "avi",
]);

function extension(path) {
  const name = path.slice(path.lastIndexOf("/") + 1);
  const dot = name.lastIndexOf(".");
  return dot < 0 ? "" : name.slice(dot + 1).toLowerCase();
}

/** "media", "editor" or "download", as Remote decides for a file click. */
export function fileAction(path) {
  const ext = extension(path);
  if (VIEWABLE.has(ext)) return "media";
  return DOWNLOADED.has(ext) ? "download" : "editor";
}

/** Code Server's address for a file, as its own opener builds it. */
export function codeServerFileUrl(cwd, path, origin = window.location.origin) {
  const url = applicationOrigin(cwd, CODE_SERVER.subdomain, origin);
  const location = /\/workspace(?:\/(.*))?$/.exec(cwd || "");
  if (!url || !location) return null;
  url.pathname = "/_static/remote-open.html";
  url.searchParams.set(
    "folder",
    location[1] ? `/workspace/${location[1]}` : "/workspace",
  );
  url.searchParams.set("file", path);
  return url.toString();
}

/** The link Scratch Tab recognises as "open this file in Scratch Tab". */
export function scratchFileUrl(chatId, path) {
  return `/api/chats/${encodeURIComponent(chatId)}/ide-open?scratch=1&path=${encodeURIComponent(path)}`;
}

// Offer a URL to Remote's page as a link click. An editor that intercepts
// such links claims it by stopping the event before it reaches the link.
export function claimedAsLink(url, doc = document) {
  const link = doc.createElement("a");
  link.href = url;
  link.hidden = true;
  let unclaimed = false;
  link.addEventListener("click", (event) => {
    unclaimed = true;
    event.preventDefault();
  });
  doc.body.append(link);
  try {
    link.dispatchEvent(
      new MouseEvent("click", { bubbles: true, cancelable: true, button: 0 }),
    );
  } finally {
    link.remove();
  }
  return !unclaimed;
}

/** Whether Code Server runs in the project, from Remote's own API. */
export async function codeServerRunning(projectId, fetcher = fetch) {
  if (!projectId) return false;
  try {
    const response = await fetcher(
      `/api/projects/${encodeURIComponent(projectId)}/applications`,
      { credentials: "same-origin" },
    );
    if (!response.ok) return false;
    const instances = await response.json();
    return (
      Array.isArray(instances) &&
      instances.some(
        (instance) =>
          instance?.applicationId === CODE_SERVER.id &&
          instance.status === "running",
      )
    );
  } catch {
    return false;
  }
}

function openTab(url) {
  const tab = window.open("", "_blank");
  if (!tab) return false;
  tab.opener = null;
  tab.location.replace(url);
  return true;
}

/**
 * Open a workspace file for a chat. Returns "opened", "download", or
 * { blocked: url } when the browser refused the new tab.
 */
export function openFileWithoutCore({ chatId, cwd, path }, codeServer) {
  const relative = path.slice("/workspace/".length);
  const action = fileAction(path);
  if (action === "media") {
    const url = `/api/chats/${encodeURIComponent(chatId)}/media-open?path=${encodeURIComponent(path)}`;
    return openTab(url) ? "opened" : { blocked: new URL(url, window.location.origin).href };
  }
  if (action === "editor") {
    // A Code Server link also lets Scratch Tab offer its "Open in IDE" button.
    const ide = codeServer ? codeServerFileUrl(cwd, path) : null;
    if (claimedAsLink(ide ?? scratchFileUrl(chatId, path))) return "opened";
    if (ide) return openTab(ide) ? "opened" : { blocked: ide };
  }
  window.location.assign(
    `/api/chats/${encodeURIComponent(chatId)}/files/download?path=${encodeURIComponent(relative)}`,
  );
  return "download";
}
