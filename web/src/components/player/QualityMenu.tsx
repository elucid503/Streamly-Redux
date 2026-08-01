import { useMemo } from "react";
import { Check, Settings } from "lucide-react";

import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

import type { PlayerHandle } from "@/hooks/usePlayer";
import { cn } from "@/lib/cn";

// Sits beside the subtitle menu but does something quite different, so the menu has to say so (see _docs/DESIGN.md §6.2).
export function QualityMenu({ handle }: { handle: PlayerHandle }) {

  const qualities = useMemo(() => {

    return handle.qualities.filter((entry) => !isOriginalLabel(entry.label));

  }, [handle.qualities]);

  return (

    <DropdownMenu>

      <DropdownMenuTrigger asChild>

        <Button variant="ghost" size="icon" className="text-white hover:bg-white/15 hover:text-white data-[state=open]:bg-white/15" disabled={qualities.length === 0}>

          <Settings />

        </Button>

      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="max-h-[200px] w-44 overflow-y-auto">

        <DropdownMenuLabel className="px-2 py-1.5">

          <div className="text-sm font-medium text-foreground">Quality</div>
          <div className="text-muted-foreground text-xs font-normal">Only visible for you</div>

        </DropdownMenuLabel>

        <DropdownMenuSeparator />

        {qualities.map((entry) => {

          const selected = handle.quality?.label === entry.label;

          return (

            <DropdownMenuItem
              key={entry.label}
              className="justify-between gap-3"
              onSelect={() => handle.selectQuality(entry.label)}
            >

              <span className="truncate">{entry.label}</span>
              <Check className={cn("size-3.5 shrink-0", selected ? "opacity-100" : "opacity-0")} />

            </DropdownMenuItem>

          );

        })}

      </DropdownMenuContent>

    </DropdownMenu>

  );

}

function isOriginalLabel(label: string): boolean {

  const normalized = label.trim().toUpperCase();

  return normalized === "ORG" || normalized === "ORIGINAL" || normalized === "SOURCE";

}
