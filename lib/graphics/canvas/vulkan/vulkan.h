#pragma once
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct VkCanvas VkCanvas;

// Create a headless Vulkan canvas.  Returns NULL on failure.
VkCanvas* vk_canvas_create(int width, int height);

// Destroy and free.
void vk_canvas_destroy(VkCanvas* c);

// Clear the canvas to the given RGBA colour (0.0–1.0 each).
void vk_canvas_clear(VkCanvas* c, float r, float g, float b, float a);

// Fill an axis-aligned rectangle.
void vk_canvas_fill_rect(VkCanvas* c,
                          float x, float y, float w, float h,
                          float r, float g, float b, float a);

// Fill a rounded rectangle (radius in pixels).
void vk_canvas_fill_rounded_rect(VkCanvas* c,
                                  float x, float y, float w, float h,
                                  float radius,
                                  float r, float g, float b, float a);

// Fill a circle (cx,cy = centre).
void vk_canvas_fill_circle(VkCanvas* c,
                            float cx, float cy, float radius,
                            float r, float g, float b, float a);

// Blit a CPU RGBA image at (dst_x, dst_y).
void vk_canvas_blit_cpu(VkCanvas* c,
                         const uint8_t* rgba, int src_w, int src_h,
                         int dst_x, int dst_y);

// Readback pixel data (caller must supply width*height*4 bytes).
void vk_canvas_pixels(VkCanvas* c, uint8_t* out);

int vk_canvas_width(VkCanvas* c);
int vk_canvas_height(VkCanvas* c);

#ifdef __cplusplus
}
#endif
