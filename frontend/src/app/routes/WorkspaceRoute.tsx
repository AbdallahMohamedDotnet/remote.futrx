import { WorkspaceProvider } from "../../context/WorkspaceContext";
import { WorkspaceContainer } from "../../containers/WorkspaceContainer";

export function WorkspaceRoute({ enabled }: { enabled: boolean }) {
  return (
    <WorkspaceProvider enabled={enabled}>
      <WorkspaceContainer />
    </WorkspaceProvider>
  );
}
