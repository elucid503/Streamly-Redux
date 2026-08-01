import { useState } from "react";

import { shortTeamName } from "@/components/sports/matchUtils";
import { cn } from "@/lib/cn";

interface TeamMarkProps {

  name?: string;
  logo?: string;
  size?: "sm" | "md" | "lg";

}

const sizes = {

  sm: { box: "size-10", img: "size-8", text: "text-xs" },
  md: { box: "size-12 sm:size-14", img: "size-9 sm:size-11", text: "text-xs sm:text-sm" },
  lg: { box: "size-16 sm:size-20", img: "size-12 sm:size-16", text: "text-sm sm:text-base" },

};

export function TeamMark({ name, logo, size = "md" }: TeamMarkProps) {

  const [failed, setFailed] = useState(false);
  const dim = sizes[size];
  const label = shortTeamName(name);

  return (

    <div className="flex min-w-0 flex-col items-center gap-1.5 text-center">

      <div className={cn("bg-muted/60 border-border flex items-center justify-center rounded-2xl border", dim.box)}>

        {logo && !failed ? (

          <img
            src={logo}
            alt={name ?? ""}
            className={cn("object-contain", dim.img)}
            loading="lazy"
            decoding="async"
            onError={() => setFailed(true)}
          />

        ) : (

          <span className={cn("text-muted-foreground font-semibold tracking-wide", dim.text)}>

            {(name ?? "?").trim().charAt(0).toUpperCase()}

          </span>

        )}

      </div>

      {name && (

        <span className={cn("text-foreground max-w-[7.5rem] truncate font-medium sm:max-w-[9rem]", dim.text)}>

          {label}

        </span>

      )}

    </div>

  );

}
