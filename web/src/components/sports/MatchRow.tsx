import { ListPlus } from "lucide-react";

import { scorePair, shortTeamName, splitStart } from "@/components/sports/matchUtils";
import { Button } from "@/components/ui/button";
import type { SportsMatch } from "@/lib/types";

interface MatchRowProps {

  match: SportsMatch;
  /** schedule = soon/upcoming (queue action); result = finished (display only). */
  variant?: "schedule" | "result";
  onQueue?: (match: SportsMatch) => void;

}

function MiniLogo({ src, name }: { src?: string; name?: string }) {

  if (!src) {

    return (

      <span className="bg-muted text-muted-foreground flex size-7 shrink-0 items-center justify-center rounded-md text-[10px] font-semibold">

        {(name ?? "?").trim().charAt(0).toUpperCase()}

      </span>

    );

  }

  return (

    <img
      src={src}
      alt=""
      className="bg-muted size-7 shrink-0 rounded-md object-contain p-0.5"
      loading="lazy"
      decoding="async"
    />

  );

}

export function MatchRow({ match, variant = "schedule", onQueue }: MatchRowProps) {

  const channel = match.channel ?? null;
  const scores = scorePair(match);
  const { day, time } = splitStart(match.startsAt);

  const away = match.awayTeam || shortTeamName(match.title.split(/\s+vs\.?\s+/i)[0]);
  const home = match.homeTeam || shortTeamName(match.title.split(/\s+vs\.?\s+/i)[1]);

  return (

    <div className="bg-card flex w-full items-center gap-3 rounded-lg border px-3 py-2.5 sm:gap-4 sm:px-4">

      <div className="hidden w-20 shrink-0 flex-col items-center justify-center sm:flex">

        {variant === "result" ? (

          <span className="text-muted-foreground text-[11px] font-semibold">Final</span>

        ) : (

          <>

            {day && <span className="text-muted-foreground text-[10px]">{day}</span>}
            <span className="text-sm font-semibold tabular-nums">{time}</span>

          </>

        )}

      </div>

      <div className="flex min-w-0 flex-1 items-center gap-2.5 sm:gap-3">

        <div className="flex min-w-0 items-center gap-2">

          <MiniLogo src={match.awayLogo} name={away} />
          <span className="truncate text-sm font-medium">{shortTeamName(away)}</span>

        </div>

        <div className="text-muted-foreground shrink-0 text-xs font-medium tabular-nums">

          {variant === "result" && scores ? (

            <span className="text-foreground font-semibold">

              {scores.left}
              <span className="text-muted-foreground mx-1 font-normal">–</span>
              {scores.right}

            </span>

          ) : (

            "vs"

          )}

        </div>

        <div className="flex min-w-0 items-center gap-2">

          <MiniLogo src={match.homeLogo} name={home} />
          <span className="truncate text-sm font-medium">{shortTeamName(home)}</span>

        </div>

      </div>

      {variant === "schedule" && channel && onQueue && (

        <Button
          type="button"
          variant="secondary"
          size="sm"
          className="shrink-0"
          onClick={() => onQueue(match)}
        >

          <ListPlus className="size-3.5" />
          <span className="hidden sm:inline">Add to Queue</span>
          <span className="sm:hidden">Queue</span>

        </Button>

      )}

      {variant === "schedule" && !channel && (

        <span className="text-muted-foreground shrink-0 text-[11px]">Unavailable</span>

      )}

    </div>

  );

}
