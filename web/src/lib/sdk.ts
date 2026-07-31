import { DiscordSDK } from "@discord/embedded-app-sdk";

import { exchangeToken, getConfig } from "@/lib/api";
import { formatError } from "@/lib/errors";

export interface AuthUser {

  id: string;
  username: string;
  displayName: string;

}

const stepTimeoutMs = 15000;

let connection: Promise<AuthUser> | null = null;
let instance: DiscordSDK | null = null;

// Every step here can hang rather than reject, so each one is raced against a timeout to keep failures legible.
function withTimeout<T>(label: string, work: Promise<T>): Promise<T> {

  return new Promise<T>((resolve, reject) => {

    const timer = setTimeout(() => {

      reject(new Error(`${label} did not respond within ${stepTimeoutMs / 1000}s`));

    }, stepTimeoutMs);

    work.then((value) => {

        clearTimeout(timer);
        resolve(value);

      }, (error) => {

        clearTimeout(timer);
        console.error("[streamly]", label, "failed", error);

        reject(new Error(`${label} failed: ${formatError(error)}`));

      },

    );

  });

}

export function connect(onStep: (step: string) => void): Promise<AuthUser> {

  if (connection) {

    return connection;

  }

  connection = run(onStep).catch((error: unknown) => {

    connection = null;

    throw error;

  });

  return connection;

}

async function run(onStep: (step: string) => void): Promise<AuthUser> {

  const report = (step: string) => {

    console.info("[streamly]", step);
    onStep(step);

  };

  report("Loading configuration");

  const { clientId } = await withTimeout("Configuration request", getConfig());

  if (!clientId) {

    throw new Error("Server returned an empty client id");

  }

  const params = new URLSearchParams(window.location.search);

  console.info("[streamly] launch params", {

    clientId,
    frameId: params.get("frame_id"),
    instanceId: params.get("instance_id"),
    platform: params.get("platform"),
    guildId: params.get("guild_id"),
    channelId: params.get("channel_id"),

  });

  if (!params.get("frame_id")) {

    throw new Error("Not running inside Discord — launch this from a voice channel's Activities shelf");

  }

  report("Waiting for Discord");

  if (!instance) {

    instance = new DiscordSDK(clientId);

  }

  const sdk = instance;

  await withTimeout("Discord client handshake", sdk.ready());

  report("Requesting authorization");

  const { code } = await withTimeout(

    "Authorization",
    sdk.commands.authorize({

      client_id: clientId,
      response_type: "code",
      state: "",
      prompt: "none",
      scope: ["identify"],

    }),

  );

  report("Exchanging token");

  const accessToken = await withTimeout("Token exchange", exchangeToken(code));

  report("Authenticating");

  const result = await withTimeout( "Authentication", sdk.commands.authenticate({

      access_token: accessToken,

  }));

  const user = result.user as { id: string; username: string; global_name?: string | null };

  return {

    id: user.id,
    username: user.username,
    displayName: user.global_name ?? user.username,

  };

}
