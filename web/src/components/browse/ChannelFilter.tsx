import { useMemo, useState } from "react";
import { Check, ChevronDown } from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";

import { Button } from "@/components/ui/button";
import { popoverVariants, snappySpring } from "@/lib/motion";
import { cn } from "@/lib/cn";
import type { Channel } from "@/lib/types";

const maxFacets = 7;

interface ChannelFilterProps {

  channels: Channel[];

  category: string | null;
  country: string | null;

  onCategory: (category: string | null) => void;
  onCountry: (country: string | null) => void;

}

// The catalog runs to several hundred channels, so the grid needs a way in that is not scrolling.
export function ChannelFilter({ channels, category, country, onCategory, onCountry }: ChannelFilterProps) {

  const categories = useMemo(() => facets(channels.map((channel) => channel.category)), [channels]);
  const countries = useMemo(() => facets(channels.map((channel) => channel.country), 40), [channels]);

  if (categories.length === 0 && countries.length === 0) {

    return null;

  }

  return (

    <div className="flex flex-wrap items-center gap-1.5">

      {countries.length > 0 && (

        <RegionSelect
          values={countries}
          selected={country}
          onSelect={onCountry}
        />

      )}

      {categories.length > 0 && (

        <>

          <Button variant={category === null ? "default" : "secondary"} size="sm" onClick={() => onCategory(null)}>

            All

          </Button>

          {categories.map((value) => (

            <Button
              key={value}
              variant={category === value ? "default" : "secondary"}
              size="sm"
              onClick={() => onCategory(category === value ? null : value)}
            >

              {value}

            </Button>

          ))}

        </>

      )}

    </div>

  );

}

function RegionSelect({

  values,
  selected,
  onSelect,

}: {

  values: string[];
  selected: string | null;
  onSelect: (value: string | null) => void;

}) {

  const [open, setOpen] = useState(false);

  const label = selected ?? "Everywhere";

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
              aria-label="Close region menu"
              onClick={() => setOpen(false)}
            />

            <motion.div
              className="bg-popover text-popover-foreground absolute top-[calc(100%+0.35rem)] left-0 z-50 max-h-64 min-w-40 origin-top-left overflow-y-auto rounded-md border p-1 shadow-lg"
              variants={popoverVariants}
              initial="hidden"
              animate="visible"
              exit="exit"
              transition={snappySpring}
            >

              <RegionOption
                label="Everywhere"
                selected={selected === null}
                onPick={() => {

                  onSelect(null);
                  setOpen(false);

                }}
              />

              {values.map((value) => (

                <RegionOption
                  key={value}
                  label={value}
                  selected={selected === value}
                  onPick={() => {

                    onSelect(value);
                    setOpen(false);

                  }}
                />

              ))}

            </motion.div>

          </>

        )}

      </AnimatePresence>

    </div>

  );

}

function RegionOption({

  label,
  selected,
  onPick,

}: {

  label: string;
  selected: boolean;
  onPick: () => void;

}) {

  return (

    <button
      type="button"
      className={cn(
        "hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm",
        selected && "bg-accent/60",
      )}
      onClick={onPick}
    >

      <Check className={cn("size-3.5 shrink-0", selected ? "opacity-100" : "opacity-0")} />
      <span className="truncate">{label}</span>

    </button>

  );

}

// Ordered by how much of the catalog each one actually covers, so the useful filters come first.
function facets(values: (string | undefined)[], limit = maxFacets): string[] {

  const counts = new Map<string, number>();

  for (const value of values) {

    if (value) {

      counts.set(value, (counts.get(value) ?? 0) + 1);

    }

  }

  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, limit)
    .map(([value]) => value);

}
