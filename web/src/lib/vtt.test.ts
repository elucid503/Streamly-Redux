import { describe, expect, test } from "bun:test";

import { activeCueText, parseWebVtt } from "@/lib/vtt";

describe("parseWebVtt", () => {

  test("parses standard webvtt", () => {

    const cues = parseWebVtt(`WEBVTT

1
00:00:01.500 --> 00:00:04.000
Hello world

2
00:00:05.000 --> 00:00:06.500
Second line
`);

    expect(cues).toHaveLength(2);
    expect(cues[0]).toEqual({ start: 1.5, end: 4, text: "Hello world" });
    expect(activeCueText(cues, 2)).toBe("Hello world");
    expect(activeCueText(cues, 5.5)).toBe("Second line");
    expect(activeCueText(cues, 0)).toBeNull();

  });

  test("parses bare srt", () => {

    const cues = parseWebVtt(`1
00:00:01,500 --> 00:00:04,000
I must not fear.

2
00:01:02,250 --> 00:01:03,100
Fear is the mind-killer.
`);

    expect(cues).toHaveLength(2);
    expect(cues[0].text).toBe("I must not fear.");
    expect(cues[1].start).toBeCloseTo(62.25);

  });

  test("parses loose blocks without blank lines", () => {

    const cues = parseWebVtt(`WEBVTT
00:00:01.000 --> 00:00:02.000
One
00:00:03.000 --> 00:00:04.000
Two`);

    expect(cues.length).toBeGreaterThanOrEqual(2);
    expect(activeCueText(cues, 1.5)).toBe("One");

  });

  test("parses fractional seconds with short ms", () => {

    const cues = parseWebVtt(`WEBVTT

00:00:01.5 --> 00:00:02.5
Half second
`);

    expect(cues[0]?.start).toBeCloseTo(1.5);
    expect(cues[0]?.end).toBeCloseTo(2.5);

  });

});
