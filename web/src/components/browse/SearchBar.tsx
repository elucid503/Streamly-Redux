import { useEffect, useState } from "react";
import { Loader2, Search, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const debounceMs = 350;

interface SearchBarProps {

  value: string;
  searching: boolean;

  onChange: (value: string) => void;
  placeholder?: string;

}

// One field spans channels and VOD; results are grouped by kind rather than split into two searches (see _docs/DESIGN.md §6.1).
export function SearchBar({ value, searching, onChange, placeholder = "Search channels, movies and series" }: SearchBarProps) {

  const [draft, setDraft] = useState(value);

  useEffect(() => {

    const timer = window.setTimeout(() => onChange(draft.trim()), debounceMs);

    return () => window.clearTimeout(timer);

  }, [draft, onChange]);

  useEffect(() => {

    setDraft(value);

  }, [value]);

  return (

    <div className="relative">

      <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />

      <Input
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        placeholder={placeholder}
        className="pl-9"
      />

      {searching && <Loader2 className="text-muted-foreground absolute top-1/2 right-3 size-4 -translate-y-1/2 animate-spin" />}

      {!searching && draft.length > 0 && (

        <Button
          variant="ghost"
          size="icon-sm"
          className="absolute top-1/2 right-1 -translate-y-1/2"
          onClick={() => setDraft("")}
        >

          <X />

        </Button>

      )}

    </div>

  );

}
