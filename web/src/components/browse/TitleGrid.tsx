import { Clapperboard, Star } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import type { Title } from "@/lib/types";

interface TitleGridProps {

  titles: Title[];

  onOpen: (title: Title) => void;

}

export function TitleGrid({ titles, onOpen }: TitleGridProps) {

  return (

    <div className="grid grid-cols-[repeat(auto-fill,minmax(8rem,1fr))] gap-3">

      {titles.map((title) => (

        <button
          key={`${title.boxType}-${title.id}`}
          onClick={() => onOpen(title)}
          className="group focus-visible:ring-ring/50 flex flex-col gap-2 rounded-lg text-left outline-none focus-visible:ring-[3px]"
        >

          <div className="bg-card relative aspect-[2/3] overflow-hidden rounded-lg border">

            {title.poster ? (

              <img
                src={title.poster}
                alt=""
                loading="lazy"
                className="size-full object-cover transition-transform duration-200 group-hover:scale-105"
              />

            ) : (

              <div className="flex size-full items-center justify-center">

                <Clapperboard className="text-muted-foreground size-6" />

              </div>

            )}

            {title.rating && (

              <Badge variant="secondary" className="absolute top-1.5 right-1.5 gap-1 bg-black/70 text-[10px]">

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

      ))}

    </div>

  );

}
