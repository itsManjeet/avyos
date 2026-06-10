// Metal GPU canvas — macOS/iOS, compiled with Objective-C.
// Each draw call appends a render pass to a shared command buffer.
// The buffer is committed only when metal_canvas_pixels() is called.

#import <Metal/Metal.h>
#import <Foundation/Foundation.h>
#include "metal.h"
#include <stdlib.h>
#include <string.h>


// 48 bytes, 16-byte aligned.
typedef struct __attribute__((packed)) {
    float color[4];   // RGBA
    float bounds[4];  // x, y, w, h  (pixel coords)
    float screen[2];  // canvas width, height
    float radius;     // corner radius (0 for sharp rect, >0 for rounded/circle)
    float _pad;
} FillParams;


static NSString* const kShaderSrc = @
"#include <metal_stdlib>\n"
"using namespace metal;\n"
"\n"
"struct FillParams {\n"
"    float4 color;\n"
"    float4 bounds;   // x, y, w, h (pixel)\n"
"    float2 screen;   // canvas size (pixel)\n"
"    float  radius;\n"
"    float  _pad;\n"
"};\n"
"\n"
"struct Vert {\n"
"    float4 pos [[position]];\n"
"    float2 uv;  // pixel coord within bounds\n"
"};\n"
"\n"
"// Quad: vertex ids 0-3 form a triangle-strip covering bounds.\n"
"vertex Vert fill_vert(uint vid [[vertex_id]],\n"
"                      constant FillParams& p [[buffer(0)]]) {\n"
"    float2 corner = float2((vid & 1) ? p.bounds.x + p.bounds.z : p.bounds.x,\n"
"                           (vid & 2) ? p.bounds.y + p.bounds.w : p.bounds.y);\n"
"    float2 ndc = corner / p.screen * 2.0 - 1.0;\n"
"    ndc.y = -ndc.y;  // flip Y for Metal (NDC origin bottom-left, pixels top-left)\n"
"    Vert v;\n"
"    v.pos = float4(ndc, 0, 1);\n"
"    v.uv  = corner - p.bounds.xy;  // local coord within bounds\n"
"    return v;\n"
"}\n"
"\n"
"// Sharp rectangle fill.\n"
"fragment float4 solid_frag(Vert in [[stage_in]],\n"
"                            constant FillParams& p [[buffer(0)]]) {\n"
"    return p.color;\n"
"}\n"
"\n"
"// Rounded rect / circle SDF fill.\n"
"fragment float4 sdf_frag(Vert in [[stage_in]],\n"
"                          constant FillParams& p [[buffer(0)]]) {\n"
"    float2 half_sz = p.bounds.zw * 0.5;\n"
"    float2 local   = in.uv - half_sz;  // centre-relative coord\n"
"    float  r       = min(p.radius, min(half_sz.x, half_sz.y));\n"
"    float2 q       = abs(local) - half_sz + r;\n"
"    float  d       = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - r;\n"
"    float  alpha   = clamp(0.5 - d, 0.0, 1.0);\n"
"    return float4(p.color.rgb, p.color.a * alpha);\n"
"}\n"
"\n"
"// Blit: copy from CPU staging texture onto the render target.\n"
"struct BlitVert {\n"
"    float4 pos [[position]];\n"
"    float2 uv;\n"
"};\n"
"vertex BlitVert blit_vert(uint vid [[vertex_id]],\n"
"                           constant float4& rect [[buffer(0)]],\n"  // x,y,w,h in NDC already
"                           constant float2& screen [[buffer(1)]]) {\n"
"    float2 corner = float2((vid & 1) ? rect.x + rect.z : rect.x,\n"
"                           (vid & 2) ? rect.y + rect.w : rect.y);\n"
"    BlitVert v;\n"
"    v.pos = float4(corner, 0, 1);\n"
"    // UV y is flipped relative to NDC y: NDC bottom (vid&2==0) → staging top (UV.y=1),\n"
"    // NDC top (vid&2!=0) → staging top (UV.y=0).  Metal NDC has Y-up; the CPU canvas\n"
"    // is Y-down (row 0 = top).  The fill_vert shaders correct this with ndc.y=-ndc.y;\n"
"    // the blit must do the equivalent by flipping UV.y so row 0 of the CPU image\n"
"    // ends up at texture row 0 (NDC top, raster top).\n"
"    v.uv  = float2((vid & 1) ? 1.0 : 0.0, (vid & 2) ? 0.0 : 1.0);\n"
"    return v;\n"
"}\n"
"fragment float4 blit_frag(BlitVert in [[stage_in]],\n"
"                           texture2d<float> tex [[texture(0)]]) {\n"
"    constexpr sampler s(coord::normalized, filter::nearest);\n"
"    return tex.sample(s, in.uv);\n"
"}\n";


struct MetalCanvas {
    id<MTLDevice>              device;
    id<MTLCommandQueue>        queue;
    id<MTLRenderPipelineState> solidPipeline;   // sharp rect
    id<MTLRenderPipelineState> sdfPipeline;     // rounded / circle
    id<MTLRenderPipelineState> blitPipeline;    // CPU blit
    id<MTLTexture>             texture;         // render target (Managed)
    id<MTLCommandBuffer>       cmdBuf;          // accumulated this frame
    MTLRenderPassDescriptor*   firstPass;       // nil after first draw
    int                        width, height;
    BOOL                       usedLoad;        // whether we've issued ≥1 draw
};


static id<MTLRenderPipelineState> make_pipeline(id<MTLDevice> dev,
                                                 id<MTLLibrary> lib,
                                                 NSString* vertFn,
                                                 NSString* fragFn) {
    MTLRenderPipelineDescriptor* d = [MTLRenderPipelineDescriptor new];
    d.vertexFunction   = [lib newFunctionWithName:vertFn];
    d.fragmentFunction = [lib newFunctionWithName:fragFn];
    d.colorAttachments[0].pixelFormat                 = MTLPixelFormatRGBA8Unorm;
    d.colorAttachments[0].blendingEnabled             = YES;
    d.colorAttachments[0].sourceRGBBlendFactor        = MTLBlendFactorSourceAlpha;
    d.colorAttachments[0].destinationRGBBlendFactor   = MTLBlendFactorOneMinusSourceAlpha;
    d.colorAttachments[0].sourceAlphaBlendFactor      = MTLBlendFactorOne;
    d.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
    NSError* err = nil;
    id<MTLRenderPipelineState> ps = [dev newRenderPipelineStateWithDescriptor:d error:&err];
    if (!ps) { NSLog(@"metal: pipeline error: %@", err); }
    return ps;
}

static id<MTLTexture> make_texture(id<MTLDevice> dev, int w, int h) {
    MTLTextureDescriptor* td = [MTLTextureDescriptor
        texture2DDescriptorWithPixelFormat:MTLPixelFormatRGBA8Unorm
                                     width:(NSUInteger)w
                                    height:(NSUInteger)h
                                 mipmapped:NO];
    td.usage        = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead;
    td.storageMode  = MTLStorageModeManaged;
    return [dev newTextureWithDescriptor:td];
}

// Begin a new render pass on the persistent command buffer.
// loadAction: Clear (first time) or Load (subsequent).
static id<MTLRenderCommandEncoder> begin_pass(struct MetalCanvas* c,
                                               MTLLoadAction load,
                                               MTLClearColor clearCol) {
    MTLRenderPassDescriptor* rp = [MTLRenderPassDescriptor new];
    rp.colorAttachments[0].texture     = c->texture;
    rp.colorAttachments[0].loadAction  = load;
    rp.colorAttachments[0].storeAction = MTLStoreActionStore;
    rp.colorAttachments[0].clearColor  = clearCol;
    return [c->cmdBuf renderCommandEncoderWithDescriptor:rp];
}

// Issue a fill draw call (solid or sdf pipeline).
static void fill_draw(struct MetalCanvas* c,
                      id<MTLRenderPipelineState> pipeline,
                      FillParams* p,
                      MTLLoadAction load,
                      MTLClearColor clearCol) {
    id<MTLRenderCommandEncoder> enc = begin_pass(c, load, clearCol);
    [enc setRenderPipelineState:pipeline];
    [enc setVertexBytes:p length:sizeof(FillParams) atIndex:0];
    [enc setFragmentBytes:p length:sizeof(FillParams) atIndex:0];
    [enc drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
    [enc endEncoding];
}


MetalCanvas* metal_canvas_create(int width, int height) {
    id<MTLDevice> dev = MTLCreateSystemDefaultDevice();
    if (!dev) return NULL;

    NSError* err = nil;
    MTLCompileOptions* opts = [MTLCompileOptions new];
    id<MTLLibrary> lib = [dev newLibraryWithSource:kShaderSrc options:opts error:&err];
    if (!lib) { NSLog(@"metal: shader compile: %@", err); return NULL; }

    struct MetalCanvas* c = calloc(1, sizeof(struct MetalCanvas));
    c->device       = dev;
    c->queue        = [dev newCommandQueue];
    c->solidPipeline = make_pipeline(dev, lib, @"fill_vert",  @"solid_frag");
    c->sdfPipeline   = make_pipeline(dev, lib, @"fill_vert",  @"sdf_frag");
    c->blitPipeline  = make_pipeline(dev, lib, @"blit_vert",  @"blit_frag");
    c->texture      = make_texture(dev, width, height);
    c->width        = width;
    c->height       = height;
    c->cmdBuf       = [c->queue commandBuffer];
    c->usedLoad     = NO;
    if (!c->solidPipeline || !c->sdfPipeline || !c->blitPipeline || !c->texture) {
        free(c);
        return NULL;
    }
    return c;
}

void metal_canvas_destroy(MetalCanvas* c) {
    if (!c) return;
    free(c);
}

void metal_canvas_clear(MetalCanvas* c, float r, float g, float b, float a) {
    // A Clear load action wipes the texture — no draw needed.
    // We start a trivially-short render pass with loadAction=Clear.
    MTLClearColor cc = MTLClearColorMake(r, g, b, a);
    MTLRenderPassDescriptor* rp = [MTLRenderPassDescriptor new];
    rp.colorAttachments[0].texture     = c->texture;
    rp.colorAttachments[0].loadAction  = MTLLoadActionClear;
    rp.colorAttachments[0].storeAction = MTLStoreActionStore;
    rp.colorAttachments[0].clearColor  = cc;
    id<MTLRenderCommandEncoder> enc = [c->cmdBuf renderCommandEncoderWithDescriptor:rp];
    [enc endEncoding];
    c->usedLoad = YES;
}

void metal_canvas_fill_rect(MetalCanvas* c,
                             float x, float y, float w, float h,
                             float r, float g, float b, float a) {
    MTLLoadAction load = c->usedLoad ? MTLLoadActionLoad : MTLLoadActionDontCare;
    FillParams p = {
        .color  = {r, g, b, a},
        .bounds = {x, y, w, h},
        .screen = {(float)c->width, (float)c->height},
        .radius = 0.0f,
    };
    fill_draw(c, c->solidPipeline, &p, load, MTLClearColorMake(0,0,0,0));
    c->usedLoad = YES;
}

void metal_canvas_fill_rounded_rect(MetalCanvas* c,
                                     float x, float y, float w, float h,
                                     float radius,
                                     float r, float g, float b, float a) {
    MTLLoadAction load = c->usedLoad ? MTLLoadActionLoad : MTLLoadActionDontCare;
    FillParams p = {
        .color  = {r, g, b, a},
        .bounds = {x, y, w, h},
        .screen = {(float)c->width, (float)c->height},
        .radius = radius,
    };
    fill_draw(c, c->sdfPipeline, &p, load, MTLClearColorMake(0,0,0,0));
    c->usedLoad = YES;
}

void metal_canvas_fill_circle(MetalCanvas* c,
                               float cx, float cy, float radius,
                               float r, float g, float b, float a) {
    // Encode a circle as a rounded rect where w=h=2*radius.
    float x = cx - radius, y = cy - radius;
    float d = radius * 2.0f;
    metal_canvas_fill_rounded_rect(c, x, y, d, d, radius, r, g, b, a);
}

void metal_canvas_blit_cpu(MetalCanvas* c,
                            const uint8_t* rgba, int src_w, int src_h,
                            int dst_x, int dst_y) {
    // Upload CPU bytes to a temporary texture, then blit onto the render target.
    MTLTextureDescriptor* td = [MTLTextureDescriptor
        texture2DDescriptorWithPixelFormat:MTLPixelFormatRGBA8Unorm
                                     width:(NSUInteger)src_w
                                    height:(NSUInteger)src_h
                                 mipmapped:NO];
    td.usage       = MTLTextureUsageShaderRead;
    td.storageMode = MTLStorageModeManaged;
    id<MTLTexture> staging = [c->device newTextureWithDescriptor:td];
    MTLRegion region = MTLRegionMake2D(0, 0, (NSUInteger)src_w, (NSUInteger)src_h);
    [staging replaceRegion:region
               mipmapLevel:0
                 withBytes:rgba
               bytesPerRow:(NSUInteger)(src_w * 4)];
#ifdef TARGET_OS_MAC
    id<MTLBlitCommandEncoder> sync = [c->cmdBuf blitCommandEncoder];
    [sync synchronizeResource:staging];
    [sync endEncoding];
#endif

    // Compute NDC rect for the destination region.
    float W = (float)c->width, H = (float)c->height;
    float nx  = (float)dst_x / W * 2.0f - 1.0f;
    float ny  = -((float)dst_y / H * 2.0f - 1.0f) - (float)src_h / H * 2.0f;
    float nw  = (float)src_w / W * 2.0f;
    float nh  = (float)src_h / H * 2.0f;
    float rect[4] = {nx, ny, nw, nh};
    float screen[2] = {W, H};

    MTLLoadAction load = c->usedLoad ? MTLLoadActionLoad : MTLLoadActionDontCare;
    MTLRenderPassDescriptor* rp = [MTLRenderPassDescriptor new];
    rp.colorAttachments[0].texture     = c->texture;
    rp.colorAttachments[0].loadAction  = load;
    rp.colorAttachments[0].storeAction = MTLStoreActionStore;
    id<MTLRenderCommandEncoder> enc = [c->cmdBuf renderCommandEncoderWithDescriptor:rp];
    [enc setRenderPipelineState:c->blitPipeline];
    [enc setVertexBytes:rect   length:sizeof(rect)   atIndex:0];
    [enc setVertexBytes:screen length:sizeof(screen) atIndex:1];
    [enc setFragmentTexture:staging atIndex:0];
    [enc drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
    [enc endEncoding];
    c->usedLoad = YES;
}

void metal_canvas_pixels(MetalCanvas* c, uint8_t* out) {
    // Synchronise Managed texture back to CPU.
    id<MTLBlitCommandEncoder> blit = [c->cmdBuf blitCommandEncoder];
    [blit synchronizeResource:c->texture];
    [blit endEncoding];

    [c->cmdBuf commit];
    [c->cmdBuf waitUntilCompleted];

    MTLRegion region = MTLRegionMake2D(0, 0, (NSUInteger)c->width, (NSUInteger)c->height);
    [c->texture getBytes:out
             bytesPerRow:(NSUInteger)(c->width * 4)
              fromRegion:region
             mipmapLevel:0];

    // Reset for the next frame.
    c->cmdBuf   = [c->queue commandBuffer];
    c->usedLoad = NO;
}

int metal_canvas_width(MetalCanvas* c)  { return c->width; }
int metal_canvas_height(MetalCanvas* c) { return c->height; }
