tracks_since_jingle = ref(0)
category = ref("tracks")
stream_url = ref("http://127.0.0.1:8090/stream/tracks")

def next_category() =
  if tracks_since_jingle() >= 1 then
    tracks_since_jingle.set(0)
    "jingles"
  else
    tracks_since_jingle.set(tracks_since_jingle() + 1)
    "tracks"
  end
end

def change_stream() =
  next_kind = next_category()
  category.set(next_kind)
  stream_url.set("http://127.0.0.1:8090/stream/#{next_kind}")
end

def stream_metadata(m) =
  is_jingle =
    if m["Icy-Radio-Jingle"] == "true" then
      true
    elsif m["Icy-Radio-Jingle"] == "false" then
      false
    else
      category() == "jingles"
    end
  updated_metadata = list.append(
    m,
    [
      (
        "radio_jingle",
        if is_jingle then "true" else "false" end
      )
    ]
  )
  if updated_metadata["title"] == "" and updated_metadata["Icy-Title"] != "" then
    list.append(updated_metadata, [("title", updated_metadata["Icy-Title"])])
  else
    updated_metadata
  end
end

stream = input.http(
  id="s3_stream_smoke_test",
  timeout=10.,
  on_connect=fun(headers) -> log.important("S3 response headers: #{headers}"),
  on_disconnect=change_stream,
  stream_url
)

radio = metadata.map(
  insert_missing=true,
  stream_metadata,
  stream
)

radio.on_metadata(synchronous=false, fun(m) -> log.important("S3 metadata: #{m}"))

output.dummy(fallible=true, mksafe(radio))
