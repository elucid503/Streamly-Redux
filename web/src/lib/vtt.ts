export interface Cue {

  start: number;
  end: number;
  text: string;

}

// Ported from Streamly-Web — the line-scanner is far more tolerant of real SubDL dumps
// than blank-line block splitting, which silently produced zero cues for many releases.
export function parseWebVtt(source: string): Cue[] {

  const raw = source.replace(/^\uFEFF/, "").replace(/\r/g, "");

  if (!raw.trim()) {

    return [];

  }

  if (/^\s*</.test(raw) || /<html[\s>]/i.test(raw)) {

    return [];

  }

  if (/^WEBVTT/i.test(raw.trim())) {

    const cues = parseVtt(raw);

    return cues.length > 0 ? cues : parseSrt(raw);

  }

  const srt = parseSrt(raw);

  return srt.length > 0 ? srt : parseVtt(raw);

}

export function activeCueText(cues: Cue[], positionSeconds: number): string | null {

  if (cues.length === 0 || !Number.isFinite(positionSeconds)) {

    return null;

  }

  for (let index = 0; index < cues.length; index += 1) {

    const cue = cues[index];

    if (positionSeconds >= cue.start && positionSeconds < cue.end) {

      return cue.text;

    }

  }

  return null;

}

function parseVtt(raw: string): Cue[] {

  const lines = raw.split("\n");
  const cues: Cue[] = [];

  let index = 0;

  while (index < lines.length) {

    const line = lines[index].trim();

    index += 1;

    if (!line || line.startsWith("WEBVTT") || line.startsWith("NOTE") || line.startsWith("STYLE") || line.startsWith("REGION")) {

      continue;

    }

    if (!line.includes("-->")) {

      continue;

    }

    const [startRaw, endRaw] = line.split("-->").map((part) => normalizeTimestamp(part.trim().split(/\s+/)[0] ?? ""));
    const textLines: string[] = [];

    while (index < lines.length) {

      const next = lines[index].trim();

      if (!next || next.includes("-->") || /^\d+$/.test(next)) {

        break;

      }

      textLines.push(next);
      index += 1;

    }

    const text = cleanCueText(textLines);
    const start = timeToSeconds(startRaw);
    const end = timeToSeconds(endRaw);

    if (!text || !Number.isFinite(start) || !Number.isFinite(end) || end <= start) {

      continue;

    }

    cues.push({ start, end, text });

  }

  return cues;

}

function parseSrt(raw: string): Cue[] {

  const lines = raw.split("\n");
  const cues: Cue[] = [];

  let index = 0;

  while (index < lines.length) {

    while (index < lines.length && !lines[index].trim()) {

      index += 1;

    }

    if (index >= lines.length) {

      break;

    }

    if (/^\d+$/.test(lines[index].trim())) {

      index += 1;

    }

    if (index >= lines.length) {

      break;

    }

    const timingLine = lines[index]?.trim() ?? "";

    if (!timingLine.includes("-->")) {

      index += 1;
      continue;

    }

    index += 1;

    const [startRaw, endRaw] = timingLine.split("-->").map((part) => normalizeTimestamp(part.trim().split(/\s+/)[0] ?? ""));
    const textLines: string[] = [];

    while (index < lines.length) {

      const next = lines[index].trim();

      if (!next || next.includes("-->") || /^\d+$/.test(next)) {

        break;

      }

      textLines.push(next);
      index += 1;

    }

    const text = cleanCueText(textLines);
    const start = timeToSeconds(startRaw);
    const end = timeToSeconds(endRaw);

    if (!text || !Number.isFinite(start) || !Number.isFinite(end) || end <= start) {

      continue;

    }

    cues.push({ start, end, text });

  }

  return cues;

}

function normalizeTimestamp(value: string): string {

  return value.trim().replace(",", ".");

}

function timeToSeconds(value: string): number {

  const normalized = normalizeTimestamp(value);
  const parts = normalized.split(":");

  if (parts.length === 3) {

    const [hours, minutes, rest] = parts;
    const [seconds, fraction = "0"] = rest.split(".");

    const total = Number(hours) * 3600 + Number(minutes) * 60 + Number(seconds) + fractionToSeconds(fraction);

    return Number.isFinite(total) ? total : Number.NaN;

  }

  if (parts.length === 2) {

    const [minutes, rest] = parts;
    const [seconds, fraction = "0"] = rest.split(".");

    const total = Number(minutes) * 60 + Number(seconds) + fractionToSeconds(fraction);

    return Number.isFinite(total) ? total : Number.NaN;

  }

  return Number.NaN;

}

// SRT/VTT fractional seconds are decimal fractions of a second, not raw digit counts.
function fractionToSeconds(fraction: string): number {

  if (!fraction) {

    return 0;

  }

  const padded = (fraction + "000").slice(0, 3);

  return Number(padded) / 1000;

}

function cleanCueText(lines: string[]): string {

  return lines
    .join("\n")
    .replace(/<\/?[^>]+>/g, "")
    .replace(/\{[^}]+\}/g, "")
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .trim();

}
