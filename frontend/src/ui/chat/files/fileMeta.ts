export type FileCategory =
  | "image"
  | "video"
  | "audio"
  | "pdf"
  | "archive"
  | "code"
  | "data"
  | "text";

const EXT_CATEGORY: Record<string, FileCategory> = {
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

export function categorize(name: string): FileCategory {
  const dot = name.lastIndexOf(".");
  if (dot < 0) return "text";
  return EXT_CATEGORY[name.slice(dot + 1).toLowerCase()] ?? "text";
}

export type MediaKind = "image" | "video" | "audio" | "pdf";

// Mirrors the backend's supported inline media types (workspacefiles
// mediaTypes): only these extensions render through the media-open endpoint.
const VIEWABLE_MEDIA: Record<string, MediaKind> = {
  avif: "image", bmp: "image", gif: "image", ico: "image", jpeg: "image",
  jpg: "image", png: "image", svg: "image", tif: "image", tiff: "image", webp: "image",
  m4v: "video", mov: "video", mp4: "video", ogv: "video", webm: "video",
  aac: "audio", flac: "audio", m4a: "audio", mp3: "audio", oga: "audio",
  ogg: "audio", opus: "audio", wav: "audio",
  pdf: "pdf",
};

// The in-app viewer kind for a filename, or null when the browser (and the
// media-open endpoint) cannot render it inline.
export function viewableMediaKind(name: string): MediaKind | null {
  const dot = name.lastIndexOf(".");
  if (dot < 0) return null;
  return VIEWABLE_MEDIA[name.slice(dot + 1).toLowerCase()] ?? null;
}

export type FileOpenAction =
  | { action: "media"; kind: MediaKind }
  | { action: "ide" }
  | { action: "download" };

// What a click on a file should do: render viewable media in the in-app
// viewer, download what neither the browser nor the IDE can display
// (archives, unsupported media), and open everything else in the IDE.
export function fileOpenAction(name: string): FileOpenAction {
  const kind = viewableMediaKind(name);
  if (kind) return { action: "media", kind };
  const category = categorize(name);
  if (category === "archive" || category === "image" || category === "video" || category === "audio") {
    return { action: "download" };
  }
  return { action: "ide" };
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(value < 10 ? 1 : 0)} ${units[i]}`;
}

/** The parent directory path of a workspace-relative path ("" = workspace root). */
export function parentDir(path: string): string {
  const slash = path.lastIndexOf("/");
  return slash < 0 ? "" : path.slice(0, slash);
}
