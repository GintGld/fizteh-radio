#!/bin/bash

ffmpeg -hide_banner -y \
    -f alsa -i hw:1,0 \
    -c:a aac \
    -b:a $BITRATE \
    -ac $CHANNELS \
    -ar $SAMPLING_RATE \
    -dash_segment_type mp4 \
    -use_template 1 \
    -use_timeline 0 \
    -init_seg_name $INIT_NAME \
    -media_seg_name $SEGMENT_NAME \
    -seg_duration $SEGMENT_DURATION \
    -f dash $OUTPUT

