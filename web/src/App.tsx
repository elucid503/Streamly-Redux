import { useEffect, useState } from "react";

import { Player } from "@/components/player/Player";

import { getStreamUrl } from "@/lib/api";
import { formatError } from "@/lib/errors";
import { connect, type AuthUser } from "@/lib/sdk";

const defaultChannelId = "51";

export function App() {

  const [user, setUser] = useState<AuthUser | null>(null);
  const [streamUrl, setStreamUrl] = useState<string | null>(null);
  const [step, setStep] = useState("Starting");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {

    let cancelled = false;

    const start = async () => {

      try {

        const authenticated = await connect((next) => {

          if (!cancelled) {

            setStep(next);

          }

        });

        if (cancelled) {

          return;

        }

        setUser(authenticated);

        const channelId = new URLSearchParams(window.location.search).get("channel") ?? defaultChannelId;

        const url = await getStreamUrl(channelId);

        if (cancelled) {

          return;

        }

        setStreamUrl(url);

      } catch (caught) {

        if (!cancelled) {

          setError(formatError(caught));

        }

      }

    };

    void start();

    return () => {

      cancelled = true;

    };

  }, []);

  if (error) {

    return (

      <Centered>

        <p className="text-red-300">{error}</p>

      </Centered>

    );

  }

  if (!user) {

    return (

      <Centered>

        <p className="text-neutral-400">{step}…</p>

      </Centered>

    );

  }

  return (

    <div className="flex h-full flex-col">

      <header className="flex items-center justify-between px-4 py-2 text-sm">

        <span className="font-medium">Streamly</span>
        <span className="text-neutral-400">{user.displayName}</span>

      </header>

      <main className="min-h-0 flex-1">

        {streamUrl ? (

          <Player src={streamUrl} />

        ) : (

          <Centered>

            <p className="text-neutral-400">Resolving stream…</p>

          </Centered>

        )}

      </main>

    </div>

  );

}

function Centered({ children }: { children: React.ReactNode }) {

  return (

    <div className="flex h-full items-center justify-center p-6 text-center text-sm">

      {children}

    </div>

  );

}
