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
  Music,
} from "../../primitives/icons";
import { categorize, countFiles, formatBytes, nodeKey, type FileCategory } from "./fileMeta";

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

export function FileTreeNodes({
  chatId,
  dir,
  nodes,
  depth,
  expanded,
  onToggle,
}: {
  chatId: string;
  dir: string;
  nodes: FileNode[];
  depth: number;
  expanded: Set<string>;
  onToggle: (key: string) => void;
}) {
  return (
    <ul class={depth > 0 ? "ml-3 border-l border-white/[0.07] pl-1" : ""}>
      {nodes.map((node) =>
        node.isDir ? (
          <FolderRow
            key={node.path}
            chatId={chatId}
            dir={dir}
            node={node}
            depth={depth}
            expanded={expanded}
            onToggle={onToggle}
          />
        ) : (
          <FileRow key={node.path} chatId={chatId} dir={dir} node={node} />
        )
      )}
    </ul>
  );
}

function FolderRow({
  chatId,
  dir,
  node,
  depth,
  expanded,
  onToggle,
}: {
  chatId: string;
  dir: string;
  node: FileNode;
  depth: number;
  expanded: Set<string>;
  onToggle: (key: string) => void;
}) {
  const key = nodeKey(dir, node.path);
  const isOpen = expanded.has(key);
  const fileCount = countFiles(node.children);

  return (
    <li>
      <div
        class="group flex items-center gap-1.5 rounded px-1.5 py-1 hover:bg-white/[0.05] cursor-pointer select-none"
        role="button"
        tabIndex={0}
        onClick={() => onToggle(key)}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            onToggle(key);
          }
        }}
      >
        <ChevronRight
          class={`w-3.5 h-3.5 flex-none text-ink-400 transition-transform ${isOpen ? "rotate-90" : ""}`}
        />
        {isOpen ? (
          <FolderOpen class="w-4 h-4 flex-none text-accent-blue" />
        ) : (
          <Folder class="w-4 h-4 flex-none text-accent-blue" />
        )}
        <span class="flex-1 min-w-0 truncate text-[13px] text-ink-100">{node.name}</span>
        <span class="text-[11px] text-ink-500 tabular-nums flex-none">{fileCount}</span>
        <a
          href={chatApi.folderDownloadUrl(chatId, dir, node.path)}
          onClick={(event) => event.stopPropagation()}
          class="h-6 w-6 grid place-items-center rounded text-ink-400 hover:text-accent-blue hover:bg-white/[0.08]
                 opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity flex-none"
          title={`Download ${node.name} as zip`}
          aria-label={`Download ${node.name} as zip`}
        >
          <Download class="w-3.5 h-3.5" />
        </a>
      </div>
      {isOpen && node.children && node.children.length > 0 && (
        <FileTreeNodes
          chatId={chatId}
          dir={dir}
          nodes={node.children}
          depth={depth + 1}
          expanded={expanded}
          onToggle={onToggle}
        />
      )}
    </li>
  );
}

function FileRow({ chatId, dir, node }: { chatId: string; dir: string; node: FileNode }) {
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
          href={chatApi.fileDownloadUrl(chatId, dir, node.path)}
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
