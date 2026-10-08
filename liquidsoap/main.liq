# ===========================================================
#        Radio Dream — Liquidsoap 2.5.4 configuration
# ===========================================================

# Environment
icecast_host = environment.get(default="icecast", "ICECAST_HOST")
icecast_port = int_of_string(environment.get(default="8000", "ICECAST_PORT"))
icecast_user = environment.get(default="source", "ICECAST_USER")
icecast_password = environment.get(default="", "ICECAST_PASSWORD")
icecast_mount = environment.get(default="/radio.mp3", "ICECAST_MOUNT")

icecast_name = environment.get(default="Radio Dream", "ICECAST_NAME")
icecast_genre = environment.get(default="Rock", "ICECAST_GENRE")
icecast_description = environment.get(default="Radio Dream", "ICECAST_DESCRIPTION")
icecast_url = environment.get(default="", "ICECAST_URL")

discogs_consumer_key = environment.get(default="", "DISCOGS_CONSUMER_KEY")
discogs_consumer_secret = environment.get(default="", "DISCOGS_CONSUMER_SECRET")

jingle_every = int_of_string(environment.get(default="4", "JINGLE_EVERY"))
stream_bitrate = int_of_string(environment.get(default="128", "STREAM_BITRATE"))

s3_bucket = environment.get(default="", "S3_BUCKET")
s3_tracks_prefix = environment.get(default="tracks", "S3_TRACKS_PREFIX")
s3_jingles_prefix = environment.get(default="jingles", "S3_JINGLES_PREFIX")
s3_access_key_id = environment.get(default="", "S3_ACCESS_KEY_ID")
s3_secret_access_key = environment.get(default="", "S3_SECRET_ACCESS_KEY")

s3_enabled = s3_bucket != ""

settings.log.level := 3

assert(icecast_password != "")
assert(jingle_every > 0)
assert(stream_bitrate >= 8 and stream_bitrate <= 320)

assert(not s3_enabled or s3_access_key_id != "")
assert(not s3_enabled or s3_secret_access_key != "")
assert(not s3_enabled or s3_tracks_prefix != s3_jingles_prefix)

assert(
  (discogs_consumer_key == "" and discogs_consumer_secret == "")
  or (discogs_consumer_key != "" and discogs_consumer_secret != "")
)


# ===========================================================
# Request and metadata helpers
# ===========================================================

def is_audio_request(r) =
  list.mem(
    string.lowercase(file.extension(request.uri(r))),
    [".mp3", ".ogg", ".opus", ".flac", ".wav", ".aif", ".aiff", ".aac", ".m4a"]
  )
end


def apply_metadata(m) =
  if m["radio_jingle"] == "true" then
    [("song", "Jingle"), ("radio_jingle", "true")]
  else
    artist = m["artist"]
    title = m["title"]

    song =
      if artist != "" and title != "" then
        artist ^ " - " ^ title
      elsif title != "" then
        title
      elsif artist != "" then
        artist
      else
        icecast_name
      end

    fields =
      if artist != "" and title != "" then
        [("song", song), ("artist", artist), ("title", title)]
      elsif artist != "" then
        [("song", song), ("artist", artist)]
      elsif title != "" then
        [("song", song), ("title", title)]
      else
        [("song", song)]
      end

    fields
  end
end


# ===========================================================
# Discogs
# ===========================================================

discogs_cache = ref({key="", cover=""})


def discogs_url(base_url, params) =
  auth_params =
    if discogs_consumer_key != "" then
      [
        ("key", discogs_consumer_key),
        ("secret", discogs_consumer_secret)
      ]
    else
      []
    end

  query = http.www_form_urlencoded(list.append(params, auth_params))

  if query == "" then
    base_url
  else
    base_url ^ "?" ^ query
  end
end


def first_discogs_image(images) =
  primary_images =
    list.filter(
      fun(release_image) ->
        release_image.type == "primary"
        and (release_image.uri ?? "") != "",
      images
    )

  if list.length(primary_images) > 0 then
    list.hd(primary_images).uri ?? ""
  elsif list.length(images) > 0 then
    list.hd(images).uri ?? ""
  else
    ""
  end
end


def discogs_release_cover(release_id, headers) =
  response =
    http.get(
      headers=headers,
      timeout=5.,
      discogs_url(
        "https://api.discogs.com/releases/#{release_id}",
        []
      )
    )

  if response.status_code != 200 then
    log.important(
      "Discogs release lookup returned HTTP #{response.status_code}"
    )
    ""
  else
    let json.parse (
      release : {
        images: [{type: string, uri: string?}]
      }
    ) = response

    first_discogs_image(release.images)
  end
end


def discogs_cover(artist, title) =
  if artist == "" or title == "" then
    ""
  else
    key = artist ^ "\n" ^ title
    cached = discogs_cache()

    if cached.key == key then
      cached.cover
    else
      search_url =
        discogs_url(
          "https://api.discogs.com/database/search",
          [
            ("type", "release"),
            ("artist", artist),
            ("track", title),
            ("per_page", "1")
          ]
        )

      headers =
        [
          (
            "User-Agent",
            "derzel-liquidsoap/1.0"
          )
        ]

      cover =
        try
          response =
            http.get(
              headers=headers,
              timeout=5.,
              search_url
            )

          if response.status_code != 200 then
            log.important(
              "Discogs cover lookup returned HTTP #{response.status_code}"
            )
            ""
          else
            let json.parse (
              result : {
                results: [{id: int, cover_image: string?}]
              }
            ) = response

            if list.length(result.results) == 0 then
              ""
            else
              match = list.hd(result.results)
              cover = match.cover_image ?? ""

              if cover != "" then
                cover
              else
                discogs_release_cover(match.id, headers)
              end
            end
          end
        catch err do
          log.important(
            "Discogs cover lookup failed: #{err.message}"
          )
          ""
        end

      discogs_cache.set({key=key, cover=cover})
      cover
    end
  end
end


# ===========================================================
# ICY metadata
# ===========================================================

def push_icy_metadata(m) =
  fields =
    if m["radio_jingle"] == "true" then
      [("song", "Jingle"), ("url", " ")]
    else
      cover = discogs_cover(m["artist"], m["title"])

      [
        ("song", m["song"]),
        ("url", cover)
      ]
    end

  icy.update_metadata(
    host=icecast_host,
    port=icecast_port,
    user=icecast_user,
    password=icecast_password,
    mount=icecast_mount,
    fields
  )
end


# ===========================================================
# S3 playlist
# ===========================================================

s3_tracks_since_jingle = ref(0)
s3_current_category = ref("tracks")
s3_stream_url = ref("http://127.0.0.1:8090/stream/tracks")


def next_s3_category() =
  tracks_since_jingle = s3_tracks_since_jingle()

  if tracks_since_jingle >= jingle_every then
    s3_tracks_since_jingle.set(0)
    "jingles"
  else
    s3_tracks_since_jingle.set(tracks_since_jingle + 1)
    "tracks"
  end
end


def switch_s3_stream() =
  category = next_s3_category()

  s3_current_category.set(category)
  s3_stream_url.set(
    "http://127.0.0.1:8090/stream/#{category}"
  )
end


def s3_metadata(m) =
  is_jingle =
    if m["Icy-Radio-Jingle"] == "true" then
      true
    elsif m["Icy-Radio-Jingle"] == "false" then
      false
    else
      s3_current_category() == "jingles"
    end

  updated_metadata =
    list.append(
      m,
      [
        (
          "radio_jingle",
          if is_jingle then "true" else "false" end
        )
      ]
    )

  if updated_metadata["title"] == ""
     and updated_metadata["Icy-Title"] != "" then

    list.append(
      updated_metadata,
      [
        ("title", updated_metadata["Icy-Title"])
      ]
    )
  else
    updated_metadata
  end
end


def s3_playlist() =
  initial_category = next_s3_category()

  s3_current_category.set(initial_category)
  s3_stream_url.set(
    "http://127.0.0.1:8090/stream/#{initial_category}"
  )

  stream =
    input.http(
      id="s3_playlist_http",
      timeout=15.,
      on_disconnect=switch_s3_stream,
      s3_stream_url
    )

  metadata.map(
    insert_missing=true,
    update=false,
    s3_metadata,
    stream
  )
end


# ===========================================================
# Playlists
# ===========================================================

rotation =
  if s3_enabled then

    s3_playlist()

  else

    tracks =
      playlist(
        id="playlist_tracks",
        check_next=is_audio_request,
        max_fail=1,
        mode="random",
        reload_mode="watch",
        "/app/storage/tracks"
      )

    jingle_playlist =
      playlist(
        id="playlist_jingles",
        check_next=is_audio_request,
        max_fail=1,
        mode="random",
        reload_mode="watch",
        "/app/storage/jingles"
      )

    jingles =
      metadata.map(
        update=false,
        fun(m) ->
          list.append(
            m,
            [("radio_jingle", "true")]
          ),
        jingle_playlist
      )

    rotate(
      weights=[jingle_every, 1],
      [tracks, jingles]
    )
  end


# ===========================================================
# Skip excessive silence
#
# Only a sufficiently long blank period causes the current
# track to be skipped.
#
# Silence inside a track is NOT removed immediately.
#
# 5 seconds of silence below -45 dB => skip current track.
# ===========================================================

rotation =
  blank.skip(
    id="radio_blank_skip",
    threshold=-45.,
    max_blank=5.,
    min_noise=0.2,
    track_sensitive=true,
    rotation
  )


# ===========================================================
# Safe source
# ===========================================================

safe_source =
  fallback(
    track_sensitive=false,
    [
      rotation,
      blank(id="radio_silence_fallback")
    ]
  )


# ===========================================================
# Audio processing and stream metadata
# ===========================================================

normalized_source =
  normalize(safe_source)


radio =
  mksafe(
    metadata.map(
      insert_missing=true,
      update=false,
      apply_metadata,
      normalized_source
    )
  )


radio.on_metadata(
  synchronous=false,
  push_icy_metadata
)


# ===========================================================
# Icecast output
# ===========================================================

output.icecast(
  %mp3(bitrate=stream_bitrate),

  host=icecast_host,
  port=icecast_port,
  user=icecast_user,
  password=icecast_password,
  mount=icecast_mount,

  name=icecast_name,
  genre=icecast_genre,
  description=icecast_description,
  url=icecast_url,

  # Metadata is updated manually to preserve StreamUrl
  # on Icecast clients.
  send_icy_metadata=false,

  radio
)
