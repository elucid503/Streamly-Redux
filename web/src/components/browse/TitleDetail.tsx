import { useEffect, useState } from "react";
import { ArrowLeft, Clapperboard, ListPlus, Play, Star } from "lucide-react";

import { PageLoader } from "@/components/PageLoader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

import { useRoom } from "@/hooks/useRoom";
import { getTitle } from "@/lib/api";
import { movieCaption } from "@/lib/titleMeta";
import type { Episode, Item, Title, TitleDetail as Detail } from "@/lib/types";

interface TitleDetailProps {

  title: Title;

  onBack: () => void;
  onPlay: () => void;

}

export function TitleDetail({ title, onBack, onPlay }: TitleDetailProps) {

  const { setItem, enqueue } = useRoom();

  const [detail, setDetail] = useState<Detail | null>(null);
  const [season, setSeason] = useState(1);
  const [pendingSeason, setPendingSeason] = useState<number | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {

    let cancelled = false;

    setDetail(null);
    setFailed(false);
    setPendingSeason(null);

    void getTitle(title.boxType, title.id, title.source)
      .then((loaded) => {

        if (cancelled) {

          return;

        }

        setDetail(loaded);
        setSeason(loaded.seasons[0]?.season ?? 1);

      })
      .catch(() => !cancelled && setFailed(true));

    return () => {

      cancelled = true;

    };

  }, [title.boxType, title.id, title.source]);

  // Keep the current episode grid mounted until every thumbnail for the next season is ready.
  useEffect(() => {

    if (pendingSeason === null || !detail) {

      return;

    }

    let cancelled = false;

    const episodes = detail.seasons.find((entry) => entry.season === pendingSeason)?.episodes ?? [];

    void preloadEpisodeThumbnails(episodes).then(() => {

      if (cancelled) {

        return;

      }

      setSeason(pendingSeason);
      setPendingSeason(null);

    });

    return () => {

      cancelled = true;

    };

  }, [pendingSeason, detail]);

  const episodes = detail?.seasons.find((entry) => entry.season === season)?.episodes ?? [];

  const itemFor = (episodeNumber?: number, episodeTitle?: string, _thumbnail?: string, description?: string): Item => ({

    kind: "vod",

    id: detail?.id ?? title.id,
    title: title.title,

    // Always the title poster — episode stills are for the grid only, not history/player art.
    poster: detail?.poster || title.poster,

    boxType: title.boxType,

    season: episodeNumber ? season : undefined,
    episode: episodeNumber,
    episodeTitle,
    description: description || detail?.description,

    caption: episodeNumber
      ? `S${season} · E${episodeNumber}`
      : movieCaption({
          year: detail?.year ?? title.year,
          rating: detail?.rating ?? title.rating,
          genres: detail?.genres ?? title.genres,
        }),

    imdbId: detail?.imdbId,
    tmdbId: detail?.tmdbId,

  });

  return (

    <div className="relative flex min-h-full w-full min-w-0 flex-col gap-6">

      {!detail && !failed && <PageLoader label="Loading title" />}

      <Button variant="ghost" size="sm" className="-ml-2 w-fit" onClick={onBack}>

        <ArrowLeft />
        Back

      </Button>

      <div className="overflow-hidden rounded-xl border bg-card">

        <div className="relative">

          <div className="relative aspect-[5/1] min-h-16 w-full overflow-hidden bg-secondary/40 sm:min-h-20">

            {detail?.banner ? (

              <img src={detail.banner} alt="" className="absolute inset-0 size-full object-cover object-top" />

            ) : (detail?.poster ?? title.poster) ? (

              <img src={detail?.poster ?? title.poster} alt="" className="absolute inset-0 size-full object-cover object-center opacity-80" />

            ) : null}

            <div className="absolute inset-0 bg-gradient-to-t from-card via-card/55 to-transparent" />

          </div>

          <div className="relative -mt-8 flex gap-3 px-4 pb-3 sm:-mt-10 sm:gap-4 sm:px-5 sm:pb-4">

            <div className="h-24 w-[4.5rem] shrink-0 overflow-hidden rounded-md border bg-card shadow-lg sm:h-28 sm:w-20">

              {detail?.poster ?? title.poster ? (

                <img src={detail?.poster ?? title.poster} alt="" className="size-full object-cover" />

              ) : (

                <div className="flex size-full items-center justify-center">

                  <Clapperboard className="text-muted-foreground size-5" />

                </div>

              )}

            </div>

            <div className="flex min-w-0 flex-1 flex-col justify-end gap-2 pb-0.5">

              <div>

                <h2 className="text-lg leading-tight font-semibold sm:text-xl">{title.title}</h2>

                <div className="text-muted-foreground mt-1 flex flex-wrap items-center gap-2 text-xs">

                  <span>{title.boxType === 2 ? "Series" : "Movie"}</span>

                  {(detail?.year ?? title.year) > 0 && <span>· {detail?.year ?? title.year}</span>}

                  {(detail?.rating ?? title.rating) && (

                    <Badge variant="secondary" className="gap-1 text-[10px]">

                      <Star className="size-2.5 fill-current" />
                      {detail?.rating ?? title.rating}

                    </Badge>

                  )}

                </div>

              </div>

              {failed && <p className="text-destructive text-sm">Could not load this title</p>}

              {detail?.description && (

                <p className="text-muted-foreground line-clamp-2 max-w-3xl text-sm leading-relaxed">{detail.description}</p>

              )}

              {title.boxType === 1 && detail && (

                <div className="flex gap-2">

                  <Button
                    size="sm"
                    onClick={() => {

                      setItem(itemFor());
                      onPlay();

                    }}
                  >

                    <Play />
                    Play now

                  </Button>

                  <Button variant="secondary" size="sm" onClick={() => enqueue(itemFor())}>

                    <ListPlus />
                    Queue

                  </Button>

                </div>

              )}

            </div>

          </div>

        </div>

      </div>

      {title.boxType === 2 && detail && detail.seasons.length > 0 && (

        <div className="flex w-full min-w-0 flex-col gap-3">

          <div className="flex flex-wrap gap-1.5">

            {detail.seasons.map((entry) => {

              const active = entry.season === season;

              return (

                <Button
                  key={entry.season}
                  variant={active ? "default" : "secondary"}
                  size="sm"
                  onClick={() => {

                    if (entry.season === season) {

                      setPendingSeason(null);
                      return;

                    }

                    if (entry.season === pendingSeason) {

                      return;

                    }

                    setPendingSeason(entry.season);

                  }}
                >

                  Season {entry.season}

                </Button>

              );

            })}

          </div>

          <div className="grid w-full min-w-0 grid-cols-1 gap-2 lg:grid-cols-2">

            {episodes.map((episode) => (

              <div key={episode.episode} className="flex min-w-0 max-w-full items-center gap-3 rounded-lg border bg-card p-2">

                <div className="bg-secondary/50 relative h-16 w-28 shrink-0 overflow-hidden rounded-md">

                  {episode.thumbnail ? (

                    <img src={episode.thumbnail} alt="" className="size-full object-cover" />

                  ) : (detail.poster ?? title.poster) ? (

                    <img src={detail.poster ?? title.poster} alt="" className="size-full object-cover opacity-50" />

                  ) : (

                    <div className="text-muted-foreground flex size-full items-center justify-center font-mono text-xs">

                      E{episode.episode}

                    </div>

                  )}

                </div>

                <div className="min-w-0 flex-1 overflow-hidden">

                  <p className="text-muted-foreground font-mono text-[11px]">E{episode.episode}</p>
                  <p className="truncate text-sm font-medium">{episode.title || `Episode ${episode.episode}`}</p>

                  {episode.description && (

                    <p className="text-muted-foreground mt-0.5 truncate text-xs">{episode.description}</p>

                  )}

                </div>

                <div className="mr-1 flex shrink-0 flex-row gap-0.5">

                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => {

                      setItem(itemFor(episode.episode, episode.title, episode.thumbnail, episode.description));
                      onPlay();

                    }}
                  >

                    <Play />

                  </Button>

                  <Button variant="ghost" size="icon-sm" onClick={() => enqueue(itemFor(episode.episode, episode.title, episode.thumbnail, episode.description))}>

                    <ListPlus />

                  </Button>

                </div>

              </div>

            ))}

          </div>

        </div>

      )}

    </div>

  );

}

function preloadEpisodeThumbnails(episodes: Episode[]): Promise<void> {

  const urls = [...new Set(episodes.map((episode) => episode.thumbnail).filter((url): url is string => Boolean(url)))];

  if (urls.length === 0) {

    return Promise.resolve();

  }

  return Promise.all(urls.map(preloadImage)).then(() => undefined);

}

function preloadImage(src: string): Promise<void> {

  return new Promise((resolve) => {

    const image = new Image();

    image.onload = () => resolve();
    image.onerror = () => resolve();
    image.src = src;

  });

}
