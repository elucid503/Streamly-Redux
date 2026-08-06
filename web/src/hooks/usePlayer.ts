import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Hls from "hls.js";

import { useRoom } from "@/hooks/useRoom";
import { driftToleranceMs, expectedPosition } from "@/lib/clock";
import type { Quality } from "@/lib/types";

const correctionIntervalMs = 2000;

// A live viewer sits at their own buffer depth; only a gap wider than this is worth a jump.
const liveEdgeToleranceSeconds = 6;

// Deliberate room seeks use the same 2s bar as periodic correction.
const deliberateSeekToleranceMs = driftToleranceMs;

// Cap how long we treat a seek as in-flight if "seeked" never arrives.
const seekLockMs = 1500;

// Soft recoveries (startLoad / recoverMediaError) before a full player remount.
const maxSoftRetries = 3;

// Full HLS remounts after soft recovery is exhausted. Matches what navigating away/back does.
const maxHardRetries = 2;

// How long currentTime may freeze while the room is playing before we force recovery.
const stallTimeoutMs = 12_000;

// Live TV that sits buffering this long advances to the next catalog source.
const liveBufferFailoverMs = 10_000;

// Ignore tiny currentTime jitter when deciding whether playback made progress.
const progressEpsilonSeconds = 0.05;

export interface PlayerHandle {

  position: number;
  duration: number;

  buffering: boolean;
  error: string | null;

  live: boolean;

  sourceIndex: number;
  sourceCount: number;

  qualities: Quality[];
  quality: Quality | null;

  selectQuality: (label: string) => void;
  selectSource: (index: number) => void;
  retry: () => void;

}

export function usePlayer(video: HTMLVideoElement | null): PlayerHandle {

  const { state, serverNow, next, setSource, nextSource } = useRoom();

  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [buffering, setBuffering] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [qualityLabel, setQualityLabel] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  const softRetries = useRef(0);
  const hardRetries = useRef(0);
  const mediaRecoveries = useRef(0);
  const hlsRef = useRef<Hls | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);

  const seekingRef = useRef(false);
  const pendingSeekRef = useRef<number | null>(null);
  const seekUnlockTimer = useRef<number | null>(null);
  const wasStallingRef = useRef(false);
  const recoveringRef = useRef(false);

  // Stall detection: only arm after the first frame so initial load is not treated as a hang.
  const hadFirstFrameRef = useRef(false);
  const lastMediaTimeRef = useRef(0);
  const lastProgressAtRef = useRef(0);

  // Live buffer watchdog: fire nextSource once per source index after sustained buffering.
  const bufferStartedAtRef = useRef<number | null>(null);
  const failoverForSourceRef = useRef<number | null>(null);

  // Stable snapshots so the correction timer is not torn down on every room poll.
  const stateRef = useRef(state);
  const serverNowRef = useRef(serverNow);
  const liveRef = useRef(false);
  const nextSourceRef = useRef(nextSource);

  stateRef.current = state;
  serverNowRef.current = serverNow;
  videoRef.current = video;
  nextSourceRef.current = nextSource;

  const playback = state.playback;
  const live = state.item?.kind === "channel";

  liveRef.current = Boolean(live);

  const sourceIndex = playback?.sourceIndex ?? 0;
  const sourceCount = playback?.sourceCount ?? 0;

  const qualities = useMemo(() => playback?.qualities ?? [], [playback]);

  const quality = useMemo(() => {

    if (qualities.length === 0) {

      return null;

    }

    return qualities.find((entry) => entry.label === qualityLabel) ?? qualities.find((entry) => entry.url === playback?.url) ?? qualities[0];

  }, [qualities, qualityLabel, playback]);

  const source = quality?.url ?? playback?.url ?? null;

  const resetSoftRecovery = useCallback(() => {

    softRetries.current = 0;
    mediaRecoveries.current = 0;
    recoveringRef.current = false;

  }, []);

  const remountPlayer = useCallback(() => {

    resetSoftRecovery();
    setError(null);
    setBuffering(true);
    setAttempt((current) => current + 1);

  }, [resetSoftRecovery]);

  // Quality is personal, so a new item resets the choice rather than carrying one title's rendition into the next (§4.7).
  useEffect(() => {

    setQualityLabel(null);
    resetSoftRecovery();
    hardRetries.current = 0;
    seekingRef.current = false;
    pendingSeekRef.current = null;
    hadFirstFrameRef.current = false;
    lastMediaTimeRef.current = 0;
    lastProgressAtRef.current = Date.now();
    bufferStartedAtRef.current = null;
    failoverForSourceRef.current = null;

    if (seekUnlockTimer.current !== null) {

      window.clearTimeout(seekUnlockTimer.current);
      seekUnlockTimer.current = null;

    }

  }, [state.item?.id, state.item?.season, state.item?.episode, resetSoftRecovery]);

  // A room source change is a fresh stream — clear local recovery state.
  useEffect(() => {

    bufferStartedAtRef.current = null;
    failoverForSourceRef.current = null;
    resetSoftRecovery();
    hardRetries.current = 0;
    setError(null);
    setBuffering(true);

  }, [sourceIndex, resetSoftRecovery]);

  // Arm the live buffer timer whenever we enter a buffering state.
  useEffect(() => {

    if (!live) {

      bufferStartedAtRef.current = null;
      return;

    }

    if (buffering) {

      if (bufferStartedAtRef.current === null) {

        bufferStartedAtRef.current = Date.now();

      }

      return;

    }

    bufferStartedAtRef.current = null;

  }, [buffering, live]);

  // Loading reads the room position once; every later change is handled by correction rather than by reloading.
  const positionAtLoad = useRef(0);

  positionAtLoad.current = expectedPosition(state, serverNow()) / 1000;

  const hardSeek = useCallback((element: HTMLVideoElement, seconds: number) => {

    if (!Number.isFinite(seconds) || seconds < 0) {

      return;

    }

    // Keeps the latest target while a seek is already in flight.
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

      // Seeks mid-playback (live edge catch-up, post-stall correct) can leave the element paused without re-running the play effect — we should resume if the room wants it.
      if (stateRef.current.playing && element.paused) {

        void element.play().catch(() => setBuffering(true));

      }

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

  const liveEdge = useCallback((element: HTMLVideoElement) => {

    return hlsRef.current?.liveSyncPosition
      ?? (element.seekable.length > 0 ? element.seekable.end(element.seekable.length - 1) : 0);

  }, []);

  const snapToLiveEdge = useCallback((element: HTMLVideoElement) => {

    const edge = liveEdge(element);

    if (edge > 0 && edge - element.currentTime > 1) {

      hardSeek(element, edge);

    }

  }, [hardSeek, liveEdge]);

  const ensurePlaying = useCallback((element: HTMLVideoElement) => {

    if (stateRef.current.playing && element.paused) {

      void element.play().catch(() => setBuffering(true));

    }

  }, []);

  // Escalating recovery: soft HLS recovery → full remount (what Browse-and-back does) → surface error.
  const recoverPlayback = useCallback((reason: "network" | "media" | "stall") => {

    const element = videoRef.current;

    if (!element || recoveringRef.current) {

      return;

    }

    recoveringRef.current = true;
    setBuffering(true);
    setError(null);

    const hls = hlsRef.current;
    const finishSoft = () => {

      // Allow another attempt after a beat so a single hung recovery cannot loop instantly.
      window.setTimeout(() => {

        recoveringRef.current = false;

      }, 1500);

    };

    if (hls && softRetries.current < maxSoftRetries) {

      softRetries.current += 1;

      if (reason === "media") {

        if (mediaRecoveries.current === 0) {

          mediaRecoveries.current = 1;
          hls.recoverMediaError();

        } else {

          hls.swapAudioCodec();
          hls.recoverMediaError();

        }

      } else {

        // Network blips and silent stalls: restart fragment loading and rejoin live if needed.
        hls.startLoad();

        if (liveRef.current) {

          snapToLiveEdge(element);

        }

      }

      ensurePlaying(element);
      lastProgressAtRef.current = Date.now();
      finishSoft();
      return;

    }

    // Native HLS (Safari) has no soft path worth keeping — remount the source.
    if (!hls && softRetries.current < maxSoftRetries) {

      softRetries.current += 1;

      if (liveRef.current) {

        snapToLiveEdge(element);

      }

      ensurePlaying(element);
      lastProgressAtRef.current = Date.now();
      finishSoft();
      return;

    }

    if (hardRetries.current < maxHardRetries) {

      hardRetries.current += 1;
      recoveringRef.current = false;
      remountPlayer();
      return;

    }

    // Exhausted local recovery on live TV: advance room source before showing a hard error.
    // Source switching is circular on the server (with a cycle timeout), so any multi-source
    // channel can still request next even when already on the last catalog entry.
    const room = stateRef.current;
    const current = room.playback?.sourceIndex ?? 0;
    const total = room.playback?.sourceCount ?? 0;

    if (liveRef.current && total > 1 && failoverForSourceRef.current !== current) {

      failoverForSourceRef.current = current;
      recoveringRef.current = false;
      nextSourceRef.current();
      return;

    }

    recoveringRef.current = false;
    setError("This source stopped responding");

  }, [ensurePlaying, remountPlayer, snapToLiveEdge]);

  useEffect(() => {

    if (!video || !source) {

      return;

    }

    setError(null);
    setBuffering(true);

    seekingRef.current = false;
    pendingSeekRef.current = null;
    recoveringRef.current = false;
    hadFirstFrameRef.current = false;
    lastMediaTimeRef.current = 0;
    lastProgressAtRef.current = Date.now();

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
        // Live IPTV manifests often have small discontinuities; a tight hole leaves the playhead stuck.
        maxBufferHole: 1.5,
        nudgeMaxRetry: 10,
        highBufferWatchdogPeriod: 1,
        liveSyncDurationCount: 3,
        liveMaxLatencyDurationCount: 12,
        fragLoadingMaxRetry: 6,
        levelLoadingMaxRetry: 4,
        manifestLoadingMaxRetry: 4,
      });

      hlsRef.current = hls;

      hls.on(Hls.Events.MANIFEST_PARSED, seekOnReady);

      hls.on(Hls.Events.ERROR, (_event, data) => {

        if (!data.fatal) {

          // bufferStalledError is non-fatal; hls.js nudges, but if the playhead is already frozen
          // long enough our stall watchdog will escalate. Avoid double-firing soft recovery here.
          return;

        }

        if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {

          recoverPlayback("network");
          return;

        }

        if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {

          recoverPlayback("media");
          return;

        }

        // Mux / other fatal: skip soft path and remount.
        recoverPlayback("stall");

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

  }, [video, source, playback?.kind, live, attempt, hardSeek, recoverPlayback]);

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

    if (!element || element.readyState < 2 || seekingRef.current || recoveringRef.current) {

      return;

    }

    const room = stateRef.current;
    const isLive = liveRef.current;

    if (isLive) {

      if (!room.playing) {

        return;

      }

      const edge = liveEdge(element);

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

  }, [video, hardSeek, liveEdge]);

  useEffect(() => {

    if (!video) {

      return;

    }

    const timer = window.setInterval(correct, correctionIntervalMs);

    // Stall watchdog: currentTime froze while the room expects playback.
    const stallTimer = window.setInterval(() => {

      if (!hadFirstFrameRef.current || seekingRef.current || recoveringRef.current) {

        return;

      }

      if (!stateRef.current.playing || error) {

        return;

      }

      if (Date.now() - lastProgressAtRef.current < stallTimeoutMs) {

        return;

      }

      recoverPlayback("stall");

    }, 2000);

    const onVisible = () => {

      if (document.visibilityState === "visible") {

        correct();

        // Background tabs often freeze live buffers; rejoin the edge on return.
        if (liveRef.current && videoRef.current && stateRef.current.playing) {

          snapToLiveEdge(videoRef.current);
          ensurePlaying(videoRef.current);

        }

      }

    };

    const onWaiting = () => {

      wasStallingRef.current = true;
      setBuffering(true);

      if (bufferStartedAtRef.current === null) {

        bufferStartedAtRef.current = Date.now();

      }

    };

    const onPlaying = () => {

      setBuffering(false);
      bufferStartedAtRef.current = null;
      hadFirstFrameRef.current = true;
      lastProgressAtRef.current = Date.now();
      lastMediaTimeRef.current = video.currentTime;

      // A successful frame means the current recovery ladder can start fresh.
      softRetries.current = 0;
      mediaRecoveries.current = 0;
      hardRetries.current = 0;
      recoveringRef.current = false;

      // After a local stall, recompute expected and jump if needed (§4.1).
      if (wasStallingRef.current) {

        wasStallingRef.current = false;
        correct();

      }

    };

    // Sustained live buffering advances to the next catalog source for the room.
    const bufferTimer = window.setInterval(() => {

      if (!liveRef.current || !stateRef.current.playing || error) {

        return;

      }

      const started = bufferStartedAtRef.current;

      if (started === null || Date.now() - started < liveBufferFailoverMs) {

        return;

      }

      const room = stateRef.current;
      const current = room.playback?.sourceIndex ?? 0;
      const total = room.playback?.sourceCount ?? 0;

      // Circular failover (server-side wrap + cycle timeout) — allow next from any source.
      if (total <= 1 || failoverForSourceRef.current === current) {

        return;

      }

      failoverForSourceRef.current = current;
      bufferStartedAtRef.current = null;
      nextSourceRef.current();

    }, 1000);

    const onTime = () => {

      if (!seekingRef.current) {

        setPosition(video.currentTime);

      }

      // Only count real forward progress so a stuck playhead with periodic timeupdate still triggers recovery.
      if (Math.abs(video.currentTime - lastMediaTimeRef.current) >= progressEpsilonSeconds) {

        lastMediaTimeRef.current = video.currentTime;
        lastProgressAtRef.current = Date.now();

        if (hadFirstFrameRef.current && !video.paused) {

          setBuffering(false);
          bufferStartedAtRef.current = null;

        }

      }

    };

    const onDuration = () => setDuration(Number.isFinite(video.duration) ? video.duration : 0);

    // Rolling to the next episode is the queue's whole purpose, including across a season boundary.
    const onEnded = () => {

      if (!liveRef.current) {

        next(stateRef.current.item);

      }

    };

    // Element-level errors (native HLS / progressive) never hit the hls.js handler.
    const onError = () => {

      if (!hlsRef.current) {

        recoverPlayback("network");

      }

    };

    document.addEventListener("visibilitychange", onVisible);

    video.addEventListener("waiting", onWaiting);
    video.addEventListener("playing", onPlaying);
    video.addEventListener("timeupdate", onTime);
    video.addEventListener("durationchange", onDuration);
    video.addEventListener("ended", onEnded);
    video.addEventListener("error", onError);

    return () => {

      window.clearInterval(timer);
      window.clearInterval(stallTimer);
      window.clearInterval(bufferTimer);

      document.removeEventListener("visibilitychange", onVisible);

      video.removeEventListener("waiting", onWaiting);
      video.removeEventListener("playing", onPlaying);
      video.removeEventListener("timeupdate", onTime);
      video.removeEventListener("durationchange", onDuration);
      video.removeEventListener("ended", onEnded);
      video.removeEventListener("error", onError);

    };

  }, [video, correct, next, recoverPlayback, snapToLiveEdge, ensurePlaying, error]);

  const selectQuality = useCallback((label: string) => {

    resetSoftRecovery();
    hardRetries.current = 0;

    setQualityLabel(label);

  }, [resetSoftRecovery]);

  const selectSource = useCallback((index: number) => {

    resetSoftRecovery();
    hardRetries.current = 0;
    failoverForSourceRef.current = null;
    bufferStartedAtRef.current = null;
    setError(null);
    setBuffering(true);
    setSource(index);

  }, [resetSoftRecovery, setSource]);

  const retry = useCallback(() => {

    resetSoftRecovery();
    hardRetries.current = 0;

    setError(null);
    setBuffering(true);
    setAttempt((current) => current + 1);

  }, [resetSoftRecovery]);

  return {

    position,
    duration,

    buffering,
    error,

    live: Boolean(live),

    sourceIndex,
    sourceCount,

    qualities,
    quality,

    selectQuality,
    selectSource,
    retry,

  };

}
