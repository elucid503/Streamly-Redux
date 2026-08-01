import { Clock } from "@/lib/clock";
import type { ClientFrame, Notice, NoticeKind, Participant, RoomState, ServerFrame } from "@/lib/types";

const pingIntervalMs = 15000;
const pollIntervalMs = 750;
const reconnectBaseMs = 1000;
const reconnectMaxMs = 15000;

// Offsets are re-measured in a burst at connect and topped up on the interval afterwards (see _docs/DESIGN.md §4.3).
const connectPings = 5;

export type ConnectionStatus = "connecting" | "open" | "closed";

export interface SyncHandlers {

  onState: (state: RoomState) => void;
  onParticipants: (participants: Participant[]) => void;
  onNotice: (notice: Omit<Notice, "id">) => void;
  onStatus: (status: ConnectionStatus) => void;

}

export class Sync {

  readonly clock = new Clock();

  private socket: WebSocket | null = null;
  private pingTimer: number | null = null;
  private pollTimer: number | null = null;
  private reconnectTimer: number | null = null;

  private attempts = 0;
  private stopped = false;

  constructor(

    private readonly clientId: string,
    private readonly instanceId: string,
    private readonly socketTicket: string,
    private readonly handlers: SyncHandlers,

  ) {}

  start() {

    this.stopped = false;
    this.open();
    this.poll();

  }

  stop() {

    this.stopped = true;

    this.clearTimers();

    if (this.pollTimer !== null) {

      window.clearTimeout(this.pollTimer);
      this.pollTimer = null;

    }

    this.socket?.close();
    this.socket = null;

  }

  send(frame: ClientFrame): boolean {

    if (frame.type === "control" || frame.type === "queue") {

      this.sendAction(frame);

      return true;

    }

    if (this.socket?.readyState !== WebSocket.OPEN) {

      return false;

    }

    this.socket.send(JSON.stringify(frame));

    return true;

  }

  serverNow(): number {

    return this.clock.now();

  }

  private sendAction(frame: ClientFrame) {

    void fetch("/.proxy/api/room", {

      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ instanceId: this.instanceId, ticket: this.socketTicket, frame }),

    }).then(async (response) => {

      if (!response.ok) {

        throw new Error(`room action failed with ${response.status}`);

      }

      await this.requestState();

    }).catch((error: unknown) => {

      console.error("[streamly] room action failed", error);
      this.handlers.onNotice({ kind: "error", text: "Could not reach the playback room" });

    });

  }

  private poll() {

    void this.requestState().catch((error: unknown) => {

      if (!this.stopped) {

        console.debug("[streamly] room poll failed", error);

      }

    }).finally(() => {

      if (!this.stopped) {

        this.pollTimer = window.setTimeout(() => this.poll(), pollIntervalMs);

      }

    });

  }

  private async requestState() {

    const query = new URLSearchParams({ instanceId: this.instanceId, ticket: this.socketTicket });
    const startedAt = Date.now();

    const response = await fetch(`/.proxy/api/room?${query.toString()}`);

    if (!response.ok) {

      throw new Error(`room state failed with ${response.status}`);

    }

    const snapshot = await response.json() as {

      state: RoomState;
      participants: Participant[];
      serverTime: number;

    };

    this.clock.sample(startedAt, snapshot.serverTime, Date.now());

    this.handlers.onState({ ...snapshot.state, queue: snapshot.state.queue ?? [] });
    this.handlers.onParticipants(snapshot.participants ?? []);
    this.handlers.onStatus("open");

  }

  private open() {

    this.handlers.onStatus("connecting");

    const query = new URLSearchParams({ instanceId: this.instanceId, ticket: this.socketTicket });
    const socket = new WebSocket(`wss://${this.clientId}.discordsays.com/ws?${query.toString()}`);

    this.socket = socket;

    socket.onmessage = (event) => {

      this.receive(JSON.parse(event.data as string) as ServerFrame);

    };

    socket.onclose = () => {

      this.clearTimers();

      this.handlers.onStatus("closed");

      if (!this.stopped) {

        this.scheduleReconnect();

      }

    };

    socket.onerror = () => {

      socket.close();

    };

  }

  private receive(frame: ServerFrame) {

    switch (frame.type) {

      case "pong":

        if (frame.t0 && frame.t1) {

          this.clock.sample(frame.t0, frame.t1, Date.now());

        }

        return;

      case "welcome":

        this.attempts = 0;
        this.handlers.onStatus("open");

        this.clock.reset();

        for (let index = 0; index < connectPings; index += 1) {

          this.send({ type: "ping", t0: Date.now() });

        }

        this.pingTimer = window.setInterval(() => {

          this.send({ type: "ping", t0: Date.now() });

        }, pingIntervalMs);

        if (frame.participants) {

          this.handlers.onParticipants(frame.participants);

        }

        if (frame.state) {

          this.handlers.onState(frame.state);

        }

        return;

      case "room":

        if (frame.state) {

          this.handlers.onState(frame.state);

        }

        return;

      case "participants":

        this.handlers.onParticipants(frame.participants ?? []);

        return;

      case "notice":

        this.handlers.onNotice({ kind: (frame.kind ?? "action") as NoticeKind, text: frame.text ?? "" });

        return;

    }

  }

  private scheduleReconnect() {

    this.attempts += 1;

    const delay = Math.min(reconnectBaseMs * 2 ** (this.attempts - 1), reconnectMaxMs);

    this.reconnectTimer = window.setTimeout(() => {

      this.open();

    }, delay);

  }

  private clearTimers() {

    if (this.pingTimer !== null) {

      window.clearInterval(this.pingTimer);
      this.pingTimer = null;

    }

    if (this.reconnectTimer !== null) {

      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;

    }

  }

}
