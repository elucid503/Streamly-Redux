import { useCallback, useEffect, useMemo, useState } from "react";
import { Trophy, Tv } from "lucide-react";

import { QueueSheet } from "@/components/browse/QueueSheet";
import { SearchBar } from "@/components/browse/SearchBar";
import { MatchCard } from "@/components/sports/MatchCard";
import { MatchRow } from "@/components/sports/MatchRow";
import { bucketFor, prettyCategory } from "@/components/sports/matchUtils";
import { MiniPlaceholder } from "@/components/layout/MiniPlaceholder";
import { PageLoader } from "@/components/PageLoader";
import { Button } from "@/components/ui/button";

import { useMiniMode } from "@/hooks/useMiniMode";
import { useRoom } from "@/hooks/useRoom";
import { getSports } from "@/lib/api";
import type { SportsMatch } from "@/lib/types";

const refreshMs = 60_000;
const livePreviewCount = 6;
const listPreviewCount = 6;

interface SportsProps {

  onWatch: () => void;

}

export function Sports({ onWatch }: SportsProps) {

  const mini = useMiniMode();
  const { state, setItem, enqueue } = useRoom();

  const [matches, setMatches] = useState<SportsMatch[] | null>(null);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<string | null>(null);
  const [queueOpen, setQueueOpen] = useState(false);

  const [showAllLive, setShowAllLive] = useState(false);
  const [showAllSoon, setShowAllSoon] = useState(false);
  const [showAllUpcoming, setShowAllUpcoming] = useState(false);
  const [showAllPast, setShowAllPast] = useState(false);

  const load = useCallback(async (initial: boolean) => {

    try {

      const next = await getSports();

      setMatches(next);

    } catch {

      if (initial) {

        setMatches([]);

      }

    }

  }, []);

  useEffect(() => {

    void load(true);

    const timer = window.setInterval(() => {

      void load(false);

    }, refreshMs);

    return () => window.clearInterval(timer);

  }, [load]);

  const categories = useMemo(() => {

    if (!matches) {

      return [];

    }

    return Array.from(new Set(["motor-sports", ...matches.map((m) => m.category).filter(Boolean)])).sort();

  }, [matches]);

  const filtered = useMemo(() => {

    if (!matches) {

      return [];

    }

    const needle = query.trim().toLowerCase();

    return matches.filter((m) => {

      if (category !== null && m.category !== category) {

        return false;

      }

      if (
        needle
        && !m.title.toLowerCase().includes(needle)
        && !(m.league ?? "").toLowerCase().includes(needle)
        && !(m.homeTeam ?? "").toLowerCase().includes(needle)
        && !(m.awayTeam ?? "").toLowerCase().includes(needle)
      ) {

        return false;

      }

      return true;

    });

  }, [matches, category, query]);

  const buckets = useMemo(() => {

    const nowSecs = Date.now() / 1000;

    const live: SportsMatch[] = [];
    const startingSoon: SportsMatch[] = [];
    const upcoming: SportsMatch[] = [];
    const past: SportsMatch[] = [];

    for (const match of filtered) {

      switch (bucketFor(match, nowSecs)) {

        case "live":
          live.push(match);
          break;
        case "soon":
          startingSoon.push(match);
          break;
        case "upcoming":
          upcoming.push(match);
          break;
        default:
          past.push(match);
          break;

      }

    }

    return { live, startingSoon, upcoming, past };

  }, [filtered]);

  const watchMatch = useCallback((match: SportsMatch) => {

    if (!match.channel) {

      return;

    }

    setItem({
      kind: "channel",
      id: match.channel.id,
      title: match.channel.name,
      caption: prettyCategory(match.category),
      poster: match.channel.logo,
    });

    onWatch();

  }, [setItem, onWatch]);

  const queueMatch = useCallback((match: SportsMatch) => {

    if (!match.channel) {

      return;

    }

    enqueue({
      kind: "channel",
      id: match.channel.id,
      title: match.title,
      caption: match.channel.name,
      poster: match.channel.logo,
    });

  }, [enqueue]);

  if (mini) {

    return <MiniPlaceholder icon={Trophy} label="Browsing Sports" />;

  }

  return (

    <div className="relative flex min-h-full w-full flex-col gap-5 px-4 pt-4 pb-[var(--bottom-dock-clearance)] sm:px-6 sm:pt-6 lg:px-8 lg:pt-8">

      {matches === null && <PageLoader label="Loading sports" />}

      <header className="flex flex-col gap-3">

        <div className="flex items-center gap-2">

          <div className="flex-1">

            <SearchBar value={query} searching={false} onChange={setQuery} placeholder="Search teams or leagues..." />

          </div>

          <QueueSheet open={queueOpen} onOpenChange={setQueueOpen} onPlay={onWatch} />

          {state.item && (

            <Button variant="secondary" onClick={onWatch}>

              <Tv />
              Watching

            </Button>

          )}

        </div>

        {matches !== null && categories.length > 0 && (

          <div className="flex flex-wrap items-center gap-1.5">

            <Button variant={category === null ? "default" : "secondary"} size="sm" onClick={() => setCategory(null)}>

              All

            </Button>

            {categories.map((value) => (

              <Button
                key={value}
                variant={category === value ? "default" : "secondary"}
                size="sm"
                onClick={() => setCategory(category === value ? null : value)}
              >

                {prettyCategory(value)}

              </Button>

            ))}

          </div>

        )}

      </header>

      {matches !== null && filtered.length === 0 && (

        <p className="text-muted-foreground py-12 text-center text-sm">No sports events match this filter</p>

      )}

      {matches !== null && filtered.length > 0 && (

        <div className="flex flex-col gap-7">

          {buckets.live.length > 0 && (

            <section className="flex flex-col gap-3">

              <SectionHeader
                title="Live now"
                count={buckets.live.length}
                expanded={showAllLive}
                canToggle={buckets.live.length > livePreviewCount}
                onToggle={() => setShowAllLive((v) => !v)}
              />

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">

                {(showAllLive ? buckets.live : buckets.live.slice(0, livePreviewCount)).map((match) => (

                  <MatchCard
                    key={match.id}
                    match={match}
                    onWatch={watchMatch}
                    onQueue={queueMatch}
                  />

                ))}

              </div>

            </section>

          )}

          <ListSection
            title="Starting soon"
            matches={buckets.startingSoon}
            expanded={showAllSoon}
            onToggle={() => setShowAllSoon((v) => !v)}
            onQueue={queueMatch}
            variant="schedule"
          />

          <ListSection
            title="Upcoming"
            matches={buckets.upcoming}
            expanded={showAllUpcoming}
            onToggle={() => setShowAllUpcoming((v) => !v)}
            onQueue={queueMatch}
            variant="schedule"
          />

          <ListSection
            title="Recently finished"
            matches={buckets.past}
            expanded={showAllPast}
            onToggle={() => setShowAllPast((v) => !v)}
            variant="result"
            previewCount={4}
          />

        </div>

      )}

    </div>

  );

}

function SectionHeader({

  title,
  count,
  expanded,
  canToggle,
  onToggle,

}: {

  title: string;
  count: number;
  expanded: boolean;
  canToggle: boolean;
  onToggle: () => void;

}) {

  return (

    <div className="flex items-center justify-between gap-3">

      <h2 className="text-sm font-semibold tracking-wide">

        {title}
        <span className="text-muted-foreground ml-2 font-medium tabular-nums">{count}</span>

      </h2>

      {canToggle && (

        <button type="button" onClick={onToggle} className="text-muted-foreground hover:text-foreground text-xs font-medium">

          {expanded ? "Show less" : "View all"}

        </button>

      )}

    </div>

  );

}

function ListSection({

  title,
  matches,
  expanded,
  onToggle,
  onQueue,
  variant,
  previewCount = listPreviewCount,

}: {

  title: string;
  matches: SportsMatch[];
  expanded: boolean;
  onToggle: () => void;
  onQueue?: (match: SportsMatch) => void;
  variant: "schedule" | "result";
  previewCount?: number;

}) {

  if (matches.length === 0) {

    return null;

  }

  const visible = expanded ? matches : matches.slice(0, previewCount);

  return (

    <section className="flex flex-col gap-3">

      <SectionHeader
        title={title}
        count={matches.length}
        expanded={expanded}
        canToggle={matches.length > previewCount}
        onToggle={onToggle}
      />

      <div className="flex flex-col gap-2">

        {visible.map((match) => (

          <MatchRow
            key={match.id}
            match={match}
            variant={variant}
            onQueue={onQueue}
          />

        ))}

      </div>

    </section>

  );

}
