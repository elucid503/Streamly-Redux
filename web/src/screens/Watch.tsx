import { LibraryBig, Tv } from "lucide-react";

import { MiniPlaceholder } from "@/components/layout/MiniPlaceholder";
import { Player } from "@/components/player/Player";
import { PageLoader } from "@/components/PageLoader";
import { Button } from "@/components/ui/button";

import { useMiniMode } from "@/hooks/useMiniMode";
import { useRoom } from "@/hooks/useRoom";

interface WatchProps {

  arrived: boolean;

  onBrowse: () => void;

}

export function Watch({ onBrowse }: WatchProps) {

  const mini = useMiniMode();
  const { state, pendingItem } = useRoom();

  if (!state.item) {

    if (pendingItem) {

      if (mini) {

        return <MiniPlaceholder icon={Tv} label={`Starting ${pendingItem.title}`} />;

      }

      return (

        <div className="relative h-full w-full">

          <PageLoader absolute label={`Starting ${pendingItem.title}`} />

        </div>

      );

    }

    if (mini) {

      return <MiniPlaceholder icon={Tv} label="Nothing is playing" />;

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

    </div>

  );

}
