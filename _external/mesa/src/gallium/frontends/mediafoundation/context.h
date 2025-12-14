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

#include <Unknwn.h>
#include "vl/vl_winsys.h"
#include "macros.h"
#include "mfpipeinterop.h"
#include "reference_frames_tracker.h"

typedef class DX12EncodeContext
{
 public:
   ComPtr<IMFSample> spSample;
   void *pAsyncCookie = nullptr;
   reference_frames_tracker_dpb_async_token *pAsyncDPBToken = nullptr;
   struct pipe_fence_handle *pAsyncFence = NULL;
   std::vector<struct pipe_resource *> pOutputBitRes;
   std::vector<struct pipe_fence_handle *> pSliceFences;
#if ( USE_D3D12_PREVIEW_HEADERS && ( D3D12_PREVIEW_SDK_VERSION >= 717 ) )
   D3D12_VIDEO_ENCODER_COMPRESSED_BITSTREAM_NOTIFICATION_MODE sliceNotificationMode =
      D3D12_VIDEO_ENCODER_COMPRESSED_BITSTREAM_NOTIFICATION_MODE_FULL_FRAME;
#endif   // (USE_D3D12_PREVIEW_HEADERS && (D3D12_PREVIEW_SDK_VERSION >= 717))
   pipe_resource *pPipeResourceQPMapStats = nullptr;
   pipe_resource *pPipeResourceSATDMapStats = nullptr;
   pipe_resource *pPipeResourceRCBitAllocMapStats = nullptr;
   pipe_resource *pPipeResourcePSNRStats = nullptr;

   // Keep all the media and sync objects until encode is done
   // and then signal EnqueueResourceRelease so the media
   // producer (e.g decoder) can reuse the buffer in their pool
   pipe_video_buffer *pPipeVideoBuffer = nullptr;
   pipe_video_buffer *pDownscaledTwoPassPipeVideoBuffer = nullptr;
   ComPtr<IMFMediaBuffer> spMediaBuffer;
   ComPtr<IMFD3D12SynchronizationObjectCommands> spSyncObjectCommands;
   ID3D12CommandQueue *pSyncObjectQueue = nullptr;   // weakref

   UINT textureWidth = 0;    // width of input sample
   UINT textureHeight = 0;   // height of input sample

   BOOL bROI = FALSE;
   ROI_AREA video_roi_area = {};

   UINT longTermReferenceFrameInfo = 0x0000FFFF;   // corresponds to MFT attribute MFSampleExtension_LongTermReferenceFrameInfo

   struct vl_screen *pVlScreen = nullptr;   // weakref
   struct pipe_video_codec encoderSettings = {};
   union
   {
      struct pipe_picture_desc base;
      struct pipe_h264_enc_picture_desc h264enc;
      struct pipe_h265_enc_picture_desc h265enc;
      struct pipe_av1_enc_picture_desc av1enc;
   } encoderPicInfo = {};
   const D3D12_VIDEO_ENCODER_CODEC m_Codec = D3D12_VIDEO_ENCODER_CODEC_H264;
   UINT32 GetPictureType()
   {
      UINT32 result = 0;
      switch( m_Codec )
      {
         case D3D12_VIDEO_ENCODER_CODEC_H264:
            result = ConvertPictureTypeToAVEncH264PictureType( encoderPicInfo.h264enc.picture_type );
            break;
         case D3D12_VIDEO_ENCODER_CODEC_HEVC:
            result = ConvertPictureTypeToAVEncH264PictureType( encoderPicInfo.h265enc.picture_type );
            break;
         case D3D12_VIDEO_ENCODER_CODEC_AV1:
            // TODO: ConvertPictureTypeToAVEncAV1PictureType(pDX12EncodeContext->encoderPicInfo.av1enc.picture_type);
            result = (UINT32) eAVEncAV1PictureType_Key;
            break;
      }
      return result;
   }

   BOOL IsPicTypeCleanPoint()
   {
      BOOL result = FALSE;
      switch( m_Codec )
      {
         case D3D12_VIDEO_ENCODER_CODEC_H264:
            if( ( encoderPicInfo.h264enc.picture_type == PIPE_H2645_ENC_PICTURE_TYPE_I ) ||
                ( encoderPicInfo.h264enc.picture_type == PIPE_H2645_ENC_PICTURE_TYPE_IDR ) )
               result = TRUE;
            break;
         case D3D12_VIDEO_ENCODER_CODEC_HEVC:
            if( ( encoderPicInfo.h265enc.picture_type == PIPE_H2645_ENC_PICTURE_TYPE_I ) ||
                ( encoderPicInfo.h265enc.picture_type == PIPE_H2645_ENC_PICTURE_TYPE_IDR ) )
               result = TRUE;
            break;
         case D3D12_VIDEO_ENCODER_CODEC_AV1:
            if( encoderPicInfo.av1enc.frame_type == PIPE_AV1_ENC_FRAME_TYPE_KEY )
               result = TRUE;
            break;
      }
      return result;
   }

   UINT32 GetFrameRateNumerator()
   {
      UINT32 result = 0;
      switch( m_Codec )
      {
         case D3D12_VIDEO_ENCODER_CODEC_H264:
            result = encoderPicInfo.h264enc.rate_ctrl[0].frame_rate_num;
            break;
         case D3D12_VIDEO_ENCODER_CODEC_HEVC:
            result = encoderPicInfo.h265enc.rc[0].frame_rate_num;
            break;
         case D3D12_VIDEO_ENCODER_CODEC_AV1:
            result = encoderPicInfo.av1enc.rc[0].frame_rate_num;
            break;
      }
      return result;
   }

   UINT32 GetFrameRateDenominator()
   {
      UINT32 result = 0;
      switch( m_Codec )
      {
         case D3D12_VIDEO_ENCODER_CODEC_H264:
            result = encoderPicInfo.h264enc.rate_ctrl[0].frame_rate_den;
            break;
         case D3D12_VIDEO_ENCODER_CODEC_HEVC:
            result = encoderPicInfo.h265enc.rc[0].frame_rate_den;
            break;
         case D3D12_VIDEO_ENCODER_CODEC_AV1:
            result = encoderPicInfo.av1enc.rc[0].frame_rate_den;
            break;
      }
      return result;
   }

   DX12EncodeContext( D3D12_VIDEO_ENCODER_CODEC codec ) : m_Codec( codec ) { };

   ~DX12EncodeContext()
   {
      if( spSyncObjectCommands )
         spSyncObjectCommands->EnqueueResourceRelease( pSyncObjectQueue );

      switch( m_Codec )
      {
         case D3D12_VIDEO_ENCODER_CODEC_H264:
            util_dynarray_foreach ( &encoderPicInfo.h264enc.raw_headers, struct pipe_enc_raw_header, header )
               delete header->buffer;
            util_dynarray_fini( &encoderPicInfo.h264enc.raw_headers );
            break;
         case D3D12_VIDEO_ENCODER_CODEC_HEVC:
            util_dynarray_foreach ( &encoderPicInfo.h265enc.raw_headers, struct pipe_enc_raw_header, header )
               delete header->buffer;
            util_dynarray_fini( &encoderPicInfo.h265enc.raw_headers );
            break;
         case D3D12_VIDEO_ENCODER_CODEC_AV1:
            util_dynarray_foreach ( &encoderPicInfo.av1enc.raw_headers, struct pipe_enc_raw_header, header )
               delete header->buffer;
            util_dynarray_fini( &encoderPicInfo.av1enc.raw_headers );
            break;
      }

      for( uint32_t slice_idx = 0; slice_idx < static_cast<uint32_t>( pOutputBitRes.size() ); slice_idx++ )
         if( ( ( slice_idx == 0 ) || pOutputBitRes[slice_idx] != pOutputBitRes[slice_idx - 1] ) && pOutputBitRes[slice_idx] )
            pVlScreen->pscreen->resource_destroy( pVlScreen->pscreen, pOutputBitRes[slice_idx] );

      if( pPipeResourceQPMapStats )
         pVlScreen->pscreen->resource_destroy( pVlScreen->pscreen, pPipeResourceQPMapStats );
      if( pPipeResourceSATDMapStats )
         pVlScreen->pscreen->resource_destroy( pVlScreen->pscreen, pPipeResourceSATDMapStats );
      if( pPipeResourceRCBitAllocMapStats )
         pVlScreen->pscreen->resource_destroy( pVlScreen->pscreen, pPipeResourceRCBitAllocMapStats );
      if( pPipeVideoBuffer )
         pPipeVideoBuffer->destroy( pPipeVideoBuffer );
      if( pDownscaledTwoPassPipeVideoBuffer )
         pDownscaledTwoPassPipeVideoBuffer->destroy( pDownscaledTwoPassPipeVideoBuffer );
      if( pPipeResourcePSNRStats )
         pVlScreen->pscreen->resource_destroy( pVlScreen->pscreen, pPipeResourcePSNRStats );
   }
} *LPDX12EncodeContext;
