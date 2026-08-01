import { useState } from "react";
import { LibraryBig, X } from "lucide-react";

import { Player } from "@/components/player/Player";
import { PageLoader } from "@/components/PageLoader";
import { Button } from "@/components/ui/button";

import { useParticipants } from "@/hooks/useParticipants";
import { useRoom } from "@/hooks/useRoom";

interface WatchProps {

  arrived: boolean;

  onBrowse: () => void;

}

export function Watch({ arrived, onBrowse }: WatchProps) {

  const { state, pendingItem } = useRoom();
  const { others } = useParticipants();

  const [cardDismissed, setCardDismissed] = useState(false);

  if (!state.item) {

    if (pendingItem) {

      return (

        <div className="relative h-full w-full">

          <PageLoader absolute label={`Starting ${pendingItem.title}`} />

        </div>

      );

    }

    return (

      <div className="flex h-full flex-col items-center justify-center gap-4 p-6 text-center">

        <p className="text-muted-foreground text-sm">Nothing is playing</p>

        <Button onClick={onBrowse}>

          <LibraryBig />
          Browse

        </Button>

      </div>

    );

  }

  return (

    <div className="relative h-full w-full">

      <Player onBrowse={onBrowse} />

      {pendingItem && <PageLoader absolute label={`Starting ${pendingItem.title}`} />}

      {/* Someone arriving mid-session is walking into a room where the TV is already on (see _docs/DESIGN.md §6.1). */}
      {arrived && !cardDismissed && (

        <div className="bg-popover/95 absolute top-14 left-3 z-30 flex max-w-xs items-start gap-3 rounded-lg border p-3 shadow-lg backdrop-blur">

          <div className="min-w-0">

            <p className="truncate text-sm font-medium">{state.item.title}</p>

            <p className="text-muted-foreground mt-0.5 text-xs">

              {others.length === 0

                ? "You are the first one here"
                : `Watching with ${others.map((participant) => participant.name).join(", ")}`}

            </p>

          </div>

          <Button variant="ghost" size="icon-sm" className="-mt-1 -mr-1" onClick={() => setCardDismissed(true)}>

            <X />

          </Button>

        </div>

      )}

    </div>

  );

}
