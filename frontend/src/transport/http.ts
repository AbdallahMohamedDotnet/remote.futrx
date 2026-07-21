type HttpMethod = "DELETE" | "GET" | "PATCH" | "POST" | "PUT";

export async function sendHttpRequest(
  method: HttpMethod,
  url: string,
  body?: unknown,
  init?: RequestInit
): Promise<Response> {
  const headers = new Headers(init?.headers);
  let payload: BodyInit | undefined;

  if (body instanceof FormData) {
    payload = body;
  } else if (body !== undefined) {
    headers.set("Content-Type", "application/json");
    payload = JSON.stringify(body);
  }

  return fetch(url, {
    ...init,
    method,
    headers,
    body: payload,
    credentials: init?.credentials ?? "same-origin",
  });
}

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
