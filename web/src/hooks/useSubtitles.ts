import { useEffect, useState } from "react";

import { loadSubtitleCues } from "@/lib/subtitles";
import { activeCueText, type Cue } from "@/lib/vtt";
import type { SubtitleTrack } from "@/lib/types";

// Drive cues from the video clock directly — room position state is too coarse and easy to desync.
export function useSubtitles(track: SubtitleTrack | null, video: HTMLVideoElement | null): string | null {

  const [cues, setCues] = useState<Cue[]>([]);
  const [text, setText] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {

    if (!track) {

      setCues([]);
      setText(null);
      setError(null);
      return;

    }

    let cancelled = false;

    setCues([]);
    setText(null);
    setError(null);

    void loadSubtitleCues(track)
      .then((parsed) => {

        if (cancelled) {

          return;

        }

        if (parsed.length === 0) {

          console.warn("[streamly] subtitle file produced no cues", track);
          setError("Subtitle file was empty");
          setCues([]);
          return;

        }

        console.info("[streamly] subtitle cues ready", { id: track.id, cues: parsed.length });
        setCues(parsed);

      })
      .catch((caught: unknown) => {

        if (cancelled) {

          return;

        }

        console.error("[streamly] subtitle load failed", caught);
        setError("Could not load subtitles");
        setCues([]);

      });

    return () => {

      cancelled = true;

    };

  }, [track?.id, track?.url]);

  useEffect(() => {

    if (!video || cues.length === 0) {

      setText(null);
      return;

    }

    let frame = 0;

    const tick = () => {

      const next = activeCueText(cues, video.currentTime);

      setText((current) => (current === next ? current : next));

      frame = window.requestAnimationFrame(tick);

    };

    // Also bind media events so we still update if rAF is throttled in the background.
    const onTime = () => {

      const next = activeCueText(cues, video.currentTime);

      setText((current) => (current === next ? current : next));

    };

    video.addEventListener("timeupdate", onTime);
    video.addEventListener("seeked", onTime);
    frame = window.requestAnimationFrame(tick);

    return () => {

      window.cancelAnimationFrame(frame);
      video.removeEventListener("timeupdate", onTime);
      video.removeEventListener("seeked", onTime);

    };

  }, [video, cues]);

  // Surface load failures for the player chrome without coupling to room notices.
  useEffect(() => {

    if (error) {

      console.warn("[streamly]", error);

    }

  }, [error]);

  return text;

}
