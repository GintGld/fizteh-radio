Demo version of the radio.
==========================

The main purpose was to create a DASH manifest generator and check its correctness. A simple client was created to check it.

The structure of the directory:
------------------------------
- <font color='Orange'>main.go</font> Executable file. Serves simple server to send html page and initialize manifest updating
- <font color='Orange'>correct-example.mpd</font> 
[Reffering example](https://reference.dashif.org/dash.js/latest/samples/multiperiod/live.html) from dash.js tutorials
- <font color='orange'>go.mod</font>, <font color='orange'>go.sum</font> configs for Go
- <font color='orange'>index.html</font> client
- stream
    - <font color='orange'>ffmpeg.go</font> wrapper for ffmpeg library, which is used to working with audio files.
    - <font color='orange'>manifest.go</font> API for editing internal manifest representation
    - <font color='orange'>manifest.go</font> includes high-level and public functions
    - <font color='orange'>store.go</font> stub for content generator

> Note: to start the demo one should move <font color='orange'>index.html</font> to tmp dir and change paths to the content in <font color='orange'>main.go</font>