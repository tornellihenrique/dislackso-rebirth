# ADR 0002: Screen share codecs and layers

- Status: accepted
- Date: 2026-10-15

## Context

ARCHITECTURE §9.5 planned VP9 with SVC (`L3T3_KEY`) and a VP8 backup as the screen share default,
with H.264 and AV1 as advanced options, and asked for the defaults to be validated with
measurements in Phase 4. Two facts about LiveKit came up while building it (livekit-client 2.22.4,
livekit-server 1.13.9):

- Asked for SVC on a **screen share**, the client publishes VP9 with a single spatial layer
  (`L1T3`) and forces the `motion` content hint, because Chrome's screen-content path for VP9 SVC
  misbehaves. So SVC gives grid tiles no smaller layer, and loses the text tuning.
- Dynacast pauses whole **simulcast** layers when nobody watches them (`encoding.active = false`),
  but never SVC spatial layers. With several shares in a grid, an SVC sharer keeps encoding full
  size for nobody.

Since server 1.13.7, LiveKit also accepts VP9 and AV1 as simulcast, one `L1T3` stream per layer.

## Measurements

`task bench:share` (e2e/bench) runs a fresh hub and LiveKit, shares synthetic screens from
Chromium with the GPU, and reads WebRTC stats on both ends. Hardware: Intel Core Ultra 9 275HX,
NVIDIA RTX 5070 laptop GPU, Windows 11. Chromium's test build exposes no hardware encoder, so all
encoders ran in software (libvpx, OpenH264, libaom); decoding used the GPU except for VP8. The
synthetic screen was captured at ~50 fps (drawing in a busy page tops out there), so frame rates
below are out of what was captured.

Scenes: **edit**, a code editor while someone types, with a short scroll every 3 s; **scroll**,
the whole editor scrolling every frame, the worst case for text; **motion**, a 3D-viewport-like
scene where every pixel moves every frame. 1440p60 preset, Text optimization for edit and scroll
and Motion for motion; one viewer in focus large enough for the full layer. "CPU" is the whole
sharer browser in cores (it includes drawing the scene, the same for every codec).

| Scene | Codec | Captured → sent (2560×1440) | Full layer | Encode, full layer | Sharer CPU | Viewer |
|---|---|---|---|---|---|---|
| edit | VP9 | 50 → 34 fps | 1.8 Mbps | 6.8 ms/frame | 0.93 | 34 fps |
| edit | VP8 | 50 → 47 fps | 2.9 Mbps | 5.1 ms/frame | 1.07 | 47 fps, 1 freeze, software decode |
| edit | H.264 | 50 → 48 fps | 0.3 Mbps | 11.6 ms/frame | 0.91 | 48 fps |
| edit | **AV1** | 50 → **48 fps** | **0.5 Mbps** | 4.2 ms/frame | 0.91 | 48 fps |
| scroll | VP9 | 50 → 32 fps | 2.5 Mbps | 10.1 ms/frame | 1.54 | 32 fps |
| scroll | VP8 | 48 → 47 fps | 4.6 Mbps | 6.7 ms/frame | 1.65 | 47 fps, software decode |
| scroll | H.264 | 43 → 37 fps | 1.1 Mbps | 22.0 ms/frame (CPU-limited) | 1.39 | 37 fps |
| scroll | **AV1** | 49 → **48 fps** | **1.6 Mbps** | 5.9 ms/frame | 1.37 | 48 fps |
| motion | **VP9** | 51 → **49 fps** | 9.6 Mbps | 10.9 ms/frame | **1.99** | 49 fps |
| motion | VP8 | 51 → 37 fps | 4.6 Mbps | 9.0 ms/frame | 1.33 | 37 fps, 3 freezes |
| motion | H.264 | 49 → 47 fps | 8.7 Mbps | 15.8 ms/frame (88 % busy) | 1.17 | 47 fps |
| motion | AV1 | 51 → 49 fps | 9.5 Mbps | 10.8 ms/frame | 2.72 | 49 fps |

The low layer (1280×720, ≤ 30 fps) cost 2–5 ms/frame more on every codec.

Layers, with three 1080p30 shares and five more viewers, everyone in the grid (window
1280×800): every viewer received all three shares at 960×540 (the low layer, ~29 fps,
~0.4 Mbps each), and every sharer's full layer was paused (0 kbps). When one viewer focused a
share in a large window, that sharer's 1920×1080 layer resumed (29 fps, 1.7 Mbps) for that viewer
only; the other two stayed paused.

## Decision

- Every codec is published as **two simulcast layers**: full size, and half size at ≤ 30 fps and
  a quarter of the bitrate (at least 300 kbps). VP9 and AV1 use `L1T3` in each layer. No
  single-stream SVC.
- **Automatic** codec (the default setting):
  - **Text: AV1**, else VP9, else VP8. With text tuning VP9 drops a third of the frames even while
    someone types; AV1 keeps the captured frame rate at a third of VP9's bitrate and about the
    same CPU.
  - **Motion: VP9**, else VP8. VP9 and AV1 both keep the frame rate, but AV1 needs ~40 % more CPU;
    VP8 drops frames and stutters; software H.264 is near its limit.
- **VP8 backup** for every other codec: a viewer whose browser cannot decode the choice (Safari
  and AV1, for example) gets VP8.
- H.264, VP8, VP9 and AV1 stay choosable in Settings → Screen share. H.264 is the one to try on a
  slow machine with a hardware encoder, which this benchmark could not measure.

## Consequences

- Grids of several shares cost each sharer only the low layer, and the host only low-layer
  egress, until someone focuses a share.
- AV1 sharing needs a browser that encodes AV1 (Chromium-based desktop browsers do); others fall
  back as above. Viewers without hardware AV1 decoding use dav1d, which is fast.
- The numbers come from one fast PC with synthetic screens. The defaults are one constant
  (`AUTO_CODECS` in `apps/web/src/core/media/share-presets.ts`); revisit them after the team
  shares real screens over the internet (Phase 5), with the stats overlay and `task bench:share`.

## Update (2026-10-17)

The team's first real use asked for one choice less: a quality preset already names a resolution
and a frame rate, so "Optimize for" went away. The preset's frame rate sets the tuning: 30 fps
presets get Text (and AV1 under Automatic), 60 fps presets get Motion (and VP9), each with the
bitrate of that column above. `task bench:share` now shares every scene at 1440p60, as people
pick it.
