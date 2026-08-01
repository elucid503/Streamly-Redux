import { parseWebVtt, type Cue } from "@/lib/vtt";
import type { SubtitleTrack } from "@/lib/types";

const cache = new Map<string, Cue[]>();
const inflight = new Map<string, Promise<Cue[]>>();

export function subtitleUrl(track: SubtitleTrack): string {

  // Prefer the server-built URL so season/episode query params survive for zip member selection.
  if (track.url) {

    if (track.url.startsWith("/api/")) {

      return `/.proxy${track.url}`;

    }

    if (track.url.startsWith("/.proxy/")) {

      return track.url;

    }

    if (track.url.startsWith("http://") || track.url.startsWith("https://")) {

      return track.url;

    }

  }

  // Fallback for room-state tracks that only retained the path id.
  if (track.id) {

    return `/.proxy/api/subtitle?path=${encodeURIComponent(track.id)}`;

  }

  return "";

}

export async function loadSubtitleCues(track: SubtitleTrack): Promise<Cue[]> {

  const key = track.id || track.url;

  if (!key) {

    return [];

  }

  const hit = cache.get(key);

  if (hit) {

    return hit;

  }

  const pending = inflight.get(key);

  if (pending) {

    return pending;

  }

  const url = subtitleUrl(track);

  if (!url) {

    return [];

  }

  const work = (async () => {

    const response = await fetch(url, {

      headers: { Accept: "text/vtt, text/plain, */*" },
      cache: "no-store",

    });

    if (!response.ok) {

      throw new Error(`subtitle download failed with ${response.status}`);

    }

    const body = await response.text();

    if (!body.trim() || /^\s*</.test(body) || /<html[\s>]/i.test(body)) {

      throw new Error("subtitle response was empty or HTML");

    }

    const cues = parseWebVtt(body);

    // Never cache a failed parse — a transient empty body would lock the track off forever.
    if (cues.length > 0) {

      cache.set(key, cues);

    } else {

      console.warn("[streamly] subtitle parse produced no cues", {

        id: track.id,
        bytes: body.length,
        preview: body.slice(0, 180),

      });

    }

    return cues;

  })();

  inflight.set(key, work);

  try {

    return await work;

  } finally {

    inflight.delete(key);

  }

}
