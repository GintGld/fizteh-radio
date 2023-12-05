Demo version of the radio.
==========================

The main purpose was to create a DASH manifest generator and check its correctness. A simple client was created to check it.

The structure of the directory:
------------------------------
- **main.go** Executable file. Serves simple server to send html page and initialize manifest updating
- **correct-example.mpd** 
[Reffering example](https://reference.dashif.org/dash.js/latest/samples/multiperiod/live.html) from dash.js tutorials
- **go.mod**, **go.sum** configs for Go
- **index.html** client
- stream
    - **ffmpeg.go** wrapper for ffmpeg library, which is used to working with audio files.
    - **manifest.go** API for editing internal manifest representation
    - **manifest.go** includes high-level and public functions
    - **store.go** stub for content generator

> Note: to start the demo one should change content paths in **main.go**