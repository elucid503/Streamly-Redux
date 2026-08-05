import { useEffect, useRef, useState } from "react";
import { ListVideo, Pause, Play, SkipForward, Volume2, VolumeX } from "lucide-react";

import { QueueSheet } from "@/components/browse/QueueSheet";
import { QualityMenu } from "@/components/player/QualityMenu";
import { SourceMenu } from "@/components/player/SourceMenu";
import { SubtitleMenu } from "@/components/player/SubtitleMenu";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";

import { useRoom } from "@/hooks/useRoom";
import type { PlayerHandle } from "@/hooks/usePlayer";
import { formatTime } from "@/lib/clock";
import { cn } from "@/lib/cn";

interface ControlsProps {

  handle: PlayerHandle;

  volume: number;
  muted: boolean;

  onVolume: (volume: number) => void;
  onMuted: (muted: boolean) => void;

}

const volumeHideDelayMs = 280;

export function Controls({ handle, volume, muted, onVolume, onMuted }: ControlsProps) {

  const { state, play, pause, seek, next } = useRoom();

  const [scrub, setScrub] = useState<number | null>(null);
  const [volumeOpen, setVolumeOpen] = useState(false);
  const [queueOpen, setQueueOpen] = useState(false);

  const hideTimer = useRef<number | null>(null);
  const queueTriggerRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {

    return () => {

      if (hideTimer.current !== null) {

        window.clearTimeout(hideTimer.current);

      }

    };

  }, []);

  const openVolume = () => {

    if (hideTimer.current !== null) {

      window.clearTimeout(hideTimer.current);
      hideTimer.current = null;

    }

    setVolumeOpen(true);

  };

  const scheduleCloseVolume = () => {

    if (hideTimer.current !== null) {

      window.clearTimeout(hideTimer.current);

    }

    hideTimer.current = window.setTimeout(() => {

      setVolumeOpen(false);
      hideTimer.current = null;

    }, volumeHideDelayMs);

  };

  const queueLength = state.queue.length;

  const live = handle.live || state.item?.kind === "channel";
  // Queue is up-next only (playing title is already consumed) — skip-forward when something remains.
  const showNext = state.item?.kind === "vod" && queueLength > 0;

  const position = scrub ?? handle.position;

  return (

    <div className="flex w-full flex-col gap-2">

      {!live && (

        <div className="grid w-full grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-2 px-4">

          <span className="font-mono text-[11px] font-normal tabular-nums tracking-tight text-white/75">

            {formatTime(position)}

          </span>

          <Slider
            className="min-w-0"
            value={[position]}
            max={Math.max(handle.duration, 1)}
            step={0.1}
            onValueChange={([value]) => setScrub(value)}
            onValueCommit={([value]) => {

              setScrub(null);
              seek(value * 1000);

            }}
          />

          <span className="font-mono text-[11px] font-normal tabular-nums tracking-tight text-white/75">

            {formatTime(handle.duration)}

          </span>

        </div>

      )}

      <div className="flex items-center gap-1 px-3">

        {live && (

          <Badge variant="destructive" className="mr-1 gap-1.5">

            <span className="size-1.5 rounded-full bg-current" />
            LIVE

          </Badge>

        )}

        <Button variant="ghost" size="icon" className="text-white hover:bg-white/15 hover:text-white" onClick={state.playing ? pause : play}>

          {state.playing ? <Pause /> : <Play />}

        </Button>

        {showNext && (

          <Button variant="ghost" size="icon" className="text-white hover:bg-white/15 hover:text-white" onClick={() => next()}>

            <SkipForward />

          </Button>

        )}

        <div
          className="ml-1 flex items-center"
          onMouseEnter={openVolume}
          onMouseLeave={scheduleCloseVolume}
          onFocusCapture={openVolume}
          onBlurCapture={(event) => {

            if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {

              scheduleCloseVolume();

            }

          }}
        >

          <Button
            variant="ghost"
            size="icon-sm"
            className="text-white hover:bg-white/15 hover:text-white"
            onClick={() => onMuted(!muted)}
          >

            {muted || volume === 0 ? <VolumeX /> : <Volume2 />}

          </Button>

          <div
            className={cn(
              "overflow-hidden transition-[max-width,opacity,margin] duration-200 ease-out",
              volumeOpen ? "ml-2 max-w-28 opacity-100" : "ml-0 max-w-0 opacity-0",
            )}
          >

            <Slider
              className="w-24"
              value={[muted ? 0 : volume]}
              max={1}
              step={0.05}
              onValueChange={([value]) => {

                onVolume(value);
                onMuted(value === 0);

              }}
            />

          </div>

        </div>

        <div className="ml-auto flex items-center gap-1">

          <div className="relative">

            <Button
              ref={queueTriggerRef}
              variant="ghost"
              size="icon"
              className="text-white hover:bg-white/15 hover:text-white"
              onClick={() => setQueueOpen(true)}
            >

              <ListVideo />

            </Button>

            {state.queue.length > 0 && (

              <span className="bg-primary text-primary-foreground absolute -top-0.5 -right-0.5 flex size-4 items-center justify-center rounded-full text-[9px] font-semibold">

                {state.queue.length}

              </span>

            )}

          </div>

          <QueueSheet
            open={queueOpen}
            onOpenChange={setQueueOpen}
            onPlay={() => undefined}
            showTrigger={false}
            anchorRef={queueTriggerRef}
          />

          {live && <SourceMenu handle={handle} />}

          {!live && (

            <>

              <SubtitleMenu />
              <QualityMenu handle={handle} />

            </>

          )}

        </div>

      </div>

    </div>

  );

}
