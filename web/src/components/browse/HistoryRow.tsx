import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Clapperboard, Radio } from "lucide-react";

import { PlayOverlay } from "@/components/browse/PlayOverlay";
import { Button } from "@/components/ui/button";
import type { HistoryEntry } from "@/lib/api";
import { cn } from "@/lib/cn";
import { formatTime } from "@/lib/clock";

interface HistoryRowProps {

  items: HistoryEntry[];

  onSelect: (entry: HistoryEntry) => void;

}

export function HistoryRow({ items, onSelect }: HistoryRowProps) {

  const scroller = useRef<HTMLDivElement | null>(null);

  const [canLeft, setCanLeft] = useState(false);
  const [canRight, setCanRight] = useState(false);

  const updateArrows = useCallback(() => {

    const el = scroller.current;

    if (!el) {

      setCanLeft(false);
      setCanRight(false);
      return;

    }

    const max = el.scrollWidth - el.clientWidth;

    setCanLeft(el.scrollLeft > 4);
    setCanRight(max > 4 && el.scrollLeft < max - 4);

  }, []);

  useEffect(() => {

    const el = scroller.current;

    if (!el) {

      return;

    }

    updateArrows();

    const onScroll = () => updateArrows();

    el.addEventListener("scroll", onScroll, { passive: true });

    const observer = new ResizeObserver(() => updateArrows());

    observer.observe(el);

    return () => {

      el.removeEventListener("scroll", onScroll);
      observer.disconnect();

    };

  }, [items, updateArrows]);

  const scrollByPage = (direction: -1 | 1) => {

    const el = scroller.current;

    if (!el) {

      return;

    }

    const amount = Math.max(el.clientWidth * 0.85, 240);

    el.scrollBy({ left: direction * amount, behavior: "smooth" });

  };

  if (items.length === 0) {

    return null;

  }

  return (

    <section className="flex flex-col gap-3">

      <div className="flex items-center justify-between gap-3">

        <div className="flex min-w-0 items-baseline gap-2">

          <h2 className="text-sm font-semibold tracking-tight">Recently played</h2>

          <span className="text-muted-foreground text-xs">{items.length}</span>

        </div>

        <div className="flex shrink-0 items-center gap-1">

          <Button
            type="button"
            variant="secondary"
            size="icon-sm"
            disabled={!canLeft}
            aria-label="Scroll recently played left"
            onClick={() => scrollByPage(-1)}
          >

            <ChevronLeft className="size-4" />

          </Button>

          <Button
            type="button"
            variant="secondary"
            size="icon-sm"
            disabled={!canRight}
            aria-label="Scroll recently played right"
            onClick={() => scrollByPage(1)}
          >

            <ChevronRight className="size-4" />

          </Button>

        </div>

      </div>

      <div
        ref={scroller}
        className={cn(
          "flex gap-3 overflow-x-auto pb-1",
          "[scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden",
        )}
      >

        {items.map((entry) => (

          <HistoryCard key={entry.key} entry={entry} onSelect={onSelect} />

        ))}

      </div>

    </section>

  );

}

function HistoryCard({ entry, onSelect }: { entry: HistoryEntry; onSelect: (entry: HistoryEntry) => void }) {

  const isChannel = entry.kind === "channel";
  const isSeries = entry.boxType === 2;

  const progress =
    entry.kind === "vod" && entry.durationMs && entry.durationMs > 0 && entry.positionMs
      ? Math.min(1, entry.positionMs / entry.durationMs)
      : 0;

  const subtitle = isChannel
    ? (entry.caption || "Live TV")
    : isSeries
      ? [
          entry.season ? `S${entry.season}` : null,
          entry.episode ? `E${entry.episode}` : null,
          entry.positionMs && entry.positionMs > 0 ? formatTime(entry.positionMs / 1000) : null,
        ]
          .filter(Boolean)
          .join(" · ") || "Series"
      : entry.positionMs && entry.positionMs > 0
        ? formatTime(entry.positionMs / 1000)
        : "Movie";

  return (

    <button
      type="button"
      onClick={() => onSelect(entry)}
      className="group focus-visible:ring-ring/50 flex w-[8.5rem] shrink-0 flex-col gap-2 rounded-lg text-left outline-none transition-[filter] hover:brightness-110 focus-visible:ring-[3px] sm:w-36"
    >

      <div
        className={cn(
          "bg-card relative overflow-hidden rounded-lg border",
          isChannel ? "aspect-video" : "aspect-[2/3]",
        )}
      >

        {entry.poster ? (

          <img
            src={entry.poster}
            alt=""
            loading="lazy"
            className={cn("size-full", isChannel ? "object-contain p-3" : "object-cover")}
          />

        ) : (

          <div className="flex size-full items-center justify-center">

            {isChannel ? (
              <Radio className="text-muted-foreground size-6" />
            ) : (
              <Clapperboard className="text-muted-foreground size-6" />
            )}

          </div>

        )}

        <PlayOverlay size={isChannel ? "sm" : "md"} />

        {progress > 0 && (

          <div className="absolute inset-x-0 bottom-0 z-20 h-1 bg-black/50">

            <div className="bg-primary h-full" style={{ width: `${progress * 100}%` }} />

          </div>

        )}

      </div>

      <div className="min-w-0">

        <p className="truncate text-sm font-medium">{entry.title}</p>

        <p className="text-muted-foreground text-xs">{subtitle}</p>

      </div>

    </button>

  );

}
