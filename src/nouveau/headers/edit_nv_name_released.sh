#!/usr/bin/bash
set -e

sed /\"\(null\)\"/d -i nvidia/g_nv_name_released.h
