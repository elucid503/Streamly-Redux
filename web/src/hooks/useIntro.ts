import { useEffect, useState } from "react";

import { useRoom } from "@/hooks/useRoom";
import { getIntro } from "@/lib/api";
import type { IntroRange } from "@/lib/types";

// One lookup per episode while autoplaying a season, which is why the token is used rather than the anonymous tier (see _docs/DESIGN.md §5.6).
export function useIntro(positionSeconds: number, durationSeconds: number): IntroRange | null {

  const { state } = useRoom();

  const [ranges, setRanges] = useState<IntroRange[]>([]);

  const item = state.item;

  // Room state arrives as a fresh object on every change, so the lookup keys off the episode rather than the reference.
  const episodeKey = item ? `${item.id}:${item.season ?? 0}:${item.episode ?? 0}` : "";
  const measured = durationSeconds > 0;

  useEffect(() => {

    setRanges([]);

    if (!item || item.kind !== "vod" || (!item.imdbId && !item.tmdbId)) {

      return;

    }

    let cancelled = false;

    void getIntro({

      imdbId: item.imdbId,
      tmdbId: item.tmdbId,

      series: item.boxType === 2,

      season: item.season,
      episode: item.episode,

      durationMs: durationSeconds > 0 ? durationSeconds * 1000 : undefined,

    })
      .then((found) => {

        if (!cancelled) {

          setRanges(found);

        }

      })
      .catch(() => undefined);

    return () => {

      cancelled = true;

    };

  }, [episodeKey, measured]);

  const positionMs = positionSeconds * 1000;

  return ranges.find((range) => positionMs >= range.startMs && positionMs < range.endMs) ?? null;

}
