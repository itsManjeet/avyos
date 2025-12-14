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
#if MFT_CODEC_AV1ENC
#include "hmft_entrypoints.h"
#include "mfbufferhelp.h"
#include "mfpipeinterop.h"

extern DWORD
CalculateQualityFromQP( DWORD QP );

HRESULT
CDX12EncHMFT::PrepareForEncodeHelper( LPDX12EncodeContext pDX12EncodeContext, bool dirtyRectFrameNumSet, uint32_t dirtyRectFrameNum )
{
   HRESULT hr = S_OK;
   // done:
   return hr;
}

HRESULT
CDX12EncHMFT::GetCodecPrivateData( LPBYTE pSPSPPSData, DWORD dwSPSPPSDataLen, LPDWORD lpdwSPSPPSDataLen )
{
   HRESULT hr = S_OK;
   // done:
   return hr;
}

static HRESULT
ConvertLevelToAVEncAV1VLevel( UINT32 uiLevel, eAVEncAV1VLevel &level )
{
   HRESULT hr = S_OK;
   level = eAVEncAV1VLevel5;
   switch( uiLevel )
   {
      case(UINT32) -1:   // -1 means auto
         level = eAVEncAV1VLevel5;
         break;
      case 0:
         level = eAVEncAV1VLevel2;
         break;
      case 1:
         level = eAVEncAV1VLevel2_1;
         break;
      case 4:
         level = eAVEncAV1VLevel3;
         break;
      case 5:
         level = eAVEncAV1VLevel3_1;
         break;
      case 8:
         level = eAVEncAV1VLevel4;
         break;
      case 9:
         level = eAVEncAV1VLevel4_1;
         break;
      case 12:
         level = eAVEncAV1VLevel5;
         break;
      case 13:
         level = eAVEncAV1VLevel5_1;
         break;
      case 14:
         level = eAVEncAV1VLevel5_2;
         break;
      case 15:
         level = eAVEncAV1VLevel5_3;
         break;
      case 16:
         level = eAVEncAV1VLevel6;
         break;
      case 17:
         level = eAVEncAV1VLevel6_1;
         break;
      case 18:
         level = eAVEncAV1VLevel6_2;
         break;
      case 19:
         level = eAVEncAV1VLevel6_3;
         break;
      default:
         hr = MF_E_INVALIDMEDIATYPE;
         break;
   }
   return hr;
}


HRESULT
CDX12EncHMFT::CheckMediaTypeLevel(
   IMFMediaType *pmt, int width, int height, const encoder_capabilities &encoderCapabilities, eAVEncAV1VLevel *pLevel ) const
{
   HRESULT hr = S_OK;
   UINT32 uiLevel = (UINT32) -1;
   uiLevel = MFGetAttributeUINT32( pmt, MF_MT_VIDEO_LEVEL, uiLevel );
   enum eAVEncAV1VLevel AVEncLevel;
   CHECKHR_GOTO( ConvertLevelToAVEncAV1VLevel( uiLevel, AVEncLevel ), done );
   if( pLevel )
   {
      *pLevel = AVEncLevel;
   }
done:
   return hr;
}

UINT32
CDX12EncHMFT::GetMaxReferences( unsigned int width, unsigned int height )
{
   UINT32 uiMaxReferences = PIPE_AV1_REFS_PER_FRAME;
   return uiMaxReferences;
}

HRESULT
CDX12EncHMFT::CreateGOPTracker( uint32_t textureWidth, uint32_t textureHeight )
{
   HRESULT hr = E_NOTIMPL;
   return hr;
}

#endif
