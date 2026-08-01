import type { SportsMatch } from "@/lib/types";

export const startingSoonWindowSecs = 3 * 60 * 60;

export function prettyCategory(category: string): string {

  const overrides: Record<string, string> = {

    afl: "AFL",
    mma: "MMA",
    ufc: "UFC",
    football: "Football",
    soccer: "Football",
    "american-football": "American Football",
    "motor-sports": "Formula 1",
    hockey: "Hockey",

  };

  if (overrides[category]) {

    return overrides[category];

  }

  return category.replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());

}

export function splitStart(unixSecs: number): { day: string | null; time: string } {

  if (!unixSecs || unixSecs <= 0 || Number.isNaN(unixSecs)) {

    return { day: null, time: "TBD" };

  }

  const date = new Date(unixSecs * 1000);

  if (Number.isNaN(date.getTime())) {

    return { day: null, time: "TBD" };

  }

  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();
  const time = date.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });

  if (isToday) {

    return { day: null, time };

  }

  return { day: date.toLocaleDateString([], { weekday: "short", month: "short", day: "numeric" }), time };

}

export function isLiveMatch(match: SportsMatch): boolean {

  return match.live || match.status === "in";

}

export function isFinished(match: SportsMatch): boolean {

  return match.status === "post";

}

/** Scores only for live or finished games — pre-game 0–0 from ESPN is not a scoreboard. */
export function scorePair(match: SportsMatch): { left: number; right: number } | null {

  if (match.status === "pre") {

    return null;

  }

  if (!isLiveMatch(match) && !isFinished(match)) {

    return null;

  }

  if (match.homeScore === undefined || match.awayScore === undefined) {

    return null;

  }

  return {
    left: match.awayScore,
    right: match.homeScore,
  };

}

export function bucketFor(match: SportsMatch, nowSecs: number): "live" | "soon" | "upcoming" | "past" {

  if (match.status === "in" || match.live) {

    return "live";

  }

  if (match.status === "post") {

    return "past";

  }

  const delta = match.startsAt - nowSecs;

  // Pre-game fixtures keep 0–0 on the wire; never treat those as finished.
  if (match.status === "pre" || match.homeScore === undefined || match.awayScore === undefined) {

    if (delta <= 0) {

      // Started according to clock but scoreboard hasn't flipped — still "soon" briefly, else past.
      if (delta > -30 * 60) {

        return "soon";

      }

      return "past";

    }

    if (delta <= startingSoonWindowSecs) {

      return "soon";

    }

    return "upcoming";

  }

  // Has scores and is not pre/in — treat as finished.
  return "past";

}

export function shortTeamName(name?: string): string {

  if (!name) {

    return "TBD";

  }

  // Prefer the last word for city-prefixed clubs ("Pittsburgh Pirates" → "Pirates").
  const parts = name.trim().split(/\s+/);

  if (parts.length >= 2 && parts[parts.length - 1]!.length > 2) {

    return parts[parts.length - 1]!;

  }

  return name;

}
