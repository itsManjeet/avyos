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

#include "hmft_entrypoints.h"
#include "mfbufferhelp.h"
#include "mfpipeinterop.h"
#include "wpptrace.h"

#include "encode.tmh"

// internal function to prepare the data needed to pass to DX12 encoder via DX12EncoderContext
// this include converting the input sample to a DX12 accepted sample.  Note that codec specific
// handling is forward to PrepareForEncodeHelper
HRESULT
CDX12EncHMFT::PrepareForEncode( IMFSample *pSample, LPDX12EncodeContext *ppDX12EncodeContext )
{
   HRESULT hr = S_OK;
   UINT unDiscontinuity = 0;
   LPDX12EncodeContext pDX12EncodeContext;
   UINT uiSubresourceIndex = 0;
   UINT textureWidth = 0u;
   UINT textureHeight = 0u;
   bool bReceivedDirtyRectBlob = false;
   uint32_t dirtyRectFrameNum = UINT32_MAX;

   ComPtr<IMFDXGIBuffer> spDXGIBuffer;
   HANDLE hTexture = NULL;
   winsys_handle winsysHandle = {};
   ROI_AREA video_roi_area = {};
   UINT32 uiROIBlobOutSize = 0;
   // Get HW Support Surface Alignment to check against input sample
   const uint32_t surfaceWidthAlignment = 1 << m_EncoderCapabilities.m_HWSupportSurfaceAlignment.bits.log2_width_alignment;
   const uint32_t surfaceHeightAlignment = 1 << m_EncoderCapabilities.m_HWSupportSurfaceAlignment.bits.log2_width_alignment;

   // Check for Discontinuity
   (void) pSample->GetUINT32( MFSampleExtension_Discontinuity, &unDiscontinuity );
   if( unDiscontinuity )
   {
      MFE_INFO( "[dx12 hmft 0x%p] Discontinuity signaled on input sample", this );
      m_bForceKeyFrame = TRUE;
   }

   CHECKNULL_GOTO( pDX12EncodeContext = new DX12EncodeContext( m_Codec ), E_OUTOFMEMORY, done );
   CHECKNULL_GOTO( pDX12EncodeContext->pAsyncDPBToken = new reference_frames_tracker_dpb_async_token(), E_OUTOFMEMORY, done );

   CHECKHR_GOTO( pSample->GetBufferByIndex( 0, &pDX12EncodeContext->spMediaBuffer ), done );

   // If we can't get a DXGIBuffer out of this incoming buffer, then its a software-based buffer
   if( FAILED( pDX12EncodeContext->spMediaBuffer.As( &spDXGIBuffer ) ) )
   {
      ComPtr<IMFSample> spSample;
      ComPtr<IMFMediaBuffer> spBuffer;
      // Allocate a video buffer
      CHECKHR_GOTO( m_spVideoSampleAllocator->AllocateSample( &spSample ), done );
      CHECKHR_GOTO( MFCopySample( spSample.Get(), pSample, m_spInputType.Get() ), done );
      CHECKHR_GOTO( spSample->GetBufferByIndex( 0, &pDX12EncodeContext->spMediaBuffer ), done );
      CHECKHR_GOTO( pDX12EncodeContext->spMediaBuffer.As( &spDXGIBuffer ), done );
      debug_printf( "[dx12 hmft 0x%p] Software input sample\n", this );
   }

   CHECKHR_GOTO( spDXGIBuffer->GetSubresourceIndex( &uiSubresourceIndex ), done );
   if( m_spDevice11 )
   {
      // D3D11 input sample path
      ComPtr<IDXGIResource1> spDXGIResource1;
      ComPtr<ID3D11Texture2D> spTexture;
      ComPtr<ID3D11DeviceContext3> spDeviceContext3;
      ComPtr<ID3D11DeviceContext4> spDeviceContext4;
      D3D11_TEXTURE2D_DESC d3d11textureDescSrc;
      D3D11_TEXTURE2D_DESC d3d11textureDescDst;
      D3D11_TEXTURE2D_DESC desc = {};
      CHECKHR_GOTO( spDXGIBuffer->GetResource( IID_PPV_ARGS( &spTexture ) ), done );
      spTexture->GetDesc( &desc );
      textureWidth = desc.Width;
      textureHeight = desc.Height;

      //
      // Attempt to create a shared handle from the DX11 texture that can
      // be opened as an ID3D12Resource to avoid a copy from DX11 -> DX12
      // and place the opened video buffer from the shared resource in
      // pDX12EncodeContext->pPipeVideoBuffer
      // video_buffer_from_handle expects data to be on first subresource (e.g no texture array)
      //
      if (uiSubresourceIndex == 0)
      {
         CHECKHR_GOTO( spTexture.As( &spDXGIResource1 ), done );
         if(SUCCEEDED(spDXGIResource1->CreateSharedHandle( nullptr, DXGI_SHARED_RESOURCE_READ, nullptr, &hTexture )))
         {
            // Open the pipe video buffer from the hTexture of the source DX11 texture directly
            // as an ID3D12Resource, wrapped in pDX12EncodeContext->pPipeVideoBuffer
            winsysHandle.handle = hTexture;
            winsysHandle.type = WINSYS_HANDLE_TYPE_FD;
            pDX12EncodeContext->pPipeVideoBuffer = m_pPipeContext->video_buffer_from_handle( m_pPipeContext, NULL, &winsysHandle, 0 );
         }
      }

      // On successful opening of the DX11 shared texture as an ID3D12Resource, wrapped in pDX12EncodeContext->pPipeVideoBuffer
      // signal readiness to the consumer of the texture's fence
      //
      // Otherwise, if the interop without a copy into pDX12EncodeContext->pPipeVideoBuffer failed, fallback to doing the copy
      // and placing the copy destination texture in pDX12EncodeContext->pPipeVideoBuffer
      //
      if (pDX12EncodeContext->pPipeVideoBuffer != nullptr)
      {
         m_spDevice11->GetImmediateContext3( &spDeviceContext3 );
         CHECKHR_GOTO( spDeviceContext3.As( &spDeviceContext4 ), done );

         // This will signal the staging fence the d3d12 mesa backend is consuming
         spDeviceContext4->Signal( m_spStagingFence11.Get(), m_NextSyncFenceValue );
         debug_printf( "[dx12 hmft 0x%p] DX11 *shared* input sample\n", this );
      }
      else
      {
         spTexture->GetDesc( &d3d11textureDescSrc );
         // We need to create a shareable texture and copy into it
         ComPtr<ID3D11Texture2D> spSharedTexture;
         D3D11_BOX d3d11Box = { 0 };
         d3d11textureDescDst = d3d11textureDescSrc;
         d3d11textureDescDst.MiscFlags = D3D11_RESOURCE_MISC_SHARED_NTHANDLE | D3D11_RESOURCE_MISC_SHARED;
         d3d11textureDescDst.BindFlags = D3D11_BIND_SHADER_RESOURCE;
         d3d11textureDescDst.ArraySize = 1;
         d3d11textureDescDst.Width = textureWidth;
         d3d11textureDescDst.Height = textureHeight;
         CHECKHR_GOTO( m_spDevice11->CreateTexture2D( &d3d11textureDescDst, nullptr, &spSharedTexture ), done );

         // Open the pipe video buffer from the hTexture of the copy destination texture
         CHECKHR_GOTO( spSharedTexture.As( &spDXGIResource1 ), done );
         CHECKHR_GOTO( spDXGIResource1->CreateSharedHandle( nullptr, DXGI_SHARED_RESOURCE_READ, nullptr, &hTexture ), done );
         debug_printf( "[dx12 hmft 0x%p] DX11 input sample\n", this );
         winsysHandle.handle = hTexture;
         winsysHandle.type = WINSYS_HANDLE_TYPE_FD;
         CHECKNULL_GOTO(
            pDX12EncodeContext->pPipeVideoBuffer = m_pPipeContext->video_buffer_from_handle( m_pPipeContext, NULL, &winsysHandle, 0 ),
            MF_E_UNEXPECTED,
            done );

         // On successful opening of shared texture, submit the copy and signal readiness to the consumer of the texture
         m_spDevice11->GetImmediateContext3( &spDeviceContext3 );
         d3d11Box.right = d3d11textureDescSrc.Width;
         d3d11Box.bottom = d3d11textureDescSrc.Height;
         d3d11Box.back = 1;
         spDeviceContext3
            ->CopySubresourceRegion( spSharedTexture.Get(), 0, 0, 0, 0, spTexture.Get(), uiSubresourceIndex, &d3d11Box );
         // This will signal the staging fence the d3d12 mesa backend is consuming
         // Since we're signaling from the D3D11 context on a shared fence, the signal
         // will happen after the d3d11 context copy is done.
         CHECKHR_GOTO( spDeviceContext3.As( &spDeviceContext4 ), done );
         spDeviceContext4->Signal( m_spStagingFence11.Get(), m_NextSyncFenceValue );
      }
   }
   else
   {
      // D3D12 input sample path
      ComPtr<ID3D12Resource> spResource;

      CHECKHR_GOTO( spDXGIBuffer->GetResource( IID_PPV_ARGS( &spResource ) ), done );

      const D3D12_RESOURCE_DESC desc = spResource->GetDesc();
      textureWidth = static_cast<UINT>( desc.Width );
      textureHeight = static_cast<UINT>( desc.Height );

      CHECKHR_GOTO(
         spDXGIBuffer->GetUnknown( MF_D3D12_SYNCHRONIZATION_OBJECT, IID_PPV_ARGS( &pDX12EncodeContext->spSyncObjectCommands ) ),
         done );
      CHECKHR_GOTO( pDX12EncodeContext->spSyncObjectCommands->EnqueueResourceReadyWait( m_spStagingQueue.Get() ), done );
      pDX12EncodeContext->pSyncObjectQueue = m_spStagingQueue.Get();

      // This will signal the staging fence the d3d12 mesa backend is consuming
      // Since we have a Wait() on spStagingQueue added by EnqueueResourceReadyWait, this will only happen after MF
      // triggered completion on the input
      m_spStagingQueue->Signal( m_spStagingFence12.Get(), m_NextSyncFenceValue );

      winsysHandle.com_obj = spResource.Get();
      winsysHandle.type = WINSYS_HANDLE_TYPE_D3D12_RES;
      CHECKBOOL_GOTO( uiSubresourceIndex == 0,
                      MF_E_UNEXPECTED,
                      done );   // video_buffer_from_handle expects data to be on first subresource (e.g no texture array)
      CHECKNULL_GOTO(
         pDX12EncodeContext->pPipeVideoBuffer = m_pPipeContext->video_buffer_from_handle( m_pPipeContext, NULL, &winsysHandle, 0 ),
         MF_E_UNEXPECTED,
         done );
      debug_printf( "[dx12 hmft 0x%p] DX12 input sample\n", this );
   }

#if ENCODE_WITH_TWO_PASS
   if( m_pPipeVideoCodec->two_pass.enable && ( m_pPipeVideoCodec->two_pass.pow2_downscale_factor > 0 ) )
   {
      // TODO: In case the app sends the downscaled input remove this

      //
      // Use VPBlit to downscale the input texture to generate the 1st pass
      // downscaled input texture
      //

      struct pipe_video_buffer templ = {};
      templ.buffer_format = pDX12EncodeContext->pPipeVideoBuffer->buffer_format;
      templ.width = static_cast<uint32_t>(
         std::ceil( pDX12EncodeContext->pPipeVideoBuffer->width / ( 1 << m_pPipeVideoCodec->two_pass.pow2_downscale_factor ) ) );
      templ.height = static_cast<uint32_t>(
         std::ceil( pDX12EncodeContext->pPipeVideoBuffer->height / ( 1 << m_pPipeVideoCodec->two_pass.pow2_downscale_factor ) ) );
      pDX12EncodeContext->pDownscaledTwoPassPipeVideoBuffer = m_pPipeContext->create_video_buffer( m_pPipeContext, &templ );

      struct pipe_vpp_desc vpblit_params = {};
      struct pipe_fence_handle *dst_surface_fence = nullptr;

      vpblit_params.base.in_fence = m_pPipeFenceHandle;   // input surface fence (driver input)
      vpblit_params.base.in_fence_value = m_CurrentSyncFenceValue;
      vpblit_params.base.out_fence = &dst_surface_fence;          // Output surface fence (driver output)

      vpblit_params.base.input_format = pDX12EncodeContext->pPipeVideoBuffer->buffer_format;
      vpblit_params.base.output_format = pDX12EncodeContext->pDownscaledTwoPassPipeVideoBuffer->buffer_format;
      vpblit_params.src_region.x0 = 0u;
      vpblit_params.src_region.y0 = 0u;
      vpblit_params.src_region.x1 = pDX12EncodeContext->pPipeVideoBuffer->width;
      vpblit_params.src_region.y1 = pDX12EncodeContext->pPipeVideoBuffer->height;

      vpblit_params.dst_region.x0 = 0u;
      vpblit_params.dst_region.y0 = 0u;
      vpblit_params.dst_region.x1 = pDX12EncodeContext->pDownscaledTwoPassPipeVideoBuffer->width;
      vpblit_params.dst_region.y1 = pDX12EncodeContext->pDownscaledTwoPassPipeVideoBuffer->height;

      m_pPipeVideoBlitter->begin_frame( m_pPipeVideoBlitter,
                                        pDX12EncodeContext->pDownscaledTwoPassPipeVideoBuffer,
                                        &vpblit_params.base );

      CHECKBOOL_GOTO(
         ( m_pPipeVideoBlitter->process_frame( m_pPipeVideoBlitter, pDX12EncodeContext->pPipeVideoBuffer, &vpblit_params ) == 0 ),
         MF_E_UNEXPECTED,
         done );
      CHECKBOOL_GOTO( ( m_pPipeVideoBlitter->end_frame( m_pPipeVideoBlitter,
                                                        pDX12EncodeContext->pDownscaledTwoPassPipeVideoBuffer,
                                                        &vpblit_params.base ) == 0 ),
                      MF_E_UNEXPECTED,
                      done );
      m_pPipeVideoBlitter->flush( m_pPipeVideoBlitter );

      assert(dst_surface_fence);   // Driver must have returned the completion fence
      // Wait for downscaling completion before encode can proceed

      ASSERTED bool finished = m_pPipeVideoCodec->fence_wait( m_pPipeVideoCodec,
                                                              dst_surface_fence,
                                                              OS_TIMEOUT_INFINITE );
      assert( finished );
      m_pPipeVideoCodec->destroy_fence( m_pPipeVideoCodec, dst_surface_fence);
   }
#endif   // ENCODE_WITH_TWO_PASS

   // validate texture dimensions with surface alignment here for now, will add handling for non-aligned textures later
   if( textureWidth % surfaceWidthAlignment != 0 || textureHeight % surfaceHeightAlignment != 0 )
   {
      assert( false );
   }
   pDX12EncodeContext->textureWidth = textureWidth;
   pDX12EncodeContext->textureHeight = textureHeight;

   if( m_uiDirtyRectEnabled )
   {
      UINT32 cBlob = 0;
      pSample->GetBlobSize( MFSampleExtension_DirtyRects, &cBlob );
      if( cBlob >= sizeof( DIRTYRECT_INFO ) )
      {
         if( m_pDirtyRectBlob.size() < cBlob )
         {
            m_pDirtyRectBlob.resize( cBlob );
         }
         if( S_OK == pSample->GetBlob( MFSampleExtension_DirtyRects, m_pDirtyRectBlob.data(), cBlob, &cBlob ) )
         {
            DIRTYRECT_INFO *pDirtyRectInfo = (DIRTYRECT_INFO *) m_pDirtyRectBlob.data();
            dirtyRectFrameNum = pDirtyRectInfo->FrameNumber;
            bReceivedDirtyRectBlob = true;
         }
      }
   }

   if( m_pGOPTracker == nullptr )
   {
      CHECKHR_GOTO( CreateGOPTracker( textureWidth, textureHeight ), done );
   }

   {
      bool markLTR = false;
      bool useLTR = false;
      uint32_t markLTRindex = 0;
      uint32_t useLTRbitmap = 0;

      if( m_uiMaxLongTermReferences > 0 )
      {
         if( m_bMarkLTRFrameSet )
         {
            markLTR = true;
            markLTRindex = m_uiMarkLTRFrame;
            assert( m_uiMarkLTRFrame < m_uiMaxLongTermReferences );   // TODO: add check at CodecAPI level
            m_bMarkLTRFrameSet = FALSE;
         }

         if( m_bUseLTRFrameSet )
         {
            useLTR = true;
            useLTRbitmap = m_uiUseLTRFrame;
            m_bUseLTRFrameSet = FALSE;
         }
      }

      m_pGOPTracker->begin_frame( pDX12EncodeContext->pAsyncDPBToken,
                                  m_bForceKeyFrame,
                                  markLTR,
                                  markLTRindex,
                                  useLTR,
                                  useLTRbitmap,
                                  m_bLayerCountSet,
                                  m_uiLayerCount,
                                  bReceivedDirtyRectBlob,
                                  dirtyRectFrameNum );
      if( m_bForceKeyFrame )
      {
         m_bForceKeyFrame = FALSE;
      }
   }

   {
      //
      // Create resources for output GPU frame stats
      //
      struct pipe_resource templ = {};
      memset( &templ, 0, sizeof( templ ) );
      templ.target = PIPE_TEXTURE_2D;
      // PIPE_USAGE_STAGING allocates resource in L0 (System Memory) heap
      // and avoid a bunch of roundtrips for uploading/reading back the bitstream headers
      // The GPU writes once the slice data (if dGPU over the PCIe bus) and all the other
      // uploads (e.g bitstream headers from CPU) and readbacks to output MFSamples
      // happen without moving data between L0/L1 pools
      templ.usage = PIPE_USAGE_DEFAULT;
      templ.depth0 = 1;
      templ.array_size = 1;

      if( m_EncoderCapabilities.m_HWSupportStatsQPMapOutput.bits.supported && m_uiVideoOutputQPMapBlockSize > 0 )
      {
         uint32_t block_size = ( 1 << m_EncoderCapabilities.m_HWSupportStatsQPMapOutput.bits.log2_values_block_size );
         templ.format = (enum pipe_format) m_EncoderCapabilities.m_HWSupportStatsQPMapOutput.bits.pipe_pixel_format;
         templ.width0 = static_cast<uint32_t>( std::ceil( m_uiOutputWidth / static_cast<float>( block_size ) ) );
         templ.height0 = static_cast<uint16_t>( std::ceil( m_uiOutputHeight / static_cast<float>( block_size ) ) );
         CHECKNULL_GOTO(
            pDX12EncodeContext->pPipeResourceQPMapStats = m_pVlScreen->pscreen->resource_create( m_pVlScreen->pscreen, &templ ),
            E_OUTOFMEMORY,
            done );
      }

      if( m_EncoderCapabilities.m_HWSupportStatsSATDMapOutput.bits.supported )
      {
         uint32_t block_size = ( 1 << m_EncoderCapabilities.m_HWSupportStatsSATDMapOutput.bits.log2_values_block_size );
         templ.format = (enum pipe_format) m_EncoderCapabilities.m_HWSupportStatsSATDMapOutput.bits.pipe_pixel_format;
         templ.width0 = static_cast<uint32_t>( std::ceil( m_uiOutputWidth / static_cast<float>( block_size ) ) );
         templ.height0 = static_cast<uint16_t>( std::ceil( m_uiOutputHeight / static_cast<float>( block_size ) ) );
         CHECKNULL_GOTO(
            pDX12EncodeContext->pPipeResourceSATDMapStats = m_pVlScreen->pscreen->resource_create( m_pVlScreen->pscreen, &templ ),
            E_OUTOFMEMORY,
            done );
      }

      if( m_EncoderCapabilities.m_HWSupportStatsRCBitAllocationMapOutput.bits.supported && m_uiVideoOutputBitsUsedMapBlockSize > 0 )
      {
         uint32_t block_size = ( 1 << m_EncoderCapabilities.m_HWSupportStatsRCBitAllocationMapOutput.bits.log2_values_block_size );
         templ.format = (enum pipe_format) m_EncoderCapabilities.m_HWSupportStatsRCBitAllocationMapOutput.bits.pipe_pixel_format;
         templ.width0 = static_cast<uint32_t>( std::ceil( m_uiOutputWidth / static_cast<float>( block_size ) ) );
         templ.height0 = static_cast<uint16_t>( std::ceil( m_uiOutputHeight / static_cast<float>( block_size ) ) );
         CHECKNULL_GOTO( pDX12EncodeContext->pPipeResourceRCBitAllocMapStats =
                            m_pVlScreen->pscreen->resource_create( m_pVlScreen->pscreen, &templ ),
                         E_OUTOFMEMORY,
                         done );
      }

      if( m_EncoderCapabilities.m_PSNRStatsSupport.bits.supports_y_channel && m_bVideoEnableFramePsnrYuv )
      {
         struct pipe_resource buffer_templ = {};
         buffer_templ.width0 = 3 * sizeof( float );   // Up to 3 float components Y, U, V
         buffer_templ.target = PIPE_BUFFER;
         // PIPE_USAGE_STAGING allocates resource in L0 (System Memory) heap
         // and avoid a bunch of roundtrips for uploading/reading back the bitstream headers
         // The GPU writes once the slice data (if dGPU over the PCIe bus) and all the other
         // uploads (e.g bitstream headers from CPU) and readbacks to output MFSamples
         // happen without moving data between L0/L1 pools
         buffer_templ.usage = PIPE_USAGE_STAGING;
         buffer_templ.format = PIPE_FORMAT_R8_UINT;
         buffer_templ.height0 = 1;
         buffer_templ.depth0 = 1;
         buffer_templ.array_size = 1;
         CHECKNULL_GOTO( pDX12EncodeContext->pPipeResourcePSNRStats =
                            m_pVlScreen->pscreen->resource_create( m_pVlScreen->pscreen, &buffer_templ ),
                         E_OUTOFMEMORY,
                         done );
      }
   }

   memset( &pDX12EncodeContext->encoderPicInfo, 0, sizeof( pDX12EncodeContext->encoderPicInfo ) );
   pDX12EncodeContext->encoderPicInfo.base.profile = m_outputPipeProfile;

   // Encode region of interest
   // When m_bVideoROIEnabled, app can (or not) set MFSampleExtension_ROIRectangle on separate frames optionally
   if( m_bVideoROIEnabled )
   {
      pSample->GetBlob( MFSampleExtension_ROIRectangle,
                        (UINT8 *) &pDX12EncodeContext->video_roi_area,
                        sizeof( ROI_AREA ),
                        &uiROIBlobOutSize );
      if( uiROIBlobOutSize > 0 )
      {
         // Check the Blob size matches the struct size we expect
         CHECKBOOL_GOTO( uiROIBlobOutSize == sizeof( ROI_AREA ), MF_E_UNEXPECTED, done );

         // When requested QPDelta == 0, just don't enable roi since it won't have any effect
         if( video_roi_area.QPDelta != 0 )
         {
            // Check for hardware support for delta QP
            CHECKBOOL_GOTO( m_EncoderCapabilities.m_HWSupportsVideoEncodeROI.bits.roi_rc_qp_delta_support == 1,
                            MF_E_UNEXPECTED,
                            done );

            pDX12EncodeContext->bROI = TRUE;
         }
      }
   }

   pDX12EncodeContext->pVlScreen = m_pVlScreen;   // weakref

   // Call the helper for encoder specific work
   CHECKHR_GOTO( PrepareForEncodeHelper( pDX12EncodeContext, bReceivedDirtyRectBlob, dirtyRectFrameNum ), done );

   {
      struct pipe_resource templ = {};
      memset( &templ, 0, sizeof( templ ) );

      // Prefer using sliced buffers + fence notifications when supported to reduce latency if
      // user requested multiple slices
      // Otherwise fallback to full frame encoding fence notification using a single output buffer
      uint32_t num_output_buffers = 1u;

#if MFT_CODEC_H264ENC
      num_output_buffers = std::max( 1u, pDX12EncodeContext->encoderPicInfo.h264enc.num_slice_descriptors );
#elif MFT_CODEC_H265ENC
      num_output_buffers = std::max( 1u, pDX12EncodeContext->encoderPicInfo.h265enc.num_slice_descriptors );
#elif MFT_CODEC_AV1ENC
      num_output_buffers =
         std::max( 1u, pDX12EncodeContext->encoderPicInfo.av1enc.tile_rows * pDX12EncodeContext->encoderPicInfo.av1enc.tile_cols );
#endif

#if ( USE_D3D12_PREVIEW_HEADERS && ( D3D12_PREVIEW_SDK_VERSION >= 717 ) )
      pDX12EncodeContext->sliceNotificationMode = D3D12_VIDEO_ENCODER_COMPRESSED_BITSTREAM_NOTIFICATION_MODE_FULL_FRAME;
      if( m_EncoderCapabilities.m_HWSupportSlicedFences.bits.supported && ( num_output_buffers > 1 ) )
      {
         pDX12EncodeContext->sliceNotificationMode = D3D12_VIDEO_ENCODER_COMPRESSED_BITSTREAM_NOTIFICATION_MODE_SUBREGIONS;
         if( m_EncoderCapabilities.m_HWSupportSlicedFences.bits.multiple_buffers_required )
         {
            // Buffer byte size for sliced buffers + notifications with multiple individual buffers per slice
            templ.width0 = ( 1024 /*1K*/ * 1024 /*1MB*/ ) * 8 /*8 MB*/;
         }
         else
         {
            // Buffer byte size for sliced buffers + notifications with a single buffer (suballocated by driver for each slice)
            templ.width0 = ( 1024 /*1K*/ * 1024 /*1MB*/ ) * 8 /*8 MB*/;
         }
      }
      else
#endif   // (USE_D3D12_PREVIEW_HEADERS && (D3D12_PREVIEW_SDK_VERSION >= 717))
      {
         // Buffer byte size for full frame bitstream (when num_output_buffers == 1)
         templ.width0 = ( 1024 /*1K*/ * 1024 /*1MB*/ ) * 8 /*8 MB*/;
      }

      templ.target = PIPE_BUFFER;
      // PIPE_USAGE_STAGING allocates resource in L0 (System Memory) heap
      // and avoid a bunch of roundtrips for uploading/reading back the bitstream headers
      // The GPU writes once the slice data (if dGPU over the PCIe bus) and all the other
      // uploads (e.g bitstream headers from CPU) and readbacks to output MFSamples
      // happen without moving data between L0/L1 pools
      templ.usage = PIPE_USAGE_STAGING;
      templ.format = PIPE_FORMAT_R8_UINT;
      templ.height0 = 1;
      templ.depth0 = 1;
      templ.array_size = 1;

      pDX12EncodeContext->pOutputBitRes.resize( num_output_buffers, NULL );
      pDX12EncodeContext->pSliceFences.resize( num_output_buffers, NULL );
      for( uint32_t slice_idx = 0; slice_idx < num_output_buffers; slice_idx++ )
      {
         if( ( slice_idx > 0 ) && !m_EncoderCapabilities.m_HWSupportSlicedFences.bits.multiple_buffers_required )
         {
            // sliced buffers + notifications with a single buffer (suballocated by driver for each slice)
            pDX12EncodeContext->pOutputBitRes[slice_idx] = pDX12EncodeContext->pOutputBitRes[0];
         }
         else
         {
            // sliced buffers + notifications with multiple individual buffers per slice
            // or, full frame bitstream (when num_output_buffers == 1)
            CHECKNULL_GOTO(
               pDX12EncodeContext->pOutputBitRes[slice_idx] = m_pVlScreen->pscreen->resource_create( m_pVlScreen->pscreen, &templ ),
               E_OUTOFMEMORY,
               done );
         }
      }
   }

   // Set the fence to be waited on m_SyncFenceValue and increment the value for the next frame
   m_CurrentSyncFenceValue = m_NextSyncFenceValue++;

done:
   if( SUCCEEDED( hr ) )
   {
      *ppDX12EncodeContext = pDX12EncodeContext;
      pDX12EncodeContext = nullptr;
   }
   SAFE_DELETE( pDX12EncodeContext );
   SAFE_CLOSEHANDLE( hTexture );
   return hr;
}

// utility function to validate the user passed in dirty rects
HRESULT
CDX12EncHMFT::ValidateDirtyRects( const LPDX12EncodeContext pDX12EncodeContext, const DIRTYRECT_INFO *pDirtyRectInfo )
{
   HRESULT hr = S_OK;
   const UINT uiNumDirtyRects = pDirtyRectInfo->NumDirtyRects;
   const LONG textureWidth = static_cast<LONG>( pDX12EncodeContext->textureWidth );
   const LONG textureHeight = static_cast<LONG>( pDX12EncodeContext->textureHeight );
   for( UINT i = 0; i < uiNumDirtyRects; i++ )
   {
      if( pDirtyRectInfo->DirtyRects[i].left < 0 || pDirtyRectInfo->DirtyRects[i].top < 0 ||
          pDirtyRectInfo->DirtyRects[i].right < pDirtyRectInfo->DirtyRects[i].left ||
          pDirtyRectInfo->DirtyRects[i].bottom < pDirtyRectInfo->DirtyRects[i].top ||
          pDirtyRectInfo->DirtyRects[i].right > textureWidth || pDirtyRectInfo->DirtyRects[i].bottom > textureHeight )
      {
         debug_printf( "MFT: invalid dirty rect %d (%d, %d, %d, %d) received\n",
                       i,
                       pDirtyRectInfo->DirtyRects[i].left,
                       pDirtyRectInfo->DirtyRects[i].top,
                       pDirtyRectInfo->DirtyRects[i].right,
                       pDirtyRectInfo->DirtyRects[i].bottom );
         CHECKHR_GOTO( E_INVALIDARG, done );
      }
   }
done:
   return hr;
}
