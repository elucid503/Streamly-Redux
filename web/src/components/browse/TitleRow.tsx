import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Clapperboard, Star } from "lucide-react";

import { PlayOverlay } from "@/components/browse/PlayOverlay";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import type { Title } from "@/lib/types";

interface TitleRowProps {

  title: string;
  count?: number;
  titles: Title[];
  onOpen: (title: Title) => void;

}

export function TitleRow({ title, count, titles, onOpen }: TitleRowProps) {

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

  }, [titles, updateArrows]);

  const scrollByPage = (direction: -1 | 1) => {

    const el = scroller.current;

    if (!el) {

      return;

    }

    const amount = Math.max(el.clientWidth * 0.85, 240);

    el.scrollBy({ left: direction * amount, behavior: "smooth" });

  };

  if (titles.length === 0) {

    return null;

  }

  return (

    <section className="flex flex-col gap-3">

      <div className="flex items-center justify-between gap-3">

        <div className="flex min-w-0 items-baseline gap-2">

          <h2 className="text-sm font-semibold tracking-tight">{title}</h2>

          {count !== undefined && <span className="text-muted-foreground text-xs">{count}</span>}

        </div>

        <div className="flex shrink-0 items-center gap-1">

          <Button
            type="button"
            variant="secondary"
            size="icon-sm"
            disabled={!canLeft}
            aria-label={`Scroll ${title} left`}
            onClick={() => scrollByPage(-1)}
          >

            <ChevronLeft className="size-4" />

          </Button>

          <Button
            type="button"
            variant="secondary"
            size="icon-sm"
            disabled={!canRight}
            aria-label={`Scroll ${title} right`}
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

        {titles.map((entry) => (

          <TitleCard key={`${entry.boxType}-${entry.id}`} title={entry} onOpen={onOpen} />

        ))}

      </div>

    </section>

  );

}

function TitleCard({ title, onOpen }: { title: Title; onOpen: (title: Title) => void }) {

  return (

    <button
      type="button"
      onClick={() => onOpen(title)}
      className="group focus-visible:ring-ring/50 flex w-[8.5rem] shrink-0 flex-col gap-2 rounded-lg text-left outline-none transition-[filter] hover:brightness-110 focus-visible:ring-[3px] sm:w-36"
    >

      <div className="bg-card relative aspect-[2/3] overflow-hidden rounded-lg border">

        {title.poster ? (

          <img
            src={title.poster}
            alt=""
            loading="lazy"
            className="size-full object-cover"
          />

        ) : (

          <div className="flex size-full items-center justify-center">

            <Clapperboard className="text-muted-foreground size-6" />

          </div>

        )}

        <PlayOverlay />

        {title.rating && (

          <Badge variant="secondary" className="absolute top-1.5 right-1.5 z-20 gap-1 bg-black/70 text-[10px]">

            <Star className="size-2.5 fill-current" />
            {title.rating}

          </Badge>

        )}

      </div>

      <div className="min-w-0">

        <p className="truncate text-sm font-medium">{title.title}</p>

        <p className="text-muted-foreground text-xs">

          {title.boxType === 2 ? "Series" : "Movie"}
          {title.year > 0 && ` · ${title.year}`}

        </p>

      </div>

    </button>

  );

}
