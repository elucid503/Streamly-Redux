import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Hls from "hls.js";

import { useRoom } from "@/hooks/useRoom";
import { driftToleranceMs, expectedPosition } from "@/lib/clock";
import type { Quality } from "@/lib/types";

const correctionIntervalMs = 2000;

// A live viewer sits at their own buffer depth; only a gap wider than this is worth a jump (see _docs/DESIGN.md §4.4).
const liveEdgeToleranceSeconds = 6;

// Deliberate room seeks use the same 2s bar as periodic correction (§4.2). A 400ms bar
// was firing on play/pause (anchorAt rewrites) and stacking micro-seeks that desynced A/V.
const deliberateSeekToleranceMs = driftToleranceMs;

// Cap how long we treat a seek as in-flight if "seeked" never arrives.
const seekLockMs = 1500;

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
  const hlsRef = useRef<Hls | null>(null);

  const seekingRef = useRef(false);
  const pendingSeekRef = useRef<number | null>(null);
  const seekUnlockTimer = useRef<number | null>(null);
  const wasStallingRef = useRef(false);

  // Stable snapshots so the correction timer is not torn down on every room poll.
  const stateRef = useRef(state);
  const serverNowRef = useRef(serverNow);
  const liveRef = useRef(false);

  stateRef.current = state;
  serverNowRef.current = serverNow;

  const playback = state.playback;
  const live = state.item?.kind === "channel";

  liveRef.current = Boolean(live);

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
    seekingRef.current = false;
    pendingSeekRef.current = null;

    if (seekUnlockTimer.current !== null) {

      window.clearTimeout(seekUnlockTimer.current);
      seekUnlockTimer.current = null;

    }

  }, [state.item?.id, state.item?.season, state.item?.episode]);

  // Loading reads the room position once; every later change is handled by correction rather than by reloading.
  const positionAtLoad = useRef(0);

  positionAtLoad.current = expectedPosition(state, serverNow()) / 1000;

  const hardSeek = useCallback((element: HTMLVideoElement, seconds: number) => {

    if (!Number.isFinite(seconds) || seconds < 0) {

      return;

    }

    // Coalesce: keep the latest target while a seek is already in flight.
    // Stacked currentTime writes are a common progressive-MP4 A/V desync trigger.
    if (seekingRef.current) {

      pendingSeekRef.current = seconds;
      return;

    }

    const deltaMs = Math.abs(element.currentTime - seconds) * 1000;

    if (deltaMs < 80) {

      return;

    }

    seekingRef.current = true;

    if (seekUnlockTimer.current !== null) {

      window.clearTimeout(seekUnlockTimer.current);

    }

    let settled = false;

    const finish = () => {

      if (settled) {

        return;

      }

      settled = true;

      element.removeEventListener("seeked", finish);
      element.removeEventListener("error", finish);

      if (seekUnlockTimer.current !== null) {

        window.clearTimeout(seekUnlockTimer.current);
        seekUnlockTimer.current = null;

      }

      seekingRef.current = false;

      const pending = pendingSeekRef.current;

      pendingSeekRef.current = null;

      if (pending !== null && Math.abs(element.currentTime - pending) * 1000 >= 80) {

        hardSeek(element, pending);

      }

    };

    element.addEventListener("seeked", finish);
    element.addEventListener("error", finish);
    seekUnlockTimer.current = window.setTimeout(finish, seekLockMs);

    try {

      element.currentTime = seconds;
      setPosition(seconds);

    } catch {

      finish();

    }

  }, []);

  useEffect(() => {

    if (!video || !source) {

      return;

    }

    setError(null);
    setBuffering(true);
    seekingRef.current = false;
    pendingSeekRef.current = null;

    const startAt = live ? 0 : positionAtLoad.current;

    const seekOnReady = () => {

      if (!live && startAt > 1) {

        hardSeek(video, startAt);

      }

    };

    if (playback?.kind === "hls" && !video.canPlayType("application/vnd.apple.mpegurl") && Hls.isSupported()) {

      const hls = new Hls({
        lowLatencyMode: false,
        backBufferLength: 30,
        maxBufferHole: 0.5,
      });

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

          } else if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {

            hls.recoverMediaError();

          } else {

            hls.startLoad();

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

  }, [video, source, playback?.kind, live, attempt, hardSeek]);

  useEffect(() => {

    if (!video || !source) {

      return;

    }

    if (state.playing) {

      // Don't call play() mid-seek — resume once the element lands.
      if (seekingRef.current) {

        const resume = () => {

          void video.play().catch(() => setBuffering(true));

        };

        video.addEventListener("seeked", resume, { once: true });

        return () => video.removeEventListener("seeked", resume);

      }

      void video.play().catch(() => setBuffering(true));

    } else {

      video.pause();

    }

  }, [video, source, state.playing]);

  // Deliberate room seeks rewrite anchorMs; apply those immediately (§4.1 / §4.2).
  // Key only on anchorMs — play rewrites anchorAt every resume and must not force a seek.
  useEffect(() => {

    if (!video || !source || live) {

      return;

    }

    const apply = () => {

      const room = stateRef.current;
      const target = expectedPosition(room, serverNowRef.current()) / 1000;

      if (Math.abs(video.currentTime - target) * 1000 > deliberateSeekToleranceMs) {

        hardSeek(video, target);

      }

    };

    if (video.readyState >= 1) {

      apply();
      return;

    }

    video.addEventListener("loadedmetadata", apply, { once: true });

    return () => video.removeEventListener("loadedmetadata", apply);

  }, [video, source, live, state.anchorMs, hardSeek]);

  const correct = useCallback(() => {

    const element = video;

    if (!element || element.readyState < 2 || seekingRef.current) {

      return;

    }

    const room = stateRef.current;
    const isLive = liveRef.current;

    if (isLive) {

      if (!room.playing) {

        return;

      }

      const edge = hlsRef.current?.liveSyncPosition
        ?? (element.seekable.length > 0 ? element.seekable.end(element.seekable.length - 1) : 0);

      if (edge > 0 && edge - element.currentTime > liveEdgeToleranceSeconds) {

        hardSeek(element, edge);

      }

      return;

    }

    if (!room.playing) {

      return;

    }

    const target = expectedPosition(room, serverNowRef.current()) / 1000;

    if (Math.abs(element.currentTime - target) * 1000 > driftToleranceMs) {

      hardSeek(element, target);

    }

  }, [video, hardSeek]);

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

    const onWaiting = () => {

      wasStallingRef.current = true;
      setBuffering(true);

    };

    const onPlaying = () => {

      setBuffering(false);
      retries.current = 0;

      // After a local stall, recompute expected and jump if needed (§4.1).
      if (wasStallingRef.current) {

        wasStallingRef.current = false;
        correct();

      }

    };

    const onTime = () => {

      if (!seekingRef.current) {

        setPosition(video.currentTime);

      }

    };

    const onDuration = () => setDuration(Number.isFinite(video.duration) ? video.duration : 0);

    // Rolling to the next episode is the queue's whole purpose, including across a season boundary (§4.6).
    const onEnded = () => {

      if (!liveRef.current) {

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

  }, [video, correct, next]);

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
