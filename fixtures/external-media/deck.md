# External media trust boundary

This fixture must prompt before the presentation opens because it combines an
outside-deck local image with remote media. The image below is intentionally
stored in the sibling `fixtures/assets` directory, rather than beside this
deck.

![Outside-deck local image](../assets/rendering-flow.png "Embedded after trust")

---

## Remote media remains remote

The browser fetches remote media directly; the local server must not proxy it.

![Remote image](https://example.com/md-present-remote-image.png "Remote image")

![Remote video](https://example.com/md-present-remote-video.mp4 "Remote video")

Run with `--allow-external-media` only when these references are trusted.
