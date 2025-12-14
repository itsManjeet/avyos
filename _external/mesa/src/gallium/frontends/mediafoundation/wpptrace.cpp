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
#include "wpptrace.h"

#include "wpptrace.tmh"


#if MFT_CODEC_H264ENC

TRACELOGGING_DEFINE_PROVIDER(   // defines g_hProvider
   g_hEtwProvider,              // Name of the provider handle
   "h264enc.etw",               // Human-readable name for the provider
                                // 0000e264-0dc9-401d-b9b8-05e4eca4977e
   ( 0x0000e264, 0x0dc9, 0x401d, 0xb9, 0xb8, 0x05, 0xe4, 0xec, 0xa4, 0x97, 0x7e ) );

#elif MFT_CODEC_H265ENC

TRACELOGGING_DEFINE_PROVIDER(   // defines g_hProvider
   g_hEtwProvider,              // Name of the provider handle
   "h265enc.etw",               // Human-readable name for the provider
                                // 0000e265-0dc9-401d-b9b8-05e4eca4977e
   ( 0x0000e265, 0x0dc9, 0x401d, 0xb9, 0xb8, 0x05, 0xe4, 0xec, 0xa4, 0x97, 0x7e ) );

#elif MFT_CODEC_AV1ENC

TRACELOGGING_DEFINE_PROVIDER(   // defines g_hProvider
   g_hEtwProvider,              // Name of the provider handle
   "av1enc.etw",                // Human-readable name for the provider
                                // 0000eaa1-0dc9-401d-b9b8-05e4eca4977e
   ( 0x0000eaa1, 0x0dc9, 0x401d, 0xb9, 0xb8, 0x05, 0xe4, 0xec, 0xa4, 0x97, 0x7e ) );

#else
#error MFT_CODEC_xxx must be defined
#endif

void
WppInit()
{
   TraceLoggingRegister( g_hEtwProvider );

   WPP_INIT_TRACING( L"MFEncoder" );
   MFE_INFO( "MFEncoder trace is enabled." );
}

void
WppClean()
{
   MFE_INFO( "MFEncoder trace is shutdown." );
   WPP_CLEANUP();

   TraceLoggingUnregister( g_hEtwProvider );
}
