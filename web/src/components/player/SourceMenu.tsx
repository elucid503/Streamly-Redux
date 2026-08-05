import { Check, RadioTower } from "lucide-react";

import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

import type { PlayerHandle } from "@/hooks/usePlayer";
import { cn } from "@/lib/cn";

// Labels stay generic on purpose — provider names never surface in the player chrome.
export function SourceMenu({ handle }: { handle: PlayerHandle }) {

  const count = handle.sourceCount;
  const current = handle.sourceIndex;

  if (count < 2) {

    return null;

  }

  const options = Array.from({ length: count }, (_, index) => index);

  return (

    <DropdownMenu>

      <DropdownMenuTrigger asChild>

        <Button variant="ghost" size="icon" className="text-white hover:bg-white/15 hover:text-white data-[state=open]:bg-white/15" aria-label="Select stream source" >

          <RadioTower />

        </Button>

      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="max-h-[200px] w-44 overflow-y-auto">

        <DropdownMenuLabel className="px-2 py-1.5">

          <div className="text-sm font-medium text-foreground">Source</div>
          <div className="text-muted-foreground text-xs font-normal">Changes for everyone</div>

        </DropdownMenuLabel>

        <DropdownMenuSeparator />

        {options.map((index) => {

          const selected = index === current;

          return (

            <DropdownMenuItem key={index} className="justify-between gap-3"

              onSelect={() => {

                if (index !== current) {

                  handle.selectSource(index);

                }

              }}

            >

              <span className="truncate">Source {index + 1}</span>
              <Check className={cn("size-3.5 shrink-0", selected ? "opacity-100" : "opacity-0")} />

            </DropdownMenuItem>

          );

        })}

      </DropdownMenuContent>

    </DropdownMenu>

  );

}
