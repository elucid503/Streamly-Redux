import { useEffect, useRef, useState } from "react";

import { fetchQueuedImage } from "@/lib/imageQueue";
import { logoBackdrop } from "@/lib/logoBackdrop";

interface ChannelLogoProps {

  logo?: string;
  name: string;

}

export function ChannelLogo({ logo, name }: ChannelLogoProps) {

  const container = useRef<HTMLDivElement | null>(null);

  const [visible, setVisible] = useState(false);
  const [generation, setGeneration] = useState(0);
  const [objectUrl, setObjectUrl] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const [background, setBackground] = useState("#f4f4f5");

  useEffect(() => {

    const target = container.current;

    if (!target || visible) {

      return;

    }

    const observer = new IntersectionObserver((entries) => {

      if (entries.some((entry) => entry.isIntersecting)) {

        setVisible(true);
        observer.disconnect();

      }

    }, { rootMargin: "320px" });

    observer.observe(target);

    return () => observer.disconnect();

  }, [visible]);

  // Load (and reload on recovery) through the shared queue so the grid never
  // opens dozens of concurrent upstream requests through the proxy.
  useEffect(() => {

    if (!logo || !visible) {

      return;

    }

    let cancelled = false;
    const controller = new AbortController();
    let createdUrl: string | null = null;

    setLoaded(false);
    setFailed(false);
    setObjectUrl((previous) => {

      if (previous) {

        URL.revokeObjectURL(previous);

      }

      return null;

    });

    void fetchQueuedImage(logo, controller.signal)
      .then((blob) => {

        if (cancelled) {

          return;

        }

        createdUrl = URL.createObjectURL(blob);
        setObjectUrl(createdUrl);
        setFailed(false);

      })
      .catch(() => {

        if (!cancelled) {

          setFailed(true);
          setLoaded(false);

        }

      });

    return () => {

      cancelled = true;
      controller.abort();

      if (createdUrl) {

        URL.revokeObjectURL(createdUrl);

      }

    };

  }, [logo, visible, generation]);

  // Soft recovery after a hard failure — no need to scroll away and back.
  useEffect(() => {

    if (!logo || !visible || !failed) {

      return;

    }

    const timer = window.setTimeout(() => {

      setGeneration((value) => value + 1);

    }, 5_000 + Math.random() * 2_500);

    return () => window.clearTimeout(timer);

  }, [logo, visible, failed]);

  return (

    <div
      ref={container}
      className="relative flex h-14 items-center justify-center overflow-hidden rounded-md transition-colors"
      style={{ backgroundColor: background }}
    >

      {!loaded && (

        <span className="text-sm font-semibold text-zinc-500" aria-label={`${name} logo unavailable`}>

          {initial(name)}

        </span>

      )}

      {objectUrl && (

        <img
          src={objectUrl}
          alt={`${name} logo`}
          className={`absolute max-h-10 max-w-[80%] object-contain ${loaded ? "opacity-100" : "opacity-0"}`}
          decoding="async"
          onLoad={(event) => {

            setLoaded(true);
            void logoBackdrop(event.currentTarget).then(setBackground);

          }}
        />

      )}

    </div>

  );

}

function initial(name: string): string {

  return name.trim().charAt(0).toUpperCase() || "?";

}
