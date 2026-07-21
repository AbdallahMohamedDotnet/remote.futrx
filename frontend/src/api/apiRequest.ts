import { sendHttpRequest, type HttpMethod } from "../transport/http";

export async function requestJson<T>(
  method: HttpMethod,
  url: string,
  body?: unknown
): Promise<T> {
  const response = await sendHttpRequest(method, url, body);
  if (response.status === 401) {
    location.reload();
    return new Promise<T>(() => {});
  }
  if (!response.ok) {
    let msg = `${response.status}`;
    try {
      msg = (await response.json()).error || msg;
    } catch {}
    throw new Error(msg);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
