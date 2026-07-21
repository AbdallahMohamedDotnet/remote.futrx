import { DeviceAuthApi } from "./deviceAuthApi";
import type { KimiAuthStatus, KimiDeviceLogin } from "../models/auth";
import { API_ROUTES, WEB_SOCKET_ROUTES } from "../config/routes";

export const kimiAuthApi = new DeviceAuthApi<KimiAuthStatus, KimiDeviceLogin>({
  status: API_ROUTES.kimiAuth.status,
  startDeviceLogin: API_ROUTES.kimiAuth.startDeviceLogin,
  statusUpdates: WEB_SOCKET_ROUTES.kimiAuthStatus,
});
