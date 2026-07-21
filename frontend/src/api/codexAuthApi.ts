import { DeviceAuthApi } from "./deviceAuthApi";
import type { CodexAuthStatus, CodexDeviceLogin } from "../models/auth";
import { API_ROUTES, WEB_SOCKET_ROUTES } from "../config/routes";

export const codexAuthApi = new DeviceAuthApi<
  CodexAuthStatus,
  CodexDeviceLogin
>({
  status: API_ROUTES.codexAuth.status,
  startDeviceLogin: API_ROUTES.codexAuth.startDeviceLogin,
  statusUpdates: WEB_SOCKET_ROUTES.codexAuthStatus,
});
