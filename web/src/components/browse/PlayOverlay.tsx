import { Play } from "lucide-react";

import { cn } from "@/lib/cn";

interface PlayOverlayProps {

  className?: string;
  size?: "sm" | "md";

}

/** Hover-reveal play control for poster and channel media surfaces. Parent must use `group`. */
export function PlayOverlay({ className, size = "md" }: PlayOverlayProps) {

  return (

    <div
      aria-hidden
      className={cn(
        "pointer-events-none absolute inset-0 z-10 flex items-center justify-center",
        "bg-black/20 opacity-0 backdrop-blur-[1px] transition-[opacity,backdrop-filter] duration-200 ease-out",
        "group-hover:opacity-100 group-focus-visible:opacity-100",
        className,
      )}
    >

      <div
        className={cn(
          "flex items-center justify-center rounded-full bg-white/90 text-black shadow-md",
          "scale-75 opacity-0 blur-sm transition-[opacity,transform,filter] duration-200 ease-out",
          "group-hover:scale-100 group-hover:opacity-100 group-hover:blur-none",
          "group-focus-visible:scale-100 group-focus-visible:opacity-100 group-focus-visible:blur-none",
          size === "sm" ? "size-8" : "size-11",
        )}
      >

        <Play className={cn("fill-current translate-x-px", size === "sm" ? "size-3.5" : "size-5")} />

      </div>

    </div>

  );

}
