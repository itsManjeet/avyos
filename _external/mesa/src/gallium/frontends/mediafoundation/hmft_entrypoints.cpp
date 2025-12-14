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

/*
   - This file should implement all the necessary public HMFT interface functions and export them using
      - PUBLIC keyword in the method definition on this file
      - Adding the DLL export entry in:
         - mesa\src\gallium\targets\mediafoundation\mediafoundation.def.in
         - mesa\src\gallium\targets\mediafoundation\mediafoundation.sym

   - The actual implementation of the methods defined here should be delegated
      to implementations in other C files in the same directory, but keeping
      all the entrypoints/stubs in this file would help organize the DLL public interface
*/

CDX12EncHMFT::CDX12EncHMFT() : CMFD3DManager( this )
{ }

CDX12EncHMFT::~CDX12EncHMFT()
{
   Shutdown();
   CMFD3DManager::Shutdown();
}

HRESULT
CDX12EncHMFT::Initialize()
{
   HRESULT hr = S_OK;

   CHECKHR_GOTO( CMFD3DManager::Initialize( m_Codec ), done );
done:
   return hr;
}

#if MFT_CODEC_H264ENC
MFT_REGISTER_TYPE_INFO rgOutputInfo = { MFMediaType_Video, MFVideoFormat_H264 };
#elif MFT_CODEC_H265ENC
MFT_REGISTER_TYPE_INFO rgOutputInfo = { MFMediaType_Video, MFVideoFormat_HEVC };
#elif MFT_CODEC_AV1ENC
MFT_REGISTER_TYPE_INFO rgOutputInfo = { MFMediaType_Video, MFVideoFormat_AV1 };
#endif
MFT_REGISTER_TYPE_INFO rgInputInfo[NUM_INPUT_TYPES] = { { MFMediaType_Video, MFVideoFormat_NV12 },
                                                        { MFMediaType_Video, MFVideoFormat_P010 },
                                                        { MFMediaType_Video, MFVideoFormat_AYUV } };

// Internal function to initialize available input/output types and their associated MF attributes
HRESULT
CDX12EncHMFT::RuntimeClassInitialize()
{
   HRESULT hr = S_OK;
   ComPtr<IMFMediaType> spVideoType = NULL;

   static_assert( MFT_CODEC_H264ENC ^ MFT_CODEC_H265ENC ^ MFT_CODEC_AV1ENC,
                  "MFT_CODEC_H264ENC or MFT_CODEC_H265ENC or MFT_CODEC_AV1ENC must be defined but only one at a time" );

   // Start by configuring for 4:2:0 NV12 as the only possible input type.
   // Once the SetOutputType() happens with a profile, we'll reconfigure the available input type
   // accordingly. For example, specifying an output profile that indicates 4:4:4 would mean we
   // expose an input-type of AYUV.
   CHECKHR_GOTO( MFCreateMediaType( &spVideoType ), done );
   CHECKHR_GOTO( spVideoType->SetGUID( MF_MT_MAJOR_TYPE, MFMediaType_Video ), done );
   CHECKHR_GOTO( spVideoType->SetGUID( MF_MT_SUBTYPE, MFVideoFormat_NV12 ), done );
   m_spAvailableInputType.Attach( spVideoType.Detach() );

   CHECKHR_GOTO( MFCreateMediaType( &spVideoType ), done );
   CHECKHR_GOTO( spVideoType->SetGUID( MF_MT_MAJOR_TYPE, rgOutputInfo.guidMajorType ), done );
   CHECKHR_GOTO( spVideoType->SetGUID( MF_MT_SUBTYPE, rgOutputInfo.guidSubtype ), done );
   CHECKHR_GOTO( spVideoType->SetUINT32( MF_MT_IN_BAND_PARAMETER_SET, TRUE ), done );
   CHECKHR_GOTO( spVideoType->SetUINT32( MF_NALU_LENGTH_SET, 1 ), done );
   m_spAvailableOutputType.Attach( spVideoType.Detach() );

   CHECKHR_GOTO( MFCreateAttributes( &m_spMFAttributes, 7 ), done );
   CHECKHR_GOTO( m_spMFAttributes->SetUINT32( MFT_ENCODER_SUPPORTS_CONFIG_EVENT, TRUE ), done );
   // These are required to indicate we are an Async MFT (like all HMFTs are)
   CHECKHR_GOTO( m_spMFAttributes->SetUINT32( MF_TRANSFORM_ASYNC, TRUE ), done );
   CHECKHR_GOTO( m_spMFAttributes->SetUINT32( MFT_SUPPORT_DYNAMIC_FORMAT_CHANGE, TRUE ), done );
   // This is required to indicate we can handle an IMFDXGIDeviceManager (which is either 11 or 12)
   // NOTE: Ignore the poor naming of MF_SA_***D3D11***_AWARE here
   CHECKHR_GOTO( m_spMFAttributes->SetUINT32( MF_SA_D3D11_AWARE, TRUE ), done );
   CHECKHR_GOTO( m_spMFAttributes->SetUINT32( MF_SA_D3D12_AWARE, TRUE ), done );
   CHECKHR_GOTO( m_spMFAttributes->SetString( MFT_ENUM_HARDWARE_VENDOR_ID_Attribute, L"VEN_1414" ), done );
   CHECKHR_GOTO( m_spMFAttributes->SetString( MFT_ENUM_HARDWARE_URL_Attribute, g_pMFTFriendlyName ), done );
   CHECKHR_GOTO( m_spMFAttributes->SetString( MFT_FRIENDLY_NAME_Attribute, g_pMFTFriendlyName ), done );

   // Set up IMFMediaEventQueue
   CHECKHR_GOTO( MFCreateEventQueue( &m_spEventQueue ), done );

   CHECKHR_GOTO( Initialize(), done );
done:
   return hr;
}

// factory function
HRESULT
CDX12EncHMFT::CreateInstance( __deref_out CDX12EncHMFT **ppDX12EncHMFT )
{
   HRESULT hr = S_OK;
   ComPtr<CDX12EncHMFT> spDX12EncHMFT = Microsoft::WRL::Make<CDX12EncHMFT>();
   CHECKNULL_GOTO( ppDX12EncHMFT, E_INVALIDARG, done );
   CHECKNULL_GOTO( spDX12EncHMFT, E_OUTOFMEMORY, done );
   CHECKHR_GOTO( spDX12EncHMFT->RuntimeClassInitialize(), done );
   *ppDX12EncHMFT = spDX12EncHMFT.Detach();

done:
   return hr;
}
