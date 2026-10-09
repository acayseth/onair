import IcecastMetadataPlayer, {
  type IcyMetadata,
  type IcecastMetadataPlayerIcyOptionsWithCallbacks,
} from "icecast-metadata-player";

export type { IcyMetadata };

export type CreateIcecastPlayerOptions = {
  url: string;
} & Omit<
  IcecastMetadataPlayerIcyOptionsWithCallbacks,
  "metadataTypes" | "onMetadata"
> & {
    onMetadata: (metadata: IcyMetadata) => void;
  };

const normalizeMetadata = (metadata: IcyMetadata): IcyMetadata => ({
  ...metadata,
  StreamTitle: metadata.StreamTitle?.trim() || "Unknown",
  StreamUrl: metadata.StreamUrl?.trim() || undefined,
});

export const createIcecastPlayer = ({
  url,
  onMetadata,
  ...options
}: CreateIcecastPlayerOptions) => {
  const playerOptions: IcecastMetadataPlayerIcyOptionsWithCallbacks = {
    ...options,
    metadataTypes: ["icy"],
    onMetadata: (metadata) => {
      onMetadata(normalizeMetadata(metadata));
    },
  };

  return new IcecastMetadataPlayer(url, playerOptions);
};

export type IcecastPlayerInstance = ReturnType<typeof createIcecastPlayer>;
