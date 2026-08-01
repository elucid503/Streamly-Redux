import { useMemo, useState } from "react";
import { Check, ChevronDown } from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";

import { Button } from "@/components/ui/button";
import { popoverVariants, snappySpring } from "@/lib/motion";
import { cn } from "@/lib/cn";
import type { Title } from "@/lib/types";

export type RatingFloor = null | 6 | 7 | 8 | 9;

interface DiscoveryFilterProps {

  titles: Title[];

  rating: RatingFloor;
  genre: string | null;

  onRating: (rating: RatingFloor) => void;
  onGenre: (genre: string | null) => void;

}

const ratingOptions: Array<{ value: RatingFloor; label: string }> = [

  { value: null, label: "Any rating" },
  { value: 6, label: "6+" },
  { value: 7, label: "7+" },
  { value: 8, label: "8+" },
  { value: 9, label: "9+" },

];

export function DiscoveryFilter({ titles, rating, genre, onRating, onGenre }: DiscoveryFilterProps) {

  const genres = useMemo(() => collectGenres(titles), [titles]);

  return (

    <div className="flex flex-wrap items-center gap-1.5">

      <RatingSelect value={rating} onChange={onRating} />

      <Button variant={genre === null ? "default" : "secondary"} size="sm" onClick={() => onGenre(null)}>

        All genres

      </Button>

      {genres.map((entry) => (

        <Button
          key={entry}
          variant={genre === entry ? "default" : "secondary"}
          size="sm"
          onClick={() => onGenre(genre === entry ? null : entry)}
        >

          {entry}

        </Button>

      ))}

    </div>

  );

}

export function filterTitles(titles: Title[], rating: RatingFloor, genre: string | null): Title[] {

  return titles.filter((title) => {

    if (rating !== null) {

      const score = Number.parseFloat(title.rating ?? "");

      if (!Number.isFinite(score) || score < rating) {

        return false;

      }

    }

    if (genre !== null) {

      if (!(title.genres ?? []).includes(genre)) {

        return false;

      }

    }

    return true;

  });

}

function RatingSelect({ value, onChange }: { value: RatingFloor; onChange: (value: RatingFloor) => void }) {

  const [open, setOpen] = useState(false);

  const label = ratingOptions.find((entry) => entry.value === value)?.label ?? "Any rating";

  return (

    <div className="relative">

      <Button
        variant="secondary"
        size="sm"
        className="min-w-28 justify-between gap-2 pr-2"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
      >

        <span className="truncate">{label}</span>
        <ChevronDown className={cn("size-3.5 opacity-70 transition-transform duration-200", open && "rotate-180")} />

      </Button>

      <AnimatePresence>

        {open && (

          <>

            <button
              type="button"
              className="fixed inset-0 z-40 cursor-default"
              aria-label="Close rating menu"
              onClick={() => setOpen(false)}
            />

            <motion.div
              className="bg-popover text-popover-foreground absolute top-[calc(100%+0.35rem)] left-0 z-50 min-w-36 origin-top-left overflow-hidden rounded-md border p-1 shadow-lg"
              variants={popoverVariants}
              initial="hidden"
              animate="visible"
              exit="exit"
              transition={snappySpring}
            >

              {ratingOptions.map((entry) => {

                const selected = entry.value === value;

                return (

                  <button
                    key={entry.label}
                    type="button"
                    className={cn(
                      "hover:bg-accent hover:text-accent-foreground flex w-full items-center justify-between gap-3 rounded-sm px-2 py-1.5 text-left text-sm",
                      selected && "bg-accent/60",
                    )}
                    onClick={() => {

                      onChange(entry.value);
                      setOpen(false);

                    }}
                  >

                    <span>{entry.label}</span>
                    <Check className={cn("size-3.5 shrink-0", selected ? "opacity-100" : "opacity-0")} />

                  </button>

                );

              })}

            </motion.div>

          </>

        )}

      </AnimatePresence>

    </div>

  );

}

function collectGenres(titles: Title[]): string[] {

  const counts = new Map<string, number>();

  for (const title of titles) {

    for (const genre of title.genres ?? []) {

      counts.set(genre, (counts.get(genre) ?? 0) + 1);

    }

  }

  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .slice(0, 10)
    .map(([genre]) => genre);

}
