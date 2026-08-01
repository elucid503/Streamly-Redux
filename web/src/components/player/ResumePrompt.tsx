import { Play, RotateCcw } from "lucide-react";

import { Button } from "@/components/ui/button";
import { formatTime } from "@/lib/clock";

interface ResumePromptProps {

  title: string;
  positionMs: number;

  onContinue: () => void;
  onStartOver: () => void;

}

export function ResumePrompt({ title, positionMs, onContinue, onStartOver }: ResumePromptProps) {

  return (

    <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">

      <div className="bg-card w-full max-w-sm rounded-xl border p-5 shadow-2xl">

        <p className="text-muted-foreground text-xs font-medium tracking-wide uppercase">Continue watching</p>

        <h2 className="mt-1 truncate text-base font-semibold">{title}</h2>

        <p className="text-muted-foreground mt-2 text-sm">

          Resume from <span className="text-foreground font-mono tabular-nums">{formatTime(positionMs / 1000)}</span>?

        </p>

        <div className="mt-4 flex gap-2">

          <Button className="flex-1" onClick={onContinue}>

            <Play />
            Continue

          </Button>

          <Button variant="secondary" className="flex-1" onClick={onStartOver}>

            <RotateCcw />
            Start over

          </Button>

        </div>

      </div>

    </div>

  );

}
