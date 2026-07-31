const base = "/.proxy";

export interface AppConfig {

  clientId: string;

}

async function request<T>(path: string, init?: RequestInit): Promise<T> {

  const response = await fetch(base + path, init);

  if (!response.ok) {

    throw new Error(`${path} failed with ${response.status}`);

  }

  return (await response.json()) as T;

}

export async function getConfig(): Promise<AppConfig> {

  return request<AppConfig>("/api/config");

}

export async function exchangeToken(code: string): Promise<string> {

  const result = await request<{ accessToken: string }>("/api/token", {

    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),

  });

  return result.accessToken;

}

export async function getStreamUrl(channelId: string): Promise<string> {

  const result = await request<{ url: string }>(`/api/stream/${encodeURIComponent(channelId)}`);

  return result.url;

}
