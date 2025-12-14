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

#pragma once

#include <windows.h>

#include <TraceLoggingProvider.h>

#if MFT_CODEC_H264ENC
#define DEFINE_MFE_WPP_GUID ( 264, 0dc9, 401d, b9b8, 05e4eca4977e )
#elif MFT_CODEC_H265ENC
#define DEFINE_MFE_WPP_GUID ( 265, 0dc9, 401d, b9b8, 05e4eca4977e )
#elif MFT_CODEC_AV1ENC
#define DEFINE_MFE_WPP_GUID ( aa1, 0dc9, 401d, b9b8, 05e4eca4977e )
#else
#error MFT_CODEC_xxx must be defined
#endif

#define WPP_CONTROL_GUIDS WPP_DEFINE_CONTROL_GUID( CTRLGUID_MFE, DEFINE_MFE_WPP_GUID, WPP_DEFINE_BIT( MFE_ALL ) )

#define WPP_FLAG_LEVEL_LOGGER( flag, level ) WPP_LEVEL_LOGGER( flag )

#define WPP_FLAG_LEVEL_ENABLED( flag, level ) ( WPP_LEVEL_ENABLED( flag ) && WPP_CONTROL( WPP_BIT_##flag ).Level >= level )

#define WPP_LEVEL_FLAGS_LOGGER( lvl, flags ) WPP_LEVEL_LOGGER( flags )

#define WPP_LEVEL_FLAGS_ENABLED( lvl, flags ) ( WPP_LEVEL_ENABLED( flags ) && WPP_CONTROL( WPP_BIT_##flags ).Level >= lvl )


// begin_wpp config
//
// FUNC MFE_INFO{FLAG=MFE_ALL,LEVEL=TRACE_LEVEL_INFORMATION}(MSG, ...);
// FUNC MFE_ERROR{FLAG=MFE_ALL,LEVEL=TRACE_LEVEL_ERROR}(MSG, ...);
// FUNC MFE_WARNING{FLAG=MFE_ALL,LEVEL=TRACE_LEVEL_WARNING}(MSG, ...);
// FUNC MFE_VERBOSE{FLAG=MFE_ALL,LEVEL=TRACE_LEVEL_VERBOSE}(MSG, ...);
//
// end_wpp

//
// TraceLogging ETW
//

TRACELOGGING_DECLARE_PROVIDER( g_hEtwProvider );


#if MFT_CODEC_H264ENC

#define ETW_MODULE_STR "H264Enc"

#elif MFT_CODEC_H265ENC

#define ETW_MODULE_STR "H265Enc"

#elif MFT_CODEC_AV1ENC

#define ETW_MODULE_STR "AV1Enc"

#else
#error MFT_CODEC_xxx must be defined
#endif

#define HMFT_ETW_EVENT_START( EventId, this )                                                                                      \
   if( g_hEtwProvider )                                                                                                            \
   {                                                                                                                               \
      TraceLoggingWrite( g_hEtwProvider,                                                                                           \
                         EventId,                                                                                                  \
                         TraceLoggingOpcode( EVENT_TRACE_TYPE_START ),                                                             \
                         TraceLoggingPointer( this, ETW_MODULE_STR ) );                                                            \
   }

#define HMFT_ETW_EVENT_STOP( EventId, this )                                                                                       \
   if( g_hEtwProvider )                                                                                                            \
   {                                                                                                                               \
      TraceLoggingWrite( g_hEtwProvider,                                                                                           \
                         EventId,                                                                                                  \
                         TraceLoggingOpcode( EVENT_TRACE_TYPE_STOP ),                                                              \
                         TraceLoggingPointer( this, ETW_MODULE_STR ) );                                                            \
   }

#define HMFT_ETW_EVENT_INFO( EventId, this )                                                                                       \
   if( g_hEtwProvider )                                                                                                            \
   {                                                                                                                               \
      TraceLoggingWrite( g_hEtwProvider,                                                                                           \
                         EventId,                                                                                                  \
                         TraceLoggingOpcode( EVENT_TRACE_TYPE_INFO ),                                                              \
                         TraceLoggingPointer( this, ETW_MODULE_STR ) );                                                            \
   }
