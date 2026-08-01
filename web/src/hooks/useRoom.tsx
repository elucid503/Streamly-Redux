import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";

import { Sync, type ConnectionStatus } from "@/lib/sync";
import type { Item, Notice, Participant, RoomState, SubtitleTrack } from "@/lib/types";
import type { Session } from "@/lib/sdk";

const emptyState: RoomState = {

  item: null,
  playback: null,

  playing: false,

  anchorMs: 0,
  anchorAt: 0,

  subtitle: null,

  queue: [],
  queueIndex: 0,

  lastActor: null,

};

const noticeLifetimeMs = 6000;

interface RoomValue {

  state: RoomState;
  pendingItem: Item | null;
  participants: Participant[];
  notices: Notice[];

  status: ConnectionStatus;
  session: Session;

  joined: boolean;
  arrivedMidSession: boolean;

  serverNow: () => number;

  play: () => void;
  pause: () => void;
  seek: (positionMs: number) => void;

  next: () => void;
  prev: () => void;

  setItem: (item: Item | null) => void;
  setSubtitle: (track: SubtitleTrack | null) => void;

  enqueue: (item: Item) => void;
  dequeue: (index: number) => void;
  reorder: (index: number, to: number) => void;

}

const RoomContext = createContext<RoomValue | null>(null);

export function RoomProvider({ session, children }: { session: Session; children: React.ReactNode }) {

  const [state, setState] = useState<RoomState>(emptyState);
  const [pendingItem, setPendingItem] = useState<Item | null>(null);
  const [participants, setParticipants] = useState<Participant[]>([]);
  const [notices, setNotices] = useState<Notice[]>([]);
  const [status, setStatus] = useState<ConnectionStatus>("connecting");
  const [joined, setJoined] = useState(false);
  const [arrivedMidSession, setArrivedMidSession] = useState(false);

  const syncRef = useRef<Sync | null>(null);
  const noticeId = useRef(0);
  const firstState = useRef(true);

  useEffect(() => {

    const sync = new Sync(session.config.clientId, session.instanceId, session.socketTicket, {

      // Whether the room was already watching at the moment of joining decides which screen opens (see _docs/DESIGN.md §6.1).
      onState: (next) => {

        if (firstState.current) {

          firstState.current = false;

          setArrivedMidSession(Boolean(next.item));
          setJoined(true);

        }

        setPendingItem((pending) => pending && itemKey(pending) === itemKey(next.item) ? null : pending);
        setState(next);

      },
      onParticipants: setParticipants,

      onNotice: (notice) => {

        if (notice.kind === "error") {

          setPendingItem(null);

        }

        noticeId.current += 1;

        const entry = { id: noticeId.current, ...notice };

        setNotices((current) => [...current, entry]);

        window.setTimeout(() => {

          setNotices((current) => current.filter((existing) => existing.id !== entry.id));

        }, noticeLifetimeMs);

      },

      onStatus: setStatus,

    });

    syncRef.current = sync;

    sync.start();

    return () => {

      sync.stop();
      syncRef.current = null;

    };

  }, [session.config.clientId, session.instanceId, session.socketTicket]);

  const serverNow = useCallback(() => syncRef.current?.serverNow() ?? Date.now(), []);

  const value = useMemo<RoomValue>(() => {

    return {

      state,
      pendingItem,
      participants,
      notices,

      status,
      session,

      joined,
      arrivedMidSession,

      serverNow,

      play: () => syncRef.current?.send({ type: "control", action: "play" }),
      pause: () => syncRef.current?.send({ type: "control", action: "pause" }),
      seek: (positionMs: number) => syncRef.current?.send({ type: "control", action: "seek", positionMs: Math.round(positionMs) }),

      next: () => syncRef.current?.send({ type: "control", action: "next" }),
      prev: () => syncRef.current?.send({ type: "control", action: "prev" }),

      setItem: (item: Item | null) => {

        if (syncRef.current?.send({ type: "control", action: "setItem", item })) {

          setPendingItem(item);

        }

      },
      setSubtitle: (track: SubtitleTrack | null) => {

        // Apply locally first so the selecting client does not wait on the room round-trip.
        setState((current) => ({ ...current, subtitle: track }));
        syncRef.current?.send({ type: "control", action: "setSubtitle", track });

      },

      enqueue: (item: Item) => syncRef.current?.send({ type: "queue", op: "add", item }),
      dequeue: (index: number) => syncRef.current?.send({ type: "queue", op: "remove", index }),
      reorder: (index: number, to: number) => syncRef.current?.send({ type: "queue", op: "move", index, to }),

    };

  }, [state, pendingItem, participants, notices, status, session, joined, arrivedMidSession, serverNow]);

  return (

    <RoomContext.Provider value={value}>

      {children}

    </RoomContext.Provider>

  );

}

function itemKey(item: Item | null): string {

  if (!item) {

    return "";

  }

  return `${item.kind}:${item.id}:${item.season ?? 0}:${item.episode ?? 0}`;

}

export function useRoom(): RoomValue {

  const value = useContext(RoomContext);

  if (!value) {

    throw new Error("useRoom must be used inside RoomProvider");

  }

  return value;

}
