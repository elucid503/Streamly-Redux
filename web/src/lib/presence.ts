import { getSDK } from "@/lib/sdk";
import type { Item } from "@/lib/types";

// Activity type 3 = Watching — reads better for a stream app than "Playing".
const activityWatching = 3;

let lastKey = "";
let lastItemKey = "";
let startedAt = 0;
let warned = false;

/** Push a minimal "now watching" line to Discord profiles. Failures are silent. */
export function syncDiscordPresence(item: Item | null, playing: boolean): void {

  const sdk = getSDK();

  if (!sdk) {

    return;

  }

  const itemKey = item
    ? `${item.kind}:${item.id}:${item.season ?? 0}:${item.episode ?? 0}`
    : "browse";

  const key = `${itemKey}:${playing ? 1 : 0}`;

  if (key === lastKey) {

    return;

  }

  lastKey = key;

  if (itemKey !== lastItemKey) {

    lastItemKey = itemKey;
    startedAt = item ? Date.now() : 0;

  }

  if (!item) {

    void setActivity(sdk, {

      type: activityWatching,
      details: "Browsing",

    });
    return;

  }

  const state = item.kind === "channel"
    ? (item.caption || "Live")
    : episodeLabel(item) || (playing ? "Watching together" : "Paused");

  void setActivity(sdk, {

    type: activityWatching,
    details: item.title,
    state,
    timestamps: startedAt ? { start: startedAt } : undefined,

  });

}

function episodeLabel(item: Item): string {

  if (!item.season || !item.episode) {

    return "";

  }

  const ep = `S${item.season}E${item.episode}`;

  return item.episodeTitle ? `${ep} · ${item.episodeTitle}` : ep;

}

async function setActivity(

  sdk: NonNullable<ReturnType<typeof getSDK>>,
  activity: {
    type: number;
    details?: string;
    state?: string;
    timestamps?: { start: number };
  },

): Promise<void> {

  try {

    await sdk.commands.setActivity({ activity });

  } catch (error) {

    if (!warned) {

      warned = true;
      console.warn("[streamly] setActivity failed (scope may be gated)", error);

    }

  }

}
