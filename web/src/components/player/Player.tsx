import { useCallback, useEffect, useRef, useState } from "react";
import { LibraryBig, Loader2, SkipForward, Volume2 } from "lucide-react";

import { Controls } from "@/components/player/Controls";
import { PauseOverlay } from "@/components/player/PauseOverlay";
import { Presence } from "@/components/player/Presence";
import { Button } from "@/components/ui/button";

import { useIntro } from "@/hooks/useIntro";
import { usePlayer } from "@/hooks/usePlayer";
import { useRoom } from "@/hooks/useRoom";
import { useSubtitles } from "@/hooks/useSubtitles";
import { cn } from "@/lib/cn";

const chromeIdleMs = 2800;

interface PlayerProps {

  onBrowse: () => void;

}

export function Player({ onBrowse }: PlayerProps) {

  const { state, seek, play } = useRoom();

  const [video, setVideo] = useState<HTMLVideoElement | null>(null);

  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(true);
  const [chromeVisible, setChromeVisible] = useState(true);

  const idleTimer = useRef<number | null>(null);

  const handle = usePlayer(video);
  const intro = useIntro(handle.position, handle.duration);
  const cueText = useSubtitles(state.subtitle, video);

  useEffect(() => {

    if (!video) {

      return;

    }

    video.volume = volume;
    video.muted = muted;

  }, [video, volume, muted]);

  const wake = useCallback(() => {

    setChromeVisible(true);

    if (idleTimer.current !== null) {

      window.clearTimeout(idleTimer.current);

    }

    idleTimer.current = window.setTimeout(() => setChromeVisible(false), chromeIdleMs);

  }, []);

  useEffect(() => wake(), [wake]);

  const title = state.item?.title ?? "Nothing playing";
  const paused = Boolean(state.item) && !state.playing && !handle.error;

  return (

    <div
      className="relative h-full w-full overflow-hidden bg-black"
      onMouseMove={wake}
      onTouchStart={wake}
    >

      <video ref={setVideo} className="h-full w-full" playsInline crossOrigin="anonymous" />

      {cueText && !paused && (

        <div className="pointer-events-none absolute inset-x-0 bottom-[5.5rem] z-30 flex justify-center px-6 sm:bottom-28">

          <div className="max-w-3xl rounded-md bg-black/40 px-3 py-1.5 text-center text-sm leading-snug font-normal whitespace-pre-line text-white shadow-lg backdrop-blur-md sm:text-base">

            {cueText}

          </div>

        </div>

      )}

      {handle.buffering && !handle.error && state.playing && (

        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">

          <Loader2 className="size-8 animate-spin text-white/80" />

        </div>

      )}

      {muted && !handle.error && state.playing && (

        <Button className="absolute top-1/2 left-1/2 z-20 -translate-x-1/2 -translate-y-1/2 shadow-lg" onClick={() => setMuted(false)}>

          <Volume2 />
          Tap to unmute

        </Button>

      )}

      {paused && state.item && (

        <PauseOverlay
          item={state.item}
          onResume={() => {

            setMuted(false);
            play();

          }}
        />

      )}

      {/* Skipping is a deliberate seek, so one person skipping takes the room with them (§5.6). */}
      {intro && !paused && (

        <Button
          variant="secondary"
          className="absolute right-6 bottom-28 z-20 shadow-lg"
          onClick={() => seek(intro.endMs)}
        >

          <SkipForward />
          Skip intro

        </Button>

      )}

      {handle.error && (

        <div className="absolute inset-0 z-20 flex flex-col items-center justify-center gap-4 bg-black/85 p-6 text-center">

          <p className="text-destructive text-sm">{handle.error}</p>

          <div className="flex gap-2">

            <Button variant="secondary" onClick={handle.retry}>Try again</Button>

            {handle.qualities.length > 1 && (

              <p className="text-muted-foreground self-center text-xs">or pick another quality below</p>

            )}

          </div>

        </div>

      )}

      <div
        className={cn(
          "absolute inset-x-0 top-0 z-20 bg-gradient-to-b from-black/80 to-transparent px-4 pt-3 pb-10 transition-opacity duration-200",
          chromeVisible && !paused ? "opacity-100" : "pointer-events-none opacity-0",
        )}
      >

        <div className="grid grid-cols-[1fr_auto_1fr] items-start gap-3">

          <div className="flex justify-start">

            <Button
              variant="secondary"
              size="sm"
              className="opacity-90 hover:opacity-100"
              onClick={onBrowse}
            >

              <LibraryBig />
              Browse

            </Button>

          </div>

          <div className="min-w-0 max-w-md text-center">

            <p className="truncate text-sm font-medium text-white">{title}</p>

            {state.item?.caption && <p className="truncate text-xs text-white/60">{state.item.caption}</p>}

          </div>

          <div className="flex justify-end">

            <Presence className="shrink-0" />

          </div>

        </div>

      </div>

      <div
        className={cn(
          "absolute inset-x-0 bottom-0 z-20 bg-gradient-to-t from-black/85 to-transparent pt-12 pb-3 transition-opacity duration-200",
          chromeVisible && !paused ? "opacity-100" : "pointer-events-none opacity-0",
        )}
      >

        <Controls
          handle={handle}
          volume={volume}
          muted={muted}
          onVolume={setVolume}
          onMuted={setMuted}
        />

      </div>

    </div>

  );

}
