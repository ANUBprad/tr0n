# TRON demo artifacts

## What is here

| File | Kind | Notes |
|---|---|---|
| `tron-walkthrough.mp4` | Edited captioned video | **Old.** Five captured UI states from the Docker app, produced **before** the optional local-model mode and the newer source-synchronization features. It is a captioned sequence of screenshots, **not** a continuous live screen recording. Do not relabel it as a new live recording. |
| `01-ownership.png` … `05-handoff.png` | Screenshots | Frames used to build the walkthrough above. |
| `captions.srt`, `frames.txt`, `video-check.png` | Captions/check | Caption track and a frame check for the walkthrough above. |

## Fresh end-to-end demo — BLOCKED, manual step required

A new, continuous, judge-accessible recording (3–5 minutes) of the current
running product was **not** produced in this environment: it has no screen
recorder and no interactive desktop session. To avoid fabricating a video, the
old screenshots were not turned into a supposedly live recording and
`tron-walkthrough.mp4` was not relabelled.

**Manual step to record the new demo (on a machine with a recorder):**

1. Follow `docs/DEMO_SCRIPT.md` → *Before recording* to reach a clean, seeded,
   7/7-verified state.
2. Start a screen recorder (e.g. OBS, Xbox Game Bar `Win+G`, or `ffmpeg` with a
   desktop capture) capturing the browser and one terminal.
3. Perform the narration in `docs/DEMO_SCRIPT.md` top to bottom in one take.
4. Save the **unedited** file as `artifacts/demo/tron-live-<date>.mp4`, verify it
   opens and plays, and add captions (`captions.srt`) if your recorder supports
   them.
5. Only after the file exists and plays, update `docs/DEMO_SCRIPT.md` and the
   demo link in `README.md`.

If a shorter edit is made, it must be clearly labelled **edited** and the full
unedited file retained as the source artifact.
