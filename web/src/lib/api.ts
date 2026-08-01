import type { Channel, IntroRange, SportsMatch, SubtitleTrack, Title, TitleDetail } from "@/lib/types";

const base = "/.proxy";

export interface AppConfig {

  clientId: string;

  vodEnabled: boolean;
  subtitlesEnabled: boolean;

}

export interface SearchResults {

  channels: Channel[];
  titles: Title[];

}

export interface TopPicks {

  movies: Title[];
  series: Title[];
  nowPlaying?: Title[];

}

export interface SubtitleQuery {

  imdbId?: string;
  tmdbId?: number;

  series: boolean;

  season?: number;
  episode?: number;

  release?: string;

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

export async function exchangeToken(code: string): Promise<{ accessToken: string; socketTicket: string }> {

  return request<{ accessToken: string; socketTicket: string }>("/api/token", {

    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),

  });

}

export async function getChannels(): Promise<Channel[]> {

  const result = await request<{ channels: Channel[] }>("/api/channels");

  return result.channels;

}

export async function getSports(): Promise<SportsMatch[]> {

  const result = await request<{ matches: SportsMatch[] }>("/api/sports");

  return result.matches ?? [];

}

export async function search(query: string): Promise<SearchResults> {

  return request<SearchResults>(`/api/search?q=${encodeURIComponent(query)}`);

}

export async function getTrending(): Promise<TopPicks> {

  return request<TopPicks>("/api/trending");

}

export async function getTitle(boxType: number, id: string, source?: Title["source"]): Promise<TitleDetail> {

  const query = source === "tmdb" ? "?source=tmdb" : "";

  return request<TitleDetail>(`/api/title/${boxType}/${encodeURIComponent(id)}${query}`);

}

export async function getSubtitles(query: SubtitleQuery): Promise<SubtitleTrack[]> {

  const params = new URLSearchParams();

  if (query.imdbId) {

    params.set("imdbId", query.imdbId);

  }

  if (query.tmdbId) {

    params.set("tmdbId", String(query.tmdbId));

  }

  if (query.series) {

    params.set("series", "1");

  }

  if (query.season) {

    params.set("season", String(query.season));

  }

  if (query.episode) {

    params.set("episode", String(query.episode));

  }

  if (query.release) {

    params.set("release", query.release);

  }

  const result = await request<{ tracks: SubtitleTrack[] }>(`/api/subtitles?${params.toString()}`);

  return result.tracks;

}

export async function getIntro(query: SubtitleQuery & { durationMs?: number }): Promise<IntroRange[]> {

  const params = new URLSearchParams();

  if (query.imdbId) {

    params.set("imdbId", query.imdbId);

  }

  if (query.tmdbId) {

    params.set("tmdbId", String(query.tmdbId));

  }

  if (query.season) {

    params.set("season", String(query.season));

  }

  if (query.episode) {

    params.set("episode", String(query.episode));

  }

  if (query.durationMs) {

    params.set("durationMs", String(Math.round(query.durationMs)));

  }

  const result = await request<{ intro: IntroRange[] }>(`/api/intro?${params.toString()}`);

  return result.intro;

}
