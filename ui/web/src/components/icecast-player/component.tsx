"use client";

import { Button, Card } from "@heroui/react";
import type { FC } from "react";
import { CoverComponent } from "@/components/icecast-player/cover.component";
import { useIcecastPlayerHook } from "@/components/icecast-player/icecast-player.hook";
import { PauseIcon, PlayIcon } from "@/components/icecast-player/icon";

export const IcecastPlayerComponent: FC<{ streamUrl: string }> = ({
  streamUrl,
}) => {
  const { metadata, isLoading, isPlaying, error, toggle } =
    useIcecastPlayerHook(streamUrl);

  if (isLoading) {
    return <>loading...</>;
  }

  return (
    <Card className="mx-auto container max-w-xs mt-12" variant="secondary">
      <Card.Content>
        <CoverComponent metadata={metadata} />
      </Card.Content>
      <Card.Header className="items-center">
        <Card.Title className="font-bold text-md">
          {metadata.StreamTitle}
        </Card.Title>
        {error && <Card.Description>{error}</Card.Description>}
      </Card.Header>
      <Card.Footer className="flex justify-center items-center">
        <Button isIconOnly size="lg" onClick={toggle} isDisabled={isLoading}>
          {!isPlaying && <PlayIcon className="w-5 h-5 ml-0.5" />}
          {isPlaying && <PauseIcon className="w-5 h-5" />}
        </Button>
      </Card.Footer>
    </Card>
  );
};
