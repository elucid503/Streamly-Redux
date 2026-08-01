import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

import { useParticipants } from "@/hooks/useParticipants";
import { cn } from "@/lib/cn";

const actionPhrases: Record<string, string> = {

  play: "resumed playback",
  pause: "paused",
  seek: "jumped to a new position",
  setItem: "started this",
  setSubtitle: "changed subtitles",

};

// The one place the chrome departs from a solo player: it makes an unexpected pause legible (see _docs/DESIGN.md §6.2).
export function Presence({ className }: { className?: string }) {

  const { participants, lastActor } = useParticipants();

  return (

    <div className={cn("flex items-center gap-3", className)}>

      <div className="flex -space-x-2">

        {participants.map((participant) => (

          <Tooltip key={participant.userId}>

            <TooltipTrigger asChild>

              <div className="ring-background/60 size-7 overflow-hidden rounded-full bg-secondary ring-2">

                {participant.avatar ? (

                  <img src={participant.avatar} alt={participant.name} className="size-full object-cover" />

                ) : (

                  <span className="flex size-full items-center justify-center text-xs font-medium">

                    {participant.name.slice(0, 1).toUpperCase()}

                  </span>

                )}

              </div>

            </TooltipTrigger>

            <TooltipContent>{participant.name}</TooltipContent>

          </Tooltip>

        ))}

      </div>

      {lastActor && (

        <span className="truncate text-xs text-white/70">

          {lastActor.name} {actionPhrases[lastActor.action] ?? lastActor.action}

        </span>

      )}

    </div>

  );

}
