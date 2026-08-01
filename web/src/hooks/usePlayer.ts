import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Hls from "hls.js";

import { useRoom } from "@/hooks/useRoom";
import { driftToleranceMs, expectedPosition } from "@/lib/clock";
import type { Quality } from "@/lib/types";

const correctionIntervalMs = 2000;

// A live viewer sits at their own buffer depth; only a gap wider than this is worth a jump (see _docs/DESIGN.md §4.4).
const liveEdgeToleranceSeconds = 6;

// Most mid-playback rendition failures are transient, so a few quiet retries come before offering alternatives (§5.4).
const maxRetries = 3;

export interface PlayerHandle {

  position: number;
  duration: number;

  buffering: boolean;
  error: string | null;

  live: boolean;

  qualities: Quality[];
  quality: Quality | null;

  selectQuality: (label: string) => void;
  retry: () => void;

}

export function usePlayer(video: HTMLVideoElement | null): PlayerHandle {

  const { state, serverNow, next } = useRoom();

  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [buffering, setBuffering] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [qualityLabel, setQualityLabel] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  const retries = useRef(0);

  const playback = state.playback;
  const live = state.item?.kind === "channel";

  const qualities = useMemo(() => playback?.qualities ?? [], [playback]);

  const quality = useMemo(() => {

    if (qualities.length === 0) {

      return null;

    }

    return qualities.find((entry) => entry.label === qualityLabel) ?? qualities.find((entry) => entry.url === playback?.url) ?? qualities[0];

  }, [qualities, qualityLabel, playback]);

  const source = quality?.url ?? playback?.url ?? null;

  // Quality is personal, so a new item resets the choice rather than carrying one title's rendition into the next (§4.7).
  useEffect(() => {

    setQualityLabel(null);
    retries.current = 0;

  }, [state.item?.id, state.item?.season, state.item?.episode]);

  const hlsRef = useRef<Hls | null>(null);

  // Loading reads the room position once; every later change is handled by correction rather than by reloading.
  const positionAtLoad = useRef(0);

  positionAtLoad.current = expectedPosition(state, serverNow()) / 1000;

  useEffect(() => {

    if (!video || !source) {

      return;

    }

    setError(null);
    setBuffering(true);

    const startAt = live ? 0 : positionAtLoad.current;

    const seekOnReady = () => {

      if (!live && startAt > 1) {

        video.currentTime = startAt;

      }

    };

    if (playback?.kind === "hls" && !video.canPlayType("application/vnd.apple.mpegurl") && Hls.isSupported()) {

      const hls = new Hls({ lowLatencyMode: false, backBufferLength: 30 });

      hlsRef.current = hls;

      hls.on(Hls.Events.MANIFEST_PARSED, seekOnReady);

      hls.on(Hls.Events.ERROR, (_event, data) => {

        if (!data.fatal) {

          return;

        }

        if (retries.current < maxRetries) {

          retries.current += 1;

          if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {

            hls.startLoad();

          } else {

            hls.recoverMediaError();

          }

          return;

        }

        setError("This source stopped responding");

      });

      hls.loadSource(source);
      hls.attachMedia(video);

      return () => {

        hls.destroy();
        hlsRef.current = null;

      };

    }

    video.src = source;

    video.addEventListener("loadedmetadata", seekOnReady, { once: true });

    return () => {

      video.removeEventListener("loadedmetadata", seekOnReady);

    };

  }, [video, source, playback?.kind, live, attempt]);

  useEffect(() => {

    if (!video || !source) {

      return;

    }

    if (state.playing) {

      void video.play().catch(() => setBuffering(true));

    } else {

      video.pause();

    }

  }, [video, source, state.playing]);

  // Deliberate room seeks rewrite the anchor; apply them immediately rather than waiting on the drift timer.
  useEffect(() => {

    if (!video || !source || live) {

      return;

    }

    const apply = () => {

      const target = expectedPosition(state, serverNow()) / 1000;

      if (Math.abs(video.currentTime - target) * 1000 > 400) {

        video.currentTime = target;
        setPosition(target);

      }

    };

    if (video.readyState >= 1) {

      apply();
      return;

    }

    video.addEventListener("loadedmetadata", apply, { once: true });

    return () => video.removeEventListener("loadedmetadata", apply);

  }, [video, source, live, state.anchorMs, state.anchorAt, state.playing, serverNow]);

  const correct = useCallback(() => {

    if (!video || video.readyState < 2) {

      return;

    }

    if (live) {

      if (!state.playing) {

        return;

      }

      const edge = hlsRef.current?.liveSyncPosition ?? (video.seekable.length > 0 ? video.seekable.end(video.seekable.length - 1) : 0);

      if (edge > 0 && edge - video.currentTime > liveEdgeToleranceSeconds) {

        video.currentTime = edge;

      }

      return;

    }

    if (!state.playing) {

      return;

    }

    const target = expectedPosition(state, serverNow()) / 1000;

    if (Math.abs(video.currentTime - target) * 1000 > driftToleranceMs) {

      video.currentTime = target;

    }

  }, [video, state, serverNow, live]);

  useEffect(() => {

    if (!video) {

      return;

    }

    const timer = window.setInterval(correct, correctionIntervalMs);

    const onVisible = () => {

      if (document.visibilityState === "visible") {

        correct();

      }

    };

    const onWaiting = () => setBuffering(true);

    const onPlaying = () => {

      setBuffering(false);
      retries.current = 0;

    };

    const onTime = () => setPosition(video.currentTime);
    const onDuration = () => setDuration(Number.isFinite(video.duration) ? video.duration : 0);

    // Rolling to the next episode is the queue's whole purpose, including across a season boundary (§4.6).
    const onEnded = () => {

      if (!live) {

        next();

      }

    };

    document.addEventListener("visibilitychange", onVisible);

    video.addEventListener("waiting", onWaiting);
    video.addEventListener("playing", onPlaying);
    video.addEventListener("timeupdate", onTime);
    video.addEventListener("durationchange", onDuration);
    video.addEventListener("ended", onEnded);

    return () => {

      window.clearInterval(timer);

      document.removeEventListener("visibilitychange", onVisible);

      video.removeEventListener("waiting", onWaiting);
      video.removeEventListener("playing", onPlaying);
      video.removeEventListener("timeupdate", onTime);
      video.removeEventListener("durationchange", onDuration);
      video.removeEventListener("ended", onEnded);

    };

  }, [video, correct, live, next]);

  const selectQuality = useCallback((label: string) => {

    retries.current = 0;

    setQualityLabel(label);

  }, []);

  const retry = useCallback(() => {

    retries.current = 0;

    setError(null);
    setAttempt((current) => current + 1);

  }, []);

  return {

    position,
    duration,

    buffering,
    error,

    live: Boolean(live),

    qualities,
    quality,

    selectQuality,
    retry,

  };

}
