import type { LucideIcon } from "lucide-react";

interface MiniPlaceholderProps {

  label: string;
  icon: LucideIcon;

}

/** Non-interactive miniplayer stub — black frame with a dim icon and a short status line. */
export function MiniPlaceholder({ label, icon: Icon }: MiniPlaceholderProps) {

  return (

    <div className="relative flex h-full min-h-full w-full items-center justify-center overflow-hidden bg-black">

      <Icon
        aria-hidden
        className="pointer-events-none absolute top-1/2 left-1/2 size-[min(42vw,9rem)] -translate-x-1/2 -translate-y-1/2 text-neutral-800"
        strokeWidth={1.25}
      />

      <p className="relative z-10 px-4 text-center text-sm font-medium tracking-tight text-white/70">

        {label}

      </p>

    </div>

  );

}
