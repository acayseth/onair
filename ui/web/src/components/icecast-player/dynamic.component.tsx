"use client";

import dynamic from "next/dynamic";

export const IcecastPlayerComponent = dynamic(
  () =>
    import("@/components/icecast-player/component").then(
      (mod) => mod.IcecastPlayerComponent,
    ),
  {
    ssr: false,
    loading: () => null,
  },
);
