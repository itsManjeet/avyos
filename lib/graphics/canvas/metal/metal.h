#pragma once
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct MetalCanvas MetalCanvas;

// Create a Metal canvas of the given pixel dimensions.
// Returns NULL on failure (no Metal device, etc.).
MetalCanvas* metal_canvas_create(int width, int height);

// Destroy and free the canvas.
void metal_canvas_destroy(MetalCanvas* c);

// Clear the entire canvas to the given RGBA colour (each component 0.0–1.0).
void metal_canvas_clear(MetalCanvas* c, float r, float g, float b, float a);

// Fill an axis-aligned rectangle.
void metal_canvas_fill_rect(MetalCanvas* c,
                             float x, float y, float w, float h,
                             float r, float g, float b, float a);

// Fill a rounded rectangle (radius in pixels, clamped to min(w,h)/2).
void metal_canvas_fill_rounded_rect(MetalCanvas* c,
                                     float x, float y, float w, float h,
                                     float radius,
                                     float r, float g, float b, float a);

// Fill a circle (cx,cy = centre, radius in pixels).
void metal_canvas_fill_circle(MetalCanvas* c,
                               float cx, float cy, float radius,
                               float r, float g, float b, float a);

// Blit an RGBA CPU image onto the canvas at (dx,dy).
// src_w/src_h are the dimensions of the source image.
void metal_canvas_blit_cpu(MetalCanvas* c,
                            const uint8_t* rgba, int src_w, int src_h,
                            int dst_x, int dst_y);

// Resolve the canvas to a flat RGBA byte array (caller-supplied, must be
// width*height*4 bytes).  After this call the canvas is ready for a new frame.
void metal_canvas_pixels(MetalCanvas* c, uint8_t* out);

// Dimensions.
int metal_canvas_width(MetalCanvas* c);
int metal_canvas_height(MetalCanvas* c);

#ifdef __cplusplus
}
#endif
