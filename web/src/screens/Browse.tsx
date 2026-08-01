import { useCallback, useEffect, useMemo, useState } from "react";
import { Clapperboard, Tv } from "lucide-react";

import { DiscoveryFilter, filterTitles, type RatingFloor } from "@/components/browse/DiscoveryFilter";
import { HistoryRow } from "@/components/browse/HistoryRow";
import { QueueSheet } from "@/components/browse/QueueSheet";
import { SearchBar } from "@/components/browse/SearchBar";
import { TitleDetail } from "@/components/browse/TitleDetail";
import { TitleGrid } from "@/components/browse/TitleGrid";
import { TitleRow } from "@/components/browse/TitleRow";
import { MiniPlaceholder } from "@/components/layout/MiniPlaceholder";
import { PageLoader } from "@/components/PageLoader";
import { Button } from "@/components/ui/button";

import { useMiniMode } from "@/hooks/useMiniMode";
import { useRoom } from "@/hooks/useRoom";
import { getHistory, getTrending, search, type HistoryEntry, type SearchResults, type TopPicks } from "@/lib/api";
import { movieCaption } from "@/lib/titleMeta";
import type { Title } from "@/lib/types";

interface BrowseProps {

  onWatch: () => void;

}

// Movies, series, and the queue share this tab; live channels live under Live TV.
export function Browse({ onWatch }: BrowseProps) {

  const mini = useMiniMode();
  const { state, session, setItem } = useRoom();

  const [picks, setPicks] = useState<TopPicks | null>(null);
  const [history, setHistory] = useState<HistoryEntry[]>([]);

  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResults | null>(null);
  const [searching, setSearching] = useState(false);

  const [selected, setSelected] = useState<Title | null>(null);

  const [rating, setRating] = useState<RatingFloor>(null);
  const [genre, setGenre] = useState<string | null>(null);
  const [queueOpen, setQueueOpen] = useState(false);

  useEffect(() => {

    if (session.config.vodEnabled) {

      void getTrending().then(setPicks).catch(() => setPicks({ movies: [], series: [], nowPlaying: [] }));

    } else {

      setPicks({ movies: [], series: [], nowPlaying: [] });

    }

  }, [session.config.vodEnabled]);

  // Refresh when this room starts something new so the home rail stays current.
  const historyKey = state.item
    ? `${state.item.kind}:${state.item.id}:${state.item.season ?? 0}:${state.item.episode ?? 0}`
    : "";

  useEffect(() => {

    if (!session.guildId) {

      setHistory([]);
      return;

    }

    let cancelled = false;

    void getHistory(session.guildId, session.socketTicket)
      .then((items) => !cancelled && setHistory(items))
      .catch(() => !cancelled && setHistory([]));

    return () => {

      cancelled = true;

    };

  }, [session.guildId, session.socketTicket, historyKey]);

  useEffect(() => {

    if (query === "") {

      setResults(null);
      return;

    }

    let cancelled = false;

    setSearching(true);

    void search(query)
      .then((found) => !cancelled && setResults(found))
      .catch(() => !cancelled && setResults({ channels: [], titles: [] }))
      .finally(() => !cancelled && setSearching(false));

    return () => {

      cancelled = true;

    };

  }, [query]);

  const catalogTitles = useMemo(() => {

    if (results) {

      return results.titles;

    }

    return [...(picks?.movies ?? []), ...(picks?.series ?? []), ...(picks?.nowPlaying ?? [])];

  }, [results, picks]);

  const filteredMovies = useMemo(() => {

    return filterTitles(picks?.movies ?? [], rating, genre);

  }, [picks, rating, genre]);

  const filteredSeries = useMemo(() => {

    return filterTitles(picks?.series ?? [], rating, genre);

  }, [picks, rating, genre]);

  const filteredNowPlaying = useMemo(() => {

    return filterTitles(picks?.nowPlaying ?? [], rating, genre);

  }, [picks, rating, genre]);

  const filteredSearchTitles = useMemo(() => {

    if (!results) {

      return [];

    }

    return filterTitles(results.titles, rating, genre);

  }, [results, rating, genre]);

  const recentVod = useMemo(
    () => history.filter((entry) => entry.kind === "vod"),
    [history],
  );

  const recentChannels = useMemo(
    () => history.filter((entry) => entry.kind === "channel"),
    [history],
  );

  const openTitle = useCallback((title: Title) => {

    // Movies skip the detail page and go straight into the room player.
    if (title.boxType !== 2) {

      setItem({
        kind: "vod",
        id: title.id,
        title: title.title,
        poster: title.poster,
        boxType: 1,
        caption: movieCaption(title),
        description: title.description,
        tmdbId: title.tmdbId,
      });

      onWatch();
      return;

    }

    setSelected(title);

  }, [setItem, onWatch]);

  const openHistory = useCallback((entry: HistoryEntry) => {

    if (entry.kind === "channel") {

      setItem({
        kind: "channel",
        id: entry.id,
        title: entry.title,
        caption: entry.caption,
        poster: entry.poster,
      });

      onWatch();
      return;

    }

    if (entry.boxType === 2) {

      setSelected({
        id: entry.id,
        boxType: 2,
        title: entry.title,
        year: 0,
        poster: entry.poster,
      });

      return;

    }

    setItem({
      kind: "vod",
      id: entry.id,
      title: entry.title,
      poster: entry.poster,
      boxType: 1,
      caption: entry.caption,
      description: entry.description,
    });

    onWatch();

  }, [setItem, onWatch]);

  if (mini) {

    return (

      <MiniPlaceholder
        icon={Clapperboard}
        label={selected ? `Viewing ${selected.title}` : "Browsing Movies & Shows"}
      />

    );

  }

  if (selected) {

    return (

      <div className="min-h-full w-full min-w-0 px-4 pt-4 pb-[var(--bottom-dock-clearance)] sm:px-6 sm:pt-6 lg:px-8 lg:pt-8">

        <TitleDetail title={selected} onBack={() => setSelected(null)} onPlay={onWatch} />

      </div>

    );

  }

  return (

    <div className="relative flex min-h-full w-full flex-col gap-8 px-4 pt-4 pb-[var(--bottom-dock-clearance)] sm:px-6 sm:pt-6 lg:px-8 lg:pt-8">

      {picks === null && <PageLoader label="Loading Streamly" />}

      <div className="flex flex-col gap-3">

        <div className="flex items-center gap-2">

          <div className="flex-1">

            <SearchBar
              value={query}
              searching={searching}
              onChange={setQuery}
              placeholder="Search movies and series..."
            />

          </div>

          <QueueSheet open={queueOpen} onOpenChange={setQueueOpen} onPlay={onWatch} />

          {state.item && (

            <Button variant="secondary" className="watching-callout" onClick={onWatch}>

              <Tv />
              Watching

            </Button>

          )}

        </div>

        {session.config.vodEnabled && (

          <DiscoveryFilter
            titles={catalogTitles}
            rating={rating}
            genre={genre}
            onRating={setRating}
            onGenre={setGenre}
          />

        )}

      </div>

      {results ? (

        <>

          {filteredSearchTitles.length > 0 && (

            <Section title="Movies & series" count={filteredSearchTitles.length}>

              <TitleGrid titles={filteredSearchTitles} onOpen={openTitle} />

            </Section>

          )}

          {filteredSearchTitles.length === 0 && !searching && (

            <p className="text-muted-foreground text-sm">No movies or series match.</p>

          )}

        </>

      ) : (

        <>

          <HistoryRow title="Recently played" items={recentVod} onSelect={openHistory} />

          <HistoryRow title="Recently streamed" items={recentChannels} onSelect={openHistory} />

          <TitleRow title="Trending movies" count={filteredMovies.length} titles={filteredMovies} onOpen={openTitle} />

          <TitleRow title="Trending series" count={filteredSeries.length} titles={filteredSeries} onOpen={openTitle} />

          <TitleRow title="In theaters" count={filteredNowPlaying.length} titles={filteredNowPlaying} onOpen={openTitle} />

        </>

      )}

    </div>

  );

}

function Section({ title, count, children }: { title: string; count?: number; children: React.ReactNode }) {

  return (

    <section className="flex flex-col gap-3">

      <div className="flex items-baseline gap-2">

        <h2 className="text-sm font-semibold tracking-tight">{title}</h2>

        {count !== undefined && <span className="text-muted-foreground text-xs">{count}</span>}

      </div>

      {children}

    </section>

  );

}
