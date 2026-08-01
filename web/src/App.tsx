import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";

import { BottomNav, type HomeTab } from "@/components/layout/BottomNav";
import { Notices } from "@/components/Notices";
import { TooltipProvider } from "@/components/ui/tooltip";

import { RoomProvider, useRoom } from "@/hooks/useRoom";
import { formatError } from "@/lib/errors";
import { fadeTransition, pageFade } from "@/lib/motion";
import { connect, type Session } from "@/lib/sdk";
import { Browse } from "@/screens/Browse";
import { Live } from "@/screens/Live";
import { Sports } from "@/screens/Sports";
import { Watch } from "@/screens/Watch";

export function App() {

  const [session, setSession] = useState<Session | null>(null);
  const [step, setStep] = useState("Starting");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {

    let cancelled = false;

    void connect((next) => !cancelled && setStep(next))
      .then((established) => !cancelled && setSession(established))
      .catch((caught: unknown) => !cancelled && setError(formatError(caught)));

    return () => {

      cancelled = true;

    };

  }, []);

  if (error) {

    return (

      <Centered>

        <p className="text-destructive">{error}</p>

      </Centered>

    );

  }

  if (!session) {

    return (

      <Centered>

        <Loader2 className="text-muted-foreground size-5 animate-spin" />
        <p className="text-muted-foreground">{step}…</p>

      </Centered>

    );

  }

  return (

    <TooltipProvider>

      <RoomProvider session={session}>

        <Shell />

        <Notices />

      </RoomProvider>

    </TooltipProvider>

  );

}

// Two full-frame modes rather than a persistent sidebar — the activity pane is small (see _docs/DESIGN.md §6.1).
function Shell() {

  const { joined, arrivedMidSession } = useRoom();

  const [mode, setMode] = useState<"home" | "watch">("home");
  const [tab, setTab] = useState<HomeTab>("vod");

  useEffect(() => {

    if (joined && arrivedMidSession) {

      setMode("watch");

    }

  }, [joined, arrivedMidSession]);

  if (mode === "watch") {

    return (

      <motion.div
        className="h-full overflow-hidden"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={fadeTransition}
      >

        <Watch arrived={arrivedMidSession} onBrowse={() => setMode("home")} />

      </motion.div>

    );

  }

  return (

    <div className="relative h-full">

      <div className="h-full overflow-y-auto">

        <AnimatePresence mode="wait">

          <motion.div
            key={tab}
            className="min-h-full"
            variants={pageFade}
            initial="hidden"
            animate="visible"
            exit="exit"
            transition={fadeTransition}
          >

            {tab === "live" ? (

              <Live onWatch={() => setMode("watch")} />

            ) : tab === "sports" ? (

              <Sports onWatch={() => setMode("watch")} />

            ) : (

              <Browse onWatch={() => setMode("watch")} />

            )}

          </motion.div>

        </AnimatePresence>

      </div>

      <BottomNav tab={tab} onChange={setTab} />

    </div>

  );

}

function Centered({ children }: { children: React.ReactNode }) {

  return (

    <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center text-sm">

      {children}

    </div>

  );

}
