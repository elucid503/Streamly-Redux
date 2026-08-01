import { useCallback, useEffect, useMemo, useState } from "react";
import { Radio, Tv } from "lucide-react";

import { ChannelFilter } from "@/components/browse/ChannelFilter";
import { ChannelGrid } from "@/components/browse/ChannelGrid";
import { QueueSheet } from "@/components/browse/QueueSheet";
import { SearchBar } from "@/components/browse/SearchBar";
import { MiniPlaceholder } from "@/components/layout/MiniPlaceholder";
import { PageLoader } from "@/components/PageLoader";
import { Button } from "@/components/ui/button";

import { useMiniMode } from "@/hooks/useMiniMode";
import { useRoom } from "@/hooks/useRoom";
import { getChannels } from "@/lib/api";
import type { Channel } from "@/lib/types";

interface LiveProps {

  onWatch: () => void;

}

export function Live({ onWatch }: LiveProps) {

  const mini = useMiniMode();
  const { state, setItem } = useRoom();

  const [channels, setChannels] = useState<Channel[] | null>(null);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<string | null>(null);
  const [country, setCountry] = useState<string | null>(null);
  const [queueOpen, setQueueOpen] = useState(false);

  useEffect(() => {

    void getChannels().then(setChannels).catch(() => setChannels([]));

  }, []);

  const visible = useMemo(() => {

    const needle = query.trim().toLowerCase();

    return (channels ?? []).filter((channel) => {

      if (category !== null && channel.category !== category) {

        return false;

      }

      if (country !== null && channel.country !== country) {

        return false;

      }

      if (needle && !channel.name.toLowerCase().includes(needle)) {

        return false;

      }

      return true;

    });

  }, [channels, category, country, query]);

  const tune = useCallback((channel: Channel) => {

    setItem({ kind: "channel", id: channel.id, title: channel.name, caption: channel.category, poster: channel.logo });

    onWatch();

  }, [setItem, onWatch]);

  if (mini) {

    return <MiniPlaceholder icon={Radio} label="Browsing Live TV" />;

  }

  return (

    <div className="relative flex min-h-full w-full flex-col gap-4 px-4 pt-4 pb-[var(--bottom-dock-clearance)] sm:px-6 sm:pt-6 lg:px-8 lg:pt-8">

      {channels === null && <PageLoader label="Loading channels" />}

      <div className="flex items-center gap-2">

        <div className="flex-1">

          <SearchBar value={query} searching={false} onChange={setQuery} placeholder="Search live TV channels..." />

        </div>

        <QueueSheet open={queueOpen} onOpenChange={setQueueOpen} onPlay={onWatch} />

        {state.item && (

          <Button variant="secondary" onClick={onWatch}>

            <Tv />
            Watching

          </Button>

        )}

      </div>

      {channels !== null && (

        <ChannelFilter
          channels={channels}
          category={category}
          country={country}
          onCategory={setCategory}
          onCountry={setCountry}
        />

      )}

      {channels !== null && (

        visible.length === 0 ? (

          <p className="text-muted-foreground text-sm">No channels match this filter</p>

        ) : (

          <ChannelGrid channels={visible} activeId={state.item?.id} onTune={tune} />

        )

      )}

    </div>

  );

}
