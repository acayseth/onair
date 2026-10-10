"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  createIcecastPlayer,
  type IcecastPlayerInstance,
  type IcyMetadata,
} from "@/lib/icecast.lib";

const DEFAULT_URL = "https://stream.radioparadise.com/rock-32";

const INITIAL_METADATA: IcyMetadata = {
  StreamTitle: "Unknown",
  StreamUrl: undefined,
};

export const useIcecastPlayerHook = (url: string = DEFAULT_URL) => {
  const playerRef = useRef<IcecastPlayerInstance | null>(null);

  const [metadata, setMetadata] = useState<IcyMetadata>(INITIAL_METADATA);
  const [isLoading, setIsLoading] = useState(true);
  const [isStarted, setIsStarted] = useState(false);
  const [isMuted, setIsMuted] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Pornește streamul automat pe mute.
  useEffect(() => {
    let disposed = false;

    setIsLoading(true);
    setIsStarted(false);
    setIsMuted(true);
    setError(null);
    setMetadata(INITIAL_METADATA);

    const player = createIcecastPlayer({
      url,

      onMetadata: (data) => {
        if (!disposed) setMetadata(data);
      },

      onLoad: () => {
        if (!disposed) setIsLoading(true);
      },

      onPlay: () => {
        if (!disposed) {
          setIsLoading(false);
          setIsStarted(true);
        }
      },

      onStop: () => {
        if (!disposed) {
          setIsStarted(false);
          setIsLoading(false);
        }
      },

      onError: (message) => {
        if (!disposed) {
          setError(message);
          setIsLoading(false);
        }
      },
    });

    playerRef.current = player;
    player.audioElement.muted = true;

    player.play().catch((e: unknown) => {
      if (disposed) return;

      if (e instanceof Error && e.name === "AbortError") return;

      setError(e instanceof Error ? e.message : "Autoplay blocked");
      setIsLoading(false);
    });

    return () => {
      disposed = true;

      if (playerRef.current === player) {
        playerRef.current = null;
      }

      // Nu apelăm player.stop() aici.
      // Nu întrerupem intenționat streamul la cleanup.
      player.audioElement.muted = true;
    };
  }, [url]);

  // Play = unmute; streamul continuă să ruleze.
  const play = useCallback(async () => {
    const player = playerRef.current;
    if (!player) return;

    player.audioElement.muted = false;
    setIsMuted(false);

    // Pornim playerul numai dacă nu a pornit deja.
    if (!isStarted) {
      setError(null);
      setIsLoading(true);

      try {
        await player.play();
      } catch (e: unknown) {
        if (e instanceof Error && e.name === "AbortError") return;

        setError(e instanceof Error ? e.message : "Playback error");
        setIsLoading(false);
      }
    }
  }, [isStarted]);

  // Pause = mute; nu oprim streamul.
  const pause = useCallback(() => {
    const player = playerRef.current;
    if (!player) return;

    player.audioElement.muted = true;
    setIsMuted(true);
  }, []);

  const isPlaying = isStarted && !isMuted;

  const toggle = useCallback(() => {
    if (isPlaying) {
      pause();
    } else {
      void play();
    }
  }, [isPlaying, play, pause]);

  return {
    metadata,
    isLoading,
    isStarted,
    isMuted,
    isPlaying,
    error,
    play,
    pause,
    toggle,
  };
};
