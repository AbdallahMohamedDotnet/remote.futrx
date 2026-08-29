import { capabilitiesApi } from "../../../api/agents/capabilitiesApi";
import { AgentCapabilityCatalogStore } from "./agentCapabilityCatalogStore";

export const agentCapabilityCatalogStore = new AgentCapabilityCatalogStore(
  capabilitiesApi.list,
);
