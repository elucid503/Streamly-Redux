import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, Subtitles } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

import { useRoom } from "@/hooks/useRoom";
import { getSubtitles } from "@/lib/api";
import { loadSubtitleCues } from "@/lib/subtitles";
import { cn } from "@/lib/cn";
import type { SubtitleTrack } from "@/lib/types";

interface LanguageGroup {

  language: string;
  tracks: SubtitleTrack[];

}

export function SubtitleMenu() {

  const { state, setSubtitle, session } = useRoom();

  const [tracks, setTracks] = useState<SubtitleTrack[]>([]);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);

  const item = state.item;

  useEffect(() => {

    setTracks([]);
    setFailed(false);

  }, [item?.id, item?.season, item?.episode]);

  const load = useCallback(async () => {

    if (!item || item.kind !== "vod" || tracks.length > 0 || loading) {

      return;

    }

    setLoading(true);
    setFailed(false);

    try {

      setTracks(await getSubtitles({

        id: item.id,
        boxType: item.boxType,
        source: item.source,

        imdbId: item.imdbId,
        tmdbId: item.tmdbId,

        series: item.boxType === 2,

        season: item.season,
        episode: item.episode,

        release: state.playback?.releaseName,

      }));

    } catch {

      setFailed(true);

    } finally {

      setLoading(false);

    }

  }, [item, tracks.length, loading, state.playback?.releaseName]);

  const groups = useMemo(() => groupByLanguage(tracks), [tracks]);

  const pick = useCallback((track: SubtitleTrack | null) => {

    setSubtitle(track);

    // Warm the cue cache immediately so the overlay is ready before the first rAF tick.
    if (track) {

      void loadSubtitleCues(track).catch((error: unknown) => {

        console.error("[streamly] subtitle prefetch failed", error);

      });

    }

  }, [setSubtitle]);

  const disabled = !session.config.subtitlesEnabled || item?.kind !== "vod";

  return (

    <DropdownMenu onOpenChange={(open) => open && void load()}>

      <DropdownMenuTrigger asChild>

        <Button variant="ghost" size="icon" className="text-white hover:bg-white/15 hover:text-white data-[state=open]:bg-white/15" disabled={disabled}>

          <Subtitles className={state.subtitle ? "text-primary" : undefined} />

        </Button>

      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="max-h-[200px] w-52 overflow-y-auto">

        <DropdownMenuLabel className="px-2 py-1.5">

          <div className="text-sm font-medium text-foreground">Subtitles</div>
          <div className="text-muted-foreground text-xs font-normal">Changes for everyone</div>

        </DropdownMenuLabel>

        <DropdownMenuSeparator />

        <DropdownMenuItem className="justify-between gap-3" onSelect={() => pick(null)}>

          <span>Off</span>
          <Check className={cn("size-3.5 shrink-0", state.subtitle ? "opacity-0" : "opacity-100")} />

        </DropdownMenuItem>

        {loading && <div className="text-muted-foreground px-2 py-1.5 text-sm">Searching…</div>}

        {failed && <div className="text-destructive px-2 py-1.5 text-sm">Subtitle search failed</div>}

        {!loading && !failed && groups.length === 0 && (

          <div className="text-muted-foreground px-2 py-1.5 text-sm">None found</div>

        )}

        {groups.map((group) => (

          <DropdownMenuSub key={group.language}>

            <DropdownMenuSubTrigger>

              <span className="flex-1 truncate">{group.language}</span>

              {state.subtitle && group.tracks.some((track) => track.id === state.subtitle?.id) && (

                <span className="text-muted-foreground text-[10px]">On</span>

              )}

            </DropdownMenuSubTrigger>

            <DropdownMenuSubContent className="max-h-[200px] w-44 overflow-y-auto">

              {group.tracks.map((track, index) => {

                const selected = state.subtitle?.id === track.id;

                return (

                  <DropdownMenuItem
                    key={track.id}
                    className="justify-between gap-3"
                    onSelect={() => pick(track)}
                  >

                    <span className="truncate">{group.language} {index + 1}</span>
                    <Check className={cn("size-3.5 shrink-0", selected ? "opacity-100" : "opacity-0")} />

                  </DropdownMenuItem>

                );

              })}

            </DropdownMenuSubContent>

          </DropdownMenuSub>

        ))}

      </DropdownMenuContent>

    </DropdownMenu>

  );

}

function groupByLanguage(tracks: SubtitleTrack[]): LanguageGroup[] {

  const counts = new Map<string, number>();
  const order: string[] = [];
  const buckets = new Map<string, SubtitleTrack[]>();

  for (const track of tracks) {

    const language = languageName(track.label);

    if (!buckets.has(language)) {

      order.push(language);
      buckets.set(language, []);

    }

    const next = (counts.get(language) ?? 0) + 1;

    counts.set(language, next);

    buckets.get(language)!.push({

      ...track,
      label: `${language} ${next}`,

    });

  }

  return order.map((language) => ({

    language,
    tracks: buckets.get(language) ?? [],

  }));

}

function languageName(label: string): string {

  const upper = label.toUpperCase();

  const known: Array<[string, string]> = [

    ["SPANISH", "Spanish"],
    ["ENGLISH", "English"],
    ["FRENCH", "French"],
    ["GERMAN", "German"],
    ["PORTUGUESE", "Portuguese"],
    ["ITALIAN", "Italian"],
    ["JAPANESE", "Japanese"],
    ["KOREAN", "Korean"],
    ["CHINESE", "Chinese"],
    ["RUSSIAN", "Russian"],
    ["ARABIC", "Arabic"],
    ["HINDI", "Hindi"],
    ["DUTCH", "Dutch"],
    ["POLISH", "Polish"],
    ["TURKISH", "Turkish"],
    ["SWEDISH", "Swedish"],
    ["NORWEGIAN", "Norwegian"],
    ["DANISH", "Danish"],
    ["FINNISH", "Finnish"],
    ["GREEK", "Greek"],
    ["HEBREW", "Hebrew"],
    ["THAI", "Thai"],
    ["VIETNAMESE", "Vietnamese"],
    ["INDONESIAN", "Indonesian"],
    ["CZECH", "Czech"],
    ["HUNGARIAN", "Hungarian"],
    ["ROMANIAN", "Romanian"],
    ["UKRAINIAN", "Ukrainian"],

  ];

  for (const [needle, name] of known) {

    if (upper.includes(needle)) {

      return name;

    }

  }

  const code = upper.match(/\b([A-Z]{2,3})\b/);

  if (code) {

    return codeMap[code[1]] ?? code[1];

  }

  const stripped = label.replace(/\s+\d+$/, "").trim();

  return stripped || "Track";

}

const codeMap: Record<string, string> = {

  EN: "English",
  ENG: "English",
  ES: "Spanish",
  SPA: "Spanish",
  FR: "French",
  FRE: "French",
  FRA: "French",
  DE: "German",
  GER: "German",
  DEU: "German",
  PT: "Portuguese",
  POR: "Portuguese",
  IT: "Italian",
  ITA: "Italian",
  JA: "Japanese",
  JPN: "Japanese",
  KO: "Korean",
  KOR: "Korean",
  ZH: "Chinese",
  CHI: "Chinese",
  ZHO: "Chinese",
  RU: "Russian",
  RUS: "Russian",
  AR: "Arabic",
  ARA: "Arabic",
  HI: "Hindi",
  HIN: "Hindi",
  NL: "Dutch",
  DUT: "Dutch",
  NLD: "Dutch",
  PL: "Polish",
  POL: "Polish",
  TR: "Turkish",
  TUR: "Turkish",

};
