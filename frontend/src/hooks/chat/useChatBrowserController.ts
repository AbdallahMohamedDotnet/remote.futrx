import { useCallback, useEffect, useState } from "preact/hooks";
import type { RefObject } from "preact";
import type { ChatMeta } from "../../models/chat";
import type { ContainerApp, ProjectMeta } from "../../models/project";
import type { Block } from "../../state/chat/messageBlocks";
import { projectService } from "../../api/projectService";
import type { BrowserElementCapture } from "../../components/chat/browser/BrowserDrawer";

export function useChatBrowserController({
  chat,
  projects,
  blocks,
  text,
  setText,
  textareaRef,
}: {
  chat: ChatMeta;
  projects: ProjectMeta[];
  blocks: Block[];
  text: string;
  setText: (text: string) => void;
  textareaRef: RefObject<HTMLTextAreaElement>;
}) {
  const [browserOpen, setBrowserOpen] = useState(false);
  const [containerApps, setContainerApps] = useState<ContainerApp[]>([]);
  const [appsLoading, setAppsLoading] = useState(false);
  const [selectedAppPort, setSelectedAppPort] = useState<number | null>(null);
  const browserProject = chat.projectId
    ? projects.find((project) => project.id === chat.projectId) ?? null
    : null;
  const browserUrl = browserProject ? latestPublicDevUrl(blocks, browserProject.slug) : "";

  useEffect(() => {
    setBrowserOpen(false);
  }, [chat.id]);

  const loadContainerApps = useCallback(async () => {
    if (!chat.projectId) {
      setContainerApps([]);
      setSelectedAppPort(null);
      return;
    }
    setAppsLoading(true);
    try {
      const apps = await projectService.listApps(chat.projectId);
      setContainerApps(apps);
      setSelectedAppPort((prev) => {
        if (apps.length === 0) return null;
        if (prev != null && apps.some((app) => app.port === prev)) return prev;
        const hinted = portFromChatUrl(browserUrl);
        if (hinted != null && apps.some((app) => app.port === hinted)) return hinted;
        return apps[apps.length - 1].port;
      });
    } catch {
      setContainerApps([]);
      setSelectedAppPort(null);
    } finally {
      setAppsLoading(false);
    }
  }, [chat.projectId, browserUrl]);

  function openBrowserDrawer() {
    if (!chat.projectId) {
      alert("This chat is not attached to a project container.");
      return;
    }
    setBrowserOpen(true);
    void loadContainerApps();
  }

  function insertBrowserElementContext(capture: BrowserElementCapture) {
    const insertion = `\n\n${formatBrowserElementCapture(capture)}\n\n`;
    const textarea = textareaRef.current;
    const start = textarea?.selectionStart ?? text.length;
    const end = textarea?.selectionEnd ?? start;
    const next = `${text.slice(0, start)}${insertion}${text.slice(end)}`;
    setText(next);
    setTimeout(() => {
      textareaRef.current?.focus();
      const pos = start + insertion.length;
      textareaRef.current?.setSelectionRange(pos, pos);
    }, 0);
  }

  return {
    browserOpen,
    browserProject,
    containerApps,
    appsLoading,
    selectedAppPort,
    setSelectedAppPort,
    openBrowserDrawer,
    closeBrowserDrawer: () => setBrowserOpen(false),
    loadContainerApps,
    insertBrowserElementContext,
  };
}

function latestPublicDevUrl(blocks: Block[], slug: string): string {
  let latest = "";
  for (const block of blocks) {
    for (const text of blockTexts(block)) {
      for (const candidate of publicDevUrls(text)) {
        if (isProjectDevUrl(candidate, slug)) latest = candidate;
      }
    }
  }
  return latest;
}

function blockTexts(block: Block): string[] {
  if (block.type === "user") return [block.text];
  if (block.type === "error") return [block.message];
  return block.parts.flatMap((part) => {
    if (part.kind === "text" || part.kind === "thinking") return [part.text];
    return part.output ? [part.output] : [];
  });
}

function publicDevUrls(text: string): string[] {
  return [...text.matchAll(/https:\/\/[a-z0-9][a-z0-9-]*--\d{4,5}\.dev\.remote\.futrx\.dev[^\s<>)\]]*/g)]
    .map((match) => match[0].replace(/[.,;:!?]+$/, ""));
}

function isProjectDevUrl(raw: string, slug: string): boolean {
  try {
    const url = new URL(raw);
    const suffix = ".dev.remote.futrx.dev";
    const portStart = `${slug}--`;
    return (
      url.protocol === "https:" &&
      url.hostname.startsWith(portStart) &&
      url.hostname.endsWith(suffix) &&
      validDevPort(url.hostname.slice(portStart.length, -suffix.length))
    );
  } catch {
    return false;
  }
}

function validDevPort(port: string): boolean {
  const value = Number(port);
  return Number.isInteger(value) && value >= 1024 && value <= 65535;
}

function formatBrowserElementCapture(capture: BrowserElementCapture): string {
  const lines = [
    "[Browser element]",
    `URL: ${capture.url || ""}`,
  ];
  if (capture.title) lines.push(`Title: ${capture.title}`);
  lines.push(`Selector: ${capture.selector || ""}`);
  lines.push(`Tag: ${capture.tag || ""}`);
  if (capture.id) lines.push(`ID: ${capture.id}`);
  if (capture.classes?.length) lines.push(`Classes: ${capture.classes.join(" ")}`);
  if (capture.role) lines.push(`Role: ${capture.role}`);
  if (capture.ariaLabel) lines.push(`ARIA label: ${capture.ariaLabel}`);
  if (capture.rect) {
    lines.push(`Box: x=${capture.rect.x} y=${capture.rect.y} w=${capture.rect.width} h=${capture.rect.height}`);
  }
  if (capture.viewport) {
    lines.push(`Viewport: ${capture.viewport.width}x${capture.viewport.height}`);
  }
  if (capture.parents?.length) {
    lines.push(`Parents: ${capture.parents.join(" > ")}`);
  }
  if (capture.styles && Object.keys(capture.styles).length) {
    lines.push("Styles:");
    for (const [key, value] of Object.entries(capture.styles)) {
      if (value) lines.push(`- ${key}: ${value}`);
    }
  }
  if (capture.text) lines.push(`Text: ${capture.text}`);
  if (capture.html) lines.push(`HTML: ${capture.html}`);
  lines.push("[/Browser element]");
  return lines.join("\n");
}

function portFromChatUrl(url: string): number | null {
  const match = /--(\d{4,5})\./.exec(url);
  return match ? Number(match[1]) : null;
}
