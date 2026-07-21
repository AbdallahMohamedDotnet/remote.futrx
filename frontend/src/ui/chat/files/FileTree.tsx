import type { JSX } from "preact";
import type { FileNode } from "../../../models/files";
import { chatApi } from "../../../api/chatApi";
import {
  Archive,
  ChevronRight,
  Code,
  Download,
  FileText,
  Film,
  Folder,
  FolderOpen,
  Image,
  Loader,
  Music,
} from "../../primitives/icons";
import { categorize, formatBytes, parentDir, type FileCategory } from "./fileMeta";

type IconComponent = (props: JSX.SVGAttributes<SVGSVGElement>) => JSX.Element;

const CATEGORY_META: Record<FileCategory, { Icon: IconComponent; color: string }> = {
  image: { Icon: Image, color: "text-[#34d399]" },
  video: { Icon: Film, color: "text-[#a78bfa]" },
  audio: { Icon: Music, color: "text-[#f472b6]" },
  pdf: { Icon: FileText, color: "text-[#f87171]" },
  archive: { Icon: Archive, color: "text-[#fbbf24]" },
  code: { Icon: Code, color: "text-[#38bdf8]" },
  data: { Icon: FileText, color: "text-[#2dd4bf]" },
  text: { Icon: FileText, color: "text-ink-300" },
};

/** Shared state + callbacks threaded through the lazily-loaded tree. */
export interface TreeState {
  chatId: string;
  expanded: Set<string>;
  loading: Set<string>;
  childrenByDir: Map<string, FileNode[]>;
  errorByDir: Map<string, string>;
  onToggle: (path: string) => void;
}

export function FileTreeNodes({
  nodes,
  depth,
  state,
}: {
  nodes: FileNode[];
  depth: number;
  state: TreeState;
}) {
  return (
    <ul class={depth > 0 ? "ml-3 border-l border-white/[0.07] pl-1" : ""}>
      {nodes.map((node) =>
        node.isDir ? (
          <FolderRow key={node.path} node={node} depth={depth} state={state} />
        ) : (
          <FileRow key={node.path} chatId={state.chatId} node={node} />
        )
      )}
    </ul>
  );
}

function FolderRow({
  node,
  depth,
  state,
}: {
  node: FileNode;
  depth: number;
  state: TreeState;
}) {
  const isOpen = state.expanded.has(node.path);
  const isLoading = state.loading.has(node.path);
  const children = state.childrenByDir.get(node.path);
  const error = state.errorByDir.get(node.path);

  return (
    <li>
      <div
        class="group flex items-center gap-1.5 rounded px-1.5 py-1 hover:bg-white/[0.05] cursor-pointer select-none"
        role="button"
        tabIndex={0}
        onClick={() => state.onToggle(node.path)}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            state.onToggle(node.path);
          }
        }}
      >
        {isLoading ? (
          <Loader class="w-3.5 h-3.5 flex-none text-ink-400 animate-spin" />
        ) : (
          <ChevronRight
            class={`w-3.5 h-3.5 flex-none text-ink-400 transition-transform ${isOpen ? "rotate-90" : ""}`}
          />
        )}
        {isOpen ? (
          <FolderOpen class="w-4 h-4 flex-none text-accent-blue" />
        ) : (
          <Folder class="w-4 h-4 flex-none text-accent-blue" />
        )}
        <span class="flex-1 min-w-0 truncate text-[13px] text-ink-100">{node.name}</span>
        {children && (
          <span class="text-[11px] text-ink-500 tabular-nums flex-none">{children.length}</span>
        )}
        <a
          href={chatApi.folderDownloadUrl(state.chatId, node.path)}
          onClick={(event) => event.stopPropagation()}
          class="h-6 w-6 grid place-items-center rounded text-ink-400 hover:text-accent-blue hover:bg-white/[0.08]
                 opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity flex-none"
          title={`Download ${node.name} as zip`}
          aria-label={`Download ${node.name} as zip`}
        >
          <Download class="w-3.5 h-3.5" />
        </a>
      </div>
      {isOpen && error && (
        <div class="ml-3 pl-2 py-1 text-[12px] text-accent-red">{error}</div>
      )}
      {isOpen && children && children.length === 0 && !error && (
        <div class="ml-3 pl-2 py-1 text-[12px] text-ink-500">Empty folder.</div>
      )}
      {isOpen && children && children.length > 0 && (
        <FileTreeNodes nodes={children} depth={depth + 1} state={state} />
      )}
    </li>
  );
}

function FileRow({ chatId, node }: { chatId: string; node: FileNode }) {
  const { Icon, color } = CATEGORY_META[categorize(node.name)];
  return (
    <li>
      <div class="group flex items-center gap-1.5 rounded px-1.5 py-1 hover:bg-white/[0.05]">
        <span class="w-3.5 flex-none" aria-hidden="true" />
        <Icon class={`w-4 h-4 flex-none ${color}`} />
        <span class="flex-1 min-w-0 truncate text-[13px] text-ink-100">{node.name}</span>
        {node.size != null && (
          <span class="text-[11px] text-ink-500 tabular-nums flex-none">{formatBytes(node.size)}</span>
        )}
        <a
          href={chatApi.fileDownloadUrl(chatId, node.path)}
          download={node.name}
          class="h-6 w-6 grid place-items-center rounded text-ink-400 hover:text-accent-blue hover:bg-white/[0.08]
                 opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity flex-none"
          title={`Download ${node.name}`}
          aria-label={`Download ${node.name}`}
        >
          <Download class="w-3.5 h-3.5" />
        </a>
      </div>
    </li>
  );
}

/** Flat row used to render server-side search results, showing the full path. */
export function SearchResultRow({ chatId, node }: { chatId: string; node: FileNode }) {
  const dir = parentDir(node.path);
  const { Icon, color } = node.isDir
    ? { Icon: Folder as IconComponent, color: "text-accent-blue" }
    : CATEGORY_META[categorize(node.name)];
  const href = node.isDir
    ? chatApi.folderDownloadUrl(chatId, node.path)
    : chatApi.fileDownloadUrl(chatId, node.path);
  return (
    <li>
      <div class="group flex items-center gap-1.5 rounded px-1.5 py-1 hover:bg-white/[0.05]">
        <Icon class={`w-4 h-4 flex-none ${color}`} />
        <div class="flex-1 min-w-0">
          <div class="truncate text-[13px] text-ink-100">{node.name}</div>
          {dir && <div class="truncate text-[11px] text-ink-500 font-mono">{dir}/</div>}
        </div>
        {!node.isDir && node.size != null && (
          <span class="text-[11px] text-ink-500 tabular-nums flex-none">{formatBytes(node.size)}</span>
        )}
        <a
          href={href}
          download={node.isDir ? undefined : node.name}
          class="h-6 w-6 grid place-items-center rounded text-ink-400 hover:text-accent-blue hover:bg-white/[0.08]
                 opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity flex-none"
          title={node.isDir ? `Download ${node.name} as zip` : `Download ${node.name}`}
          aria-label={node.isDir ? `Download ${node.name} as zip` : `Download ${node.name}`}
        >
          <Download class="w-3.5 h-3.5" />
        </a>
      </div>
    </li>
  );
}
