import { ProjectContainersRoute } from "../app/routes/ProjectContainersRoute";
import { SettingsRoute } from "../app/routes/SettingsRoute";
import { AppShell } from "../ui/layout/AppShell";
import { NoChatSelected } from "../ui/layout/NoChatSelected";
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
      ) : workspace.ui.view === "project-containers" ? (
        <ProjectContainersRoute
          projects={workspace.projects}
          selectedProjectId={workspace.ui.containerProjectId}
          onBack={workspace.showChat}
          onHamburger={workspace.openSidebar}
        />
      ) : workspace.activeChat ? (
        <ChatContainer
          key={workspace.activeChat.id}
          chat={workspace.activeChat}
          projects={workspace.projects}
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
