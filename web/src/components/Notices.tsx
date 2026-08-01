import { AlertTriangle, Info, Shuffle } from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";

import { useRoom } from "@/hooks/useRoom";
import { noticeVariants, softSpring } from "@/lib/motion";
import { cn } from "@/lib/cn";
import type { NoticeKind } from "@/lib/types";

const icons: Record<NoticeKind, typeof Info> = {

  failover: Shuffle,
  error: AlertTriangle,
  action: Info,

};

// One lightweight channel for every room-level event; voice chat is the real communication layer (see _docs/DESIGN.md §6.3).
export function Notices() {

  const { notices } = useRoom();

  return (

    <div className="pointer-events-none fixed inset-x-0 bottom-4 z-50 flex flex-col items-center gap-2 px-4">

      <AnimatePresence mode="popLayout">

        {notices.map((notice) => {

          const Icon = icons[notice.kind];

          return (

            <motion.div
              key={notice.id}
              layout
              className={cn(
                "bg-popover flex items-center gap-2 rounded-full border py-1.5 pr-4 pl-3 text-sm shadow-lg",
                notice.kind === "error" && "border-destructive/50",
              )}
              variants={noticeVariants}
              initial="hidden"
              animate="visible"
              exit="exit"
              transition={softSpring}
            >

              <Icon className={cn("size-3.5 shrink-0", notice.kind === "error" ? "text-destructive" : "text-muted-foreground")} />

              <span className="truncate">{notice.text}</span>

            </motion.div>

          );

        })}

      </AnimatePresence>

    </div>

  );

}
