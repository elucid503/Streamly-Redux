import { useRoom } from "@/hooks/useRoom";
import type { Participant } from "@/lib/types";

interface Presence {

  participants: Participant[];
  others: Participant[];

  lastActor: { name: string; action: string } | null;

}

// Presence is whoever holds a live connection, which is not the same set as the voice channel (see _docs/DESIGN.md §3).
export function useParticipants(): Presence {

  const { participants, state, session } = useRoom();

  return {

    participants,
    others: participants.filter((participant) => participant.userId !== session.user.id),

    lastActor: state.lastActor ? { name: state.lastActor.name, action: state.lastActor.action } : null,

  };

}
