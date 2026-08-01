import { Loader2 } from "lucide-react";

interface PageLoaderProps {

  label?: string;
  absolute?: boolean;

}

export function PageLoader({ label = "Loading", absolute = false }: PageLoaderProps) {

  return (

    <div className={`${absolute ? "absolute" : "fixed"} inset-0 z-40 flex items-center justify-center bg-background/80 backdrop-blur-sm`}>

      <div className="flex flex-col items-center gap-3 text-sm text-muted-foreground">

        <Loader2 className="size-7 animate-spin" />
        <span>{label}</span>

      </div>

    </div>

  );

}
