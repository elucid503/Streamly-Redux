import { ChevronDown, ChevronUp, ListVideo, Play, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useRoom } from "@/hooks/useRoom";
import { cn } from "@/lib/cn";

interface QueueProps {

  onPlay: () => void;

}

// VOD only — tuning to a channel leaves this untouched, and stopping the channel comes back to it (see _docs/DESIGN.md §4.6).
export function Queue({ onPlay }: QueueProps) {

  const { state, setItem, dequeue, reorder } = useRoom();

  if (state.queue.length === 0) {

    return (

      <div className="border-border/60 text-muted-foreground flex items-center gap-2 rounded-lg py-2 px-1 text-sm">

        <ListVideo className="size-4" />
        Nothing queued, yet

      </div>

    );

  }

  return (

    <div className="flex flex-col gap-1.5">

      {state.queue.map((item, index) => (

        <div
          key={`${item.id}-${item.season ?? 0}-${item.episode ?? 0}`}
          className={cn(
            "bg-card flex items-center gap-3 rounded-lg border p-2 pl-3",
            index === state.queueIndex && state.item?.kind === "vod" && "border-primary",
          )}
        >

          <div className="min-w-0 flex-1">

            <p className="truncate text-sm font-medium">{item.title}</p>

            {item.caption && <p className="text-muted-foreground truncate text-xs">{item.caption}</p>}

          </div>

          <div className="flex items-center gap-0.5">

            <Button
              variant="ghost"
              size="icon-sm"
              disabled={index === 0}
              onClick={() => reorder(index, index - 1)}
            >

              <ChevronUp />

            </Button>

            <Button
              variant="ghost"
              size="icon-sm"
              disabled={index === state.queue.length - 1}
              onClick={() => reorder(index, index + 1)}
            >

              <ChevronDown />

            </Button>

            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => {

                setItem(item);
                onPlay();

              }}
            >

              <Play />

            </Button>

            <Button variant="ghost" size="icon-sm" onClick={() => dequeue(index)}>

              <X />

            </Button>

          </div>

        </div>

      ))}

    </div>

  );

}
