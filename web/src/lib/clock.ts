import type { RoomState } from "@/lib/types";

const maxSamples = 9;

// Correction threshold: below this a client leaves itself alone (see _docs/DESIGN.md §4.2).
export const driftToleranceMs = 2000;

export function medianOf(values: number[]): number {

  if (values.length === 0) {

    return 0;

  }

  const sorted = [...values].sort((a, b) => a - b);

  return sorted[Math.floor(sorted.length / 2)];

}

// A skewed client clock sits confidently in the wrong place, so every position is measured against server time (§4.3).
export class Clock {

  private offsets: number[] = [];

  sample(sentAt: number, serverTime: number, receivedAt: number) {

    const roundTrip = receivedAt - sentAt;

    this.offsets.push(serverTime - (sentAt + roundTrip / 2));

    if (this.offsets.length > maxSamples) {

      this.offsets.shift();

    }

  }

  reset() {

    this.offsets = [];

  }

  get offset(): number {

    return medianOf(this.offsets);

  }

  get ready(): boolean {

    return this.offsets.length > 0;

  }

  now(): number {

    return Date.now() + this.offset;

  }

}

export function expectedPosition(state: RoomState, serverNow: number): number {

  if (!state.playing) {

    return state.anchorMs;

  }

  return state.anchorMs + (serverNow - state.anchorAt);

}

export function formatTime(seconds: number): string {

  if (!Number.isFinite(seconds) || seconds < 0) {

    return "0:00";

  }

  const whole = Math.floor(seconds);

  const hours = Math.floor(whole / 3600);
  const minutes = Math.floor((whole % 3600) / 60);
  const rest = whole % 60;

  const padded = `${minutes.toString().padStart(hours > 0 ? 2 : 1, "0")}:${rest.toString().padStart(2, "0")}`;

  return hours > 0 ? `${hours}:${padded}` : padded;

}
