#pragma once

#include <stdio.h>
#include <stdbool.h>
#include <pixman.h>

bool svg_load(FILE *fp, const char *path);
pixman_image_t *svg_render(const int width, const int height, bool stretch);
void svg_free(void);
