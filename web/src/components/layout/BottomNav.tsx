import { Clapperboard, Radio, Trophy } from "lucide-react";
import { motion } from "framer-motion";

import { softSpring } from "@/lib/motion";
import { cn } from "@/lib/cn";

export type HomeTab = "vod" | "live" | "sports";

interface BottomNavProps {

  tab: HomeTab;
  onChange: (tab: HomeTab) => void;

}

const tabs: Array<{ id: HomeTab; label: string; icon: typeof Clapperboard }> = [

  { id: "vod", label: "Movies & Shows", icon: Clapperboard },
  { id: "live", label: "Live TV", icon: Radio },
  { id: "sports", label: "Sports", icon: Trophy },

];

export function BottomNav({ tab, onChange }: BottomNavProps) {

  return (

    <div className="pointer-events-none fixed inset-x-0 bottom-0 z-40 flex justify-center px-3 pb-3">

      <nav className="pointer-events-auto flex items-center gap-1 rounded-full border bg-card/90 p-1 shadow-xl backdrop-blur-md">

        {tabs.map((entry) => {

          const active = tab === entry.id;
          const Icon = entry.icon;

          return (

            <button
              key={entry.id}
              type="button"
              onClick={() => onChange(entry.id)}
              className={cn(
                "relative flex items-center gap-2 rounded-full px-3.5 py-2 text-xs font-medium sm:px-4 sm:text-sm",
                active ? "text-primary-foreground" : "text-muted-foreground hover:text-foreground",
              )}
            >

              {active && (

                <motion.span
                  layoutId="bottom-nav-pill"
                  className="bg-primary absolute inset-0 rounded-full shadow-sm"
                  transition={softSpring}
                />

              )}

              <span className="relative z-10 flex items-center gap-2">

                <Icon className="size-3.5 sm:size-4" />
                <span className="whitespace-nowrap">{entry.label}</span>

              </span>

            </button>

          );

        })}

      </nav>

    </div>

  );

}
