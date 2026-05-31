import { useMemo } from "preact/hooks";
import { Sidebar } from "../components/sidebar/Sidebar";
import { useAuthContext } from "../context/AuthContext";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { useSidebarState } from "../hooks/workspace/useSidebarState";
import { useWorkspaceCommands } from "../hooks/workspace/useWorkspaceCommands";
import { buildWorkspaceSidebarModel } from "../state/workspace/selectors";

export function SidebarContainer() {
  const { auth } = useAuthContext();
  const workspace = useWorkspaceContext();
  const sidebar = useSidebarState(workspace.ui.sidebarOpen, workspace.closeSidebar);
  const commands = useWorkspaceCommands();
  const model = useMemo(
    () => buildWorkspaceSidebarModel(workspace.chats, workspace.projects, sidebar.query),
    [workspace.chats, workspace.projects, sidebar.query]
  );

  return (
    <Sidebar
      open={workspace.ui.sidebarOpen}
      model={model}
      query={sidebar.query}
      collapsed={sidebar.collapsed}
      activeChatId={workspace.ui.activeChatId}
      account={{
        email: auth.email,
        authenticated: auth.authenticated,
        noAuth: auth.noAuth,
      }}
      onClose={workspace.closeSidebar}
      onQueryChange={sidebar.setQuery}
      onClearQuery={() => sidebar.setQuery("")}
      onNewProject={commands.newProject}
      onNewChatInProject={commands.newChatInProject}
      onToggleProject={sidebar.toggleCollapsed}
      onSelectChat={workspace.selectChat}
      onDeleteChat={commands.deleteChat}
      onStartProject={commands.startProject}
      onStopProject={commands.stopProject}
      onDeleteProject={commands.deleteProject}
      onOpenProjectContainers={workspace.showProjectContainers}
      onOpenSettings={workspace.showSettings}
    />
  );
}
