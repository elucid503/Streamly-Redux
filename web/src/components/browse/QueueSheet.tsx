import { useEffect, useRef, useState, type RefObject } from "react";
import { ListVideo, X } from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";

import { Queue } from "@/components/browse/Queue";
import { Button } from "@/components/ui/button";

import { useRoom } from "@/hooks/useRoom";
import { backdropVariants, fadeTransition, sheetVariants, softSpring } from "@/lib/motion";
import { cn } from "@/lib/cn";

interface QueueSheetProps {

  open: boolean;
  onOpenChange: (open: boolean) => void;
  onPlay: () => void;

  /** When false, only the panel is rendered (caller supplies its own trigger). */
  showTrigger?: boolean;
  /** Anchor the panel under this element when showTrigger is false. */
  anchorRef?: RefObject<HTMLElement | null>;
  className?: string;

}

export function QueueSheet({ open, onOpenChange, onPlay, showTrigger = true, anchorRef, className }: QueueSheetProps) {

  const { state } = useRoom();

  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const [anchor, setAnchor] = useState<{ top: number; right: number; dropUp: boolean } | null>(null);

  useEffect(() => {

    if (!open) {

      return;

    }

    const update = () => {

      const node = showTrigger ? triggerRef.current : anchorRef?.current;

      if (!node) {

        setAnchor({ top: 56, right: 12, dropUp: false });
        return;

      }

      const box = node.getBoundingClientRect();
      const spaceBelow = window.innerHeight - box.bottom;
      const dropUp = spaceBelow < 280 && box.top > spaceBelow;

      setAnchor({

        top: dropUp ? Math.max(12, box.top - 8) : box.bottom + 8,
        right: Math.max(12, window.innerWidth - box.right),
        dropUp,

      });

    };

    update();

    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);

    return () => {

      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);

    };

  }, [open, showTrigger, anchorRef]);

  return (

    <>

      {showTrigger && (

        <Button
          ref={triggerRef}
          variant="secondary"
          className={cn(className)}
          onClick={() => onOpenChange(true)}
        >

          <ListVideo />
          Queue
          {state.queue.length > 0 && (

            <span className="bg-primary text-primary-foreground rounded-full px-1.5 py-0.5 text-[10px] font-semibold leading-none">

              {state.queue.length}

            </span>

          )}

        </Button>

      )}

      <AnimatePresence>

        {open && (

          <>

            <motion.button
              type="button"
              className="fixed inset-0 z-50 bg-black/40"
              aria-label="Close queue"
              onClick={() => onOpenChange(false)}
              variants={backdropVariants}
              initial="hidden"
              animate="visible"
              exit="exit"
              transition={fadeTransition}
            />

            <motion.div
              className="bg-card fixed z-50 flex max-h-[min(70vh,28rem)] w-[min(calc(100vw-1.5rem),24rem)] flex-col overflow-hidden rounded-xl border shadow-2xl"
              style={
                anchor?.dropUp
                  ? { bottom: window.innerHeight - (anchor.top), right: anchor.right, transformOrigin: "bottom right" }
                  : { top: anchor?.top ?? 56, right: anchor?.right ?? 12, transformOrigin: "top right" }
              }
              variants={sheetVariants}
              initial="hidden"
              animate="visible"
              exit="exit"
              transition={softSpring}
            >

              <div className="flex items-center justify-between border-b px-4 py-3">

                <div>

                  <p className="text-sm font-semibold">Watch queue</p>
                  <p className="text-muted-foreground text-xs">{state.queue.length} item{state.queue.length === 1 ? "" : "s"}</p>

                </div>

                <Button variant="ghost" size="icon-sm" onClick={() => onOpenChange(false)}>

                  <X />

                </Button>

              </div>

              <div className="overflow-y-auto p-3">

                <Queue
                  onPlay={() => {

                    onOpenChange(false);
                    onPlay();

                  }}
                />

              </div>

            </motion.div>

          </>

        )}

      </AnimatePresence>

    </>

  );

}
