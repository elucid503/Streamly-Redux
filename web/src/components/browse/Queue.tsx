import { ChevronDown, ChevronUp, ListVideo, Play, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useRoom } from "@/hooks/useRoom";
import type { Item } from "@/lib/types";

interface QueueProps {

  onPlay: () => void;

}

// Up-next list only — starting a row consumes it server-side (see _docs/DESIGN.md §4.6).
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
          key={queueRowKey(item)}
          className="bg-card flex items-center gap-3 rounded-lg border p-2 pl-3"
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

function queueRowKey(item: Item): string {

  return `${item.kind}:${item.id}:${item.season ?? 0}:${item.episode ?? 0}`;

}
