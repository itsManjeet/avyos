/*
 * Copyright © Microsoft Corporation
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice (including the next
 * paragraph) shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
 * IN THE SOFTWARE.
 */

#include <guiddef.h>

#if MFT_CODEC_H264ENC
const wchar_t *g_pMFTFriendlyName = L"Microsoft AVC DX12 Encoder HMFT";
DEFINE_GUID( CLSID_CDX12EncoderHMFT, 0x8994db7c, 0x288a, 0x4c62, 0xa1, 0x36, 0xa3, 0xc3, 0xc2, 0xa2, 0x08, 0xa8 );
#elif MFT_CODEC_H265ENC
const wchar_t *g_pMFTFriendlyName = L"Microsoft HEVC DX12 Encoder HMFT";
DEFINE_GUID( CLSID_CDX12EncoderHMFT, 0xe7ffb8eb, 0xfa0b, 0x4fb0, 0xac, 0xdf, 0x12, 0x2, 0xf6, 0x63, 0xcd, 0xe5 );
#elif MFT_CODEC_AV1ENC
const wchar_t *g_pMFTFriendlyName = L"Microsoft AV1 DX12 Encoder HMFT";
DEFINE_GUID( CLSID_CDX12EncoderHMFT, 0x1a6f3150, 0xb121, 0x4ce9, 0x94, 0x97, 0x50, 0xfe, 0xdb, 0x3d, 0xcb, 0x70 );
#else
#error MFT_CODEC_xxx must be defined
#endif

extern "C" struct pipe_screen *
sw_screen_create_vk( struct sw_winsys *winsys, const struct pipe_screen_config *config, bool sw_vk )
{
   return nullptr;
}