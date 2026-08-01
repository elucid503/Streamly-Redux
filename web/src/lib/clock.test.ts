import { describe, expect, test } from "bun:test";

import { Clock, expectedPosition, formatTime, medianOf } from "@/lib/clock";
import type { RoomState } from "@/lib/types";

const base: RoomState = {

  item: null,
  playback: null,

  playing: true,

  anchorMs: 10000,
  anchorAt: 1000000,

  subtitle: null,

  queue: [],
  queueIndex: 0,

  lastActor: null,

};

describe("expectedPosition", () => {

  test("advances with server time while playing", () => {

    expect(expectedPosition(base, 1005000)).toBe(15000);

  });

  test("holds the anchor while paused", () => {

    expect(expectedPosition({ ...base, playing: false }, 1005000)).toBe(10000);

  });

});

describe("Clock", () => {

  test("takes the median offset rather than the last sample", () => {

    const clock = new Clock();

    clock.sample(1000, 1600, 1200);
    clock.sample(2000, 2500, 2200);
    clock.sample(3000, 9000, 3200);

    expect(clock.offset).toBe(500);

  });

  test("reports no offset before any sample", () => {

    expect(new Clock().ready).toBe(false);

  });

});

describe("medianOf", () => {

  test("is zero for an empty set", () => {

    expect(medianOf([])).toBe(0);

  });

});

describe("formatTime", () => {

  test("drops the hour when it is zero", () => {

    expect(formatTime(75)).toBe("1:15");

  });

  test("pads minutes once hours appear", () => {

    expect(formatTime(3675)).toBe("1:01:15");

  });

});
