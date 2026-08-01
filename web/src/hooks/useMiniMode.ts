import { useEffect, useSyncExternalStore } from "react";

/**
 * Discord's activity miniplayer shrinks the iframe to a short, narrow frame
 * (roughly ~420×320–400 from observed chrome: search + chips + bottom dock).
 * Height is the reliable signal; width alone would also fire on tall mobile panes.
 */
function isMiniViewport(): boolean {

  const width = window.innerWidth;
  const height = window.innerHeight;

  return height <= 400 || (width <= 520 && height <= 480);

}

function subscribe(onStoreChange: () => void): () => void {

  window.addEventListener("resize", onStoreChange);
  window.visualViewport?.addEventListener("resize", onStoreChange);

  return () => {

    window.removeEventListener("resize", onStoreChange);
    window.visualViewport?.removeEventListener("resize", onStoreChange);

  };

}

export function useMiniMode(): boolean {

  return useSyncExternalStore(subscribe, isMiniViewport, () => false);

}

/** Keeps `data-mini` on <html> so CSS can react without per-component hooks. */
export function useMiniModeAttribute(): boolean {

  const mini = useMiniMode();

  useEffect(() => {

    document.documentElement.toggleAttribute("data-mini", mini);

    return () => document.documentElement.removeAttribute("data-mini");

  }, [mini]);

  return mini;

}
