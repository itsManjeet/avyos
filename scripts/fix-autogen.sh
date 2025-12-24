#!/bin/sh

for dir in automake autoconf ; do
    (cd _external/$dir; autoreconf -fiv)
done