import { SettingsRoute } from "../app/routes/SettingsRoute";
import { AppShell } from "../components/layout/AppShell";
import { NoChatSelected } from "../components/layout/NoChatSelected";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { useWorkspaceCommands } from "../hooks/workspace/useWorkspaceCommands";
import { ChatContainer } from "./ChatContainer";
import { SidebarContainer } from "./SidebarContainer";

export function WorkspaceContainer() {
  const workspace = useWorkspaceContext();
  const commands = useWorkspaceCommands();

  return (
    <AppShell sidebar={<SidebarContainer />}>
      {workspace.ui.view === "settings" ? (
        <SettingsRoute
          onBack={workspace.showChat}
          onHamburger={workspace.openSidebar}
        />
      ) : workspace.activeChat ? (
        <ChatContainer
          key={workspace.activeChat.id}
          chat={workspace.activeChat}
          onHamburger={workspace.openSidebar}
          onMetaUpdate={workspace.refreshChats}
        />
      ) : (
        <NoChatSelected
          hasProjects={workspace.projects.length > 0}
          onNewProject={commands.newProject}
          onNewChat={() => commands.newChatInProject(undefined)}
          onHamburger={workspace.openSidebar}
        />
      )}
    </AppShell>
  );
}
