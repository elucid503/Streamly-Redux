import { useState } from "react";
import { Play } from "lucide-react";
import { motion } from "framer-motion";

import { Button } from "@/components/ui/button";
import { fadeTransition, softSpring } from "@/lib/motion";
import { cn } from "@/lib/cn";
import type { Item } from "@/lib/types";

interface PauseOverlayProps {

  item: Item;
  onResume: () => void;

}

export function PauseOverlay({ item, onResume }: PauseOverlayProps) {

  const [imageFailed, setImageFailed] = useState(false);

  const live = item.kind === "channel";
  const episode = Boolean(item.episode);
  const showImage = Boolean(item.poster) && !imageFailed;

  return (

    <motion.button
      type="button"
      className="absolute inset-0 z-40 flex cursor-pointer items-center justify-center overflow-hidden bg-black/50 px-6 backdrop-blur-2xl"
      onClick={onResume}
      aria-label="Resume playback"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={fadeTransition}
    >

      <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/70 via-black/35 to-black/20" />

      <motion.div
        className={cn(
          "pointer-events-none relative z-10 mx-auto flex flex-col items-center gap-5 text-center sm:flex-row sm:items-center sm:gap-6 sm:text-left",
          live ? "w-fit max-w-3xl" : "w-full max-w-3xl",
        )}
        initial={{ opacity: 0, y: 10, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={softSpring}
      >

        {showImage && (

          <div
            className={cn(
              "shrink-0 overflow-hidden rounded-xl shadow-2xl ring-1 ring-white/10",
              live && "flex size-32 items-center justify-center bg-black/30 sm:size-40",
              !live && episode && "aspect-video w-56 sm:w-72",
              !live && !episode && "aspect-[2/3] w-32 sm:w-40",
            )}
          >

            <img
              src={item.poster}
              alt=""
              className={cn("size-full", live ? "object-contain p-3" : "object-cover")}
              onError={() => setImageFailed(true)}
            />

          </div>

        )}

        <div className={cn("min-w-0 space-y-1.5", !live && "flex-1")}>

          <h2 className="text-xl font-semibold tracking-tight text-white sm:text-2xl">{item.title}</h2>

          {item.episodeTitle && (

            <p className="text-sm text-white/80 sm:text-base">

              {item.caption ? `${item.caption} · ` : ""}
              {item.episodeTitle}

            </p>

          )}

          {!item.episodeTitle && item.caption && (

            <p className="text-sm text-white/60">{item.caption}</p>

          )}

          {item.description && (

            <p className="line-clamp-2 text-sm leading-relaxed text-white/55">{item.description}</p>

          )}

          <div className="pointer-events-auto flex justify-center pt-2 sm:justify-start">

            <Button
              size="sm"
              className="shadow-lg"
              onClick={(event) => {

                event.stopPropagation();
                onResume();

              }}
            >

              <Play />
              {live ? "Back to live" : "Resume"}

            </Button>

          </div>

        </div>

      </motion.div>

    </motion.button>

  );

}
