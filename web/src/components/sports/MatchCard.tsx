import { ListPlus, Play } from "lucide-react";

import { TeamMark } from "@/components/sports/TeamMark";
import { prettyCategory, scorePair, shortTeamName } from "@/components/sports/matchUtils";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import type { SportsMatch } from "@/lib/types";

interface MatchCardProps {

  match: SportsMatch;
  onWatch: (match: SportsMatch) => void;
  onQueue: (match: SportsMatch) => void;

}

export function MatchCard({ match, onWatch, onQueue }: MatchCardProps) {

  const channel = match.channel ?? null;
  const scores = scorePair(match);

  const away = match.awayTeam || shortTeamName(match.title.split(/\s+vs\.?\s+/i)[0]);
  const home = match.homeTeam || shortTeamName(match.title.split(/\s+vs\.?\s+/i)[1]);

  return (

    <div
      className={cn(
        "bg-card flex w-full flex-col gap-3 rounded-xl border p-4",
        !channel && "opacity-65",
      )}
    >

      <div className="flex items-center justify-between gap-2">

        <span className="text-muted-foreground truncate text-[11px] font-medium">

          {match.league || prettyCategory(match.category)}

        </span>

        <span className="text-muted-foreground shrink-0 text-[11px] font-medium">

          {prettyCategory(match.category)}

        </span>

      </div>

      <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2">

        <TeamMark name={away} logo={match.awayLogo} size="sm" />

        <div className="flex flex-col items-center gap-0.5 px-1">

          {scores ? (

            <div className="flex items-baseline gap-1.5 text-2xl font-bold tabular-nums sm:text-3xl">

              <span>{scores.left}</span>
              <span className="text-muted-foreground text-lg font-normal">–</span>
              <span>{scores.right}</span>

            </div>

          ) : (

            <span className="text-muted-foreground text-xs font-semibold tracking-wider uppercase">vs</span>

          )}

          {match.statusDetail && (

            <span className="text-muted-foreground text-center text-[11px]">{match.statusDetail}</span>

          )}

        </div>

        <TeamMark name={home} logo={match.homeLogo} size="sm" />

      </div>

      <div className="flex items-center gap-2 border-t pt-3">

        {channel ? (

          <>

            <Button size="sm" className="flex-1" onClick={() => onWatch(match)}>

              <Play className="size-3.5 fill-current" />
              Watch Now

            </Button>

            <Button size="sm" variant="secondary" className="flex-1" onClick={() => onQueue(match)}>

              <ListPlus className="size-3.5" />
              Add to Queue

            </Button>

          </>

        ) : (

          <span className="text-muted-foreground w-full text-center text-xs">Scores only</span>

        )}

      </div>

    </div>

  );

}
