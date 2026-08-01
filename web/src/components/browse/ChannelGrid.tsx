import { Badge } from "@/components/ui/badge";
import { ChannelLogo } from "@/components/browse/ChannelLogo";
import { cn } from "@/lib/cn";
import type { Channel } from "@/lib/types";

interface ChannelGridProps {

  channels: Channel[];
  activeId?: string;

  onTune: (channel: Channel) => void;

}

export function ChannelGrid({ channels, activeId, onTune }: ChannelGridProps) {

  return (

    <div className="grid grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] gap-3">

      {channels.map((channel) => (

        <button
          key={channel.id}
          onClick={() => onTune(channel)}
          className={cn(
            "group bg-card hover:border-primary/60 focus-visible:ring-ring/50 flex flex-col gap-2 rounded-lg border p-3 text-left transition-colors outline-none focus-visible:ring-[3px]",
            activeId === channel.id && "border-primary",
          )}
        >

          <ChannelLogo logo={channel.logo} name={channel.name} />

          <div className="min-w-0">

            <p className="truncate text-sm font-medium">{channel.name}</p>

            <div className="mt-1 flex items-center gap-1.5">

              <span className="text-muted-foreground truncate text-xs">

                {[channel.country, channel.category].filter(Boolean).join(" · ")}

              </span>

              {channel.backups > 0 && <Badge variant="secondary" className="text-[10px]">backup</Badge>}

            </div>

          </div>

        </button>

      ))}

    </div>

  );

}
