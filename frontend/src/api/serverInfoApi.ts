import { API_ROUTES } from "../config/routes";
import type { ServerInfo } from "../models/serverInfo";
import { requestJson } from "./apiRequest";

export const serverInfoApi = {
  fetch: () => requestJson<ServerInfo>("GET", API_ROUTES.serverInfo),
};
