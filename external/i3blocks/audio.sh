#!/bin/sh

audio_volume=$(pamixer --get-volume-human)
audio_is_muted=$(pamixer --get-mute)

if [ $audio_is_muted = "true" ] ; then
    audio_active=''
else
    audio_active=''
fi

echo "$audio_active  $audio_volume"