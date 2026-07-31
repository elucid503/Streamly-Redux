import { useEffect, useRef, useState } from "react";
import Hls from "hls.js";

interface PlayerProps {

  src: string;

}

export function Player({ src }: PlayerProps) {

  const videoRef = useRef<HTMLVideoElement>(null);

  const [muted, setMuted] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {

    const video = videoRef.current;

    if (!video) {

      return;

    }

    setError(null);

    if (video.canPlayType("application/vnd.apple.mpegurl")) {

      video.src = src;

      return;

    }

    if (!Hls.isSupported()) {

      setError("HLS is not supported in this client");

      return;

    }

    const hls = new Hls({

      lowLatencyMode: false,

    });

    hls.on(Hls.Events.ERROR, (_event, data) => {

      if (!data.fatal) {

        return;

      }

      setError(`${data.type}: ${data.details}`);

    });

    hls.loadSource(src);
    hls.attachMedia(video);

    return () => {

      hls.destroy();

    };

  }, [src]);

  const handleUnmute = () => {

    setMuted(false);

  };

  return (

    <div className="relative h-full w-full bg-black">

      <video ref={videoRef} className="h-full w-full" autoPlay playsInline controls muted={muted} />

      {muted && !error && (

        <button className="absolute bottom-16 left-1/2 -translate-x-1/2 rounded-full bg-white/90 px-5 py-2 text-sm font-medium text-black hover:bg-white" onClick={handleUnmute} >

          Tap to unmute

        </button>

      )}

      {error && (

        <div className="absolute inset-0 flex items-center justify-center bg-black/80 p-6 text-center text-sm text-red-300">

          {error}

        </div>

      )}

    </div>

  );

}
