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

#include <strmif.h>
#include "../hmft_entrypoints.h"
#include <gtest/gtest.h>
#include "hmft_test_helpers.h"

extern "C" IMAGE_DOS_HEADER __ImageBase;

const char *
TryFindD3D12CoreNextToSelf( char *path, DWORD path_arr_size )
{
   UINT32 path_size = GetModuleFileNameA( (HINSTANCE) &__ImageBase, path, path_arr_size );
   if( !path_arr_size || path_size == path_arr_size )
   {
      printf( "Unable to get path to self\n" );
      return nullptr;
   }

   auto last_slash = strrchr( path, '\\' );
   if( !last_slash )
   {
      printf( "Unable to get path to self\n" );
      return nullptr;
   }

   *( last_slash + 1 ) = '\0';
   if( strcat_s( path, path_arr_size, "D3D12Core.dll" ) != 0 )
   {
      printf( "Unable to get path to D3D12Core.dll next to self\n" );
      return nullptr;
   }

   if( GetFileAttributesA( path ) == INVALID_FILE_ATTRIBUTES )
   {
      printf( "No D3D12Core.dll exists next to self\n" );
      return nullptr;
   }

   *( last_slash + 1 ) = '\0';
   return path;
}

HRESULT
CreateD3D12DeviceFactory( ID3D12DeviceFactory **ppFactory, UINT32 SDKVersion )
{
   /* A device factory allows us to isolate things like debug layer enablement from other callers,
    * and can potentially even refer to a different D3D12 redist implementation from others.
    */

   HMODULE D3D12Module = LoadLibraryW( L"d3d12.dll" );
   if( !D3D12Module )
   {
      printf( "D3D12: Failed to LoadLibrary d3d12.dll" );
      return MF_E_DXGI_DEVICE_NOT_INITIALIZED;
   }

   typedef HRESULT( WINAPI * PFN_D3D12_GET_INTERFACE )( REFCLSID clsid, REFIID riid, void **ppFactory );
   PFN_D3D12_GET_INTERFACE D3D12GetInterface = (PFN_D3D12_GET_INTERFACE) GetProcAddress( D3D12Module, "D3D12GetInterface" );
   if( !D3D12GetInterface )
   {
      printf( "D3D12: Failed to retrieve D3D12GetInterface" );
      return MF_E_DXGI_DEVICE_NOT_INITIALIZED;
   }

   /* First, try to create a device factory from a DLL-parallel D3D12Core.dll */
   ID3D12SDKConfiguration *sdk_config = nullptr;
   if( SUCCEEDED( D3D12GetInterface( CLSID_D3D12SDKConfiguration, IID_PPV_ARGS( &sdk_config ) ) ) )
   {
      ID3D12SDKConfiguration1 *sdk_config1 = nullptr;
      if( SUCCEEDED( sdk_config->QueryInterface( &sdk_config1 ) ) )
      {
         char self_path[MAX_PATH];
         const char *d3d12core_path = TryFindD3D12CoreNextToSelf( self_path, sizeof( self_path ) );
         if( d3d12core_path &&
             SUCCEEDED( sdk_config1->CreateDeviceFactory( SDKVersion, d3d12core_path, IID_PPV_ARGS( ppFactory ) ) ) )
         {
            sdk_config->Release();
            sdk_config1->Release();
            ( *ppFactory )
               ->SetFlags( D3D12_DEVICE_FACTORY_FLAG_ALLOW_RETURNING_EXISTING_DEVICE |
                           D3D12_DEVICE_FACTORY_FLAG_ALLOW_RETURNING_INCOMPATIBLE_EXISTING_DEVICE );
            return S_OK;
         }

         printf( "D3D12: Can't find matching D3D12Core.dll next to self." );
         sdk_config->Release();
         sdk_config1->Release();
         return MF_E_DXGI_DEVICE_NOT_INITIALIZED;
      }
      else
      {
         printf( "D3D12: Failed to retrieve ID3D12SDKConfiguration1" );
         return MF_E_DXGI_DEVICE_NOT_INITIALIZED;
      }
   }
   else
   {
      printf( "D3D12: Failed to retrieve CLSID_D3D12SDKConfiguration" );
      return MF_E_DXGI_DEVICE_NOT_INITIALIZED;
   }

   return MF_E_DXGI_DEVICE_NOT_INITIALIZED;
}

HRESULT
CreateD3D12Manager( IMFDXGIDeviceManager **ppMgr, UINT32 SDKVersion )
{
   HRESULT hr = S_OK;
   UINT resetToken;
   ComPtr<ID3D12Device> spD3D12Device;
   DXGI_ADAPTER_DESC adapterDesc = { 0 };
   ComPtr<IDXGIFactory4> spDXGIFactory;
   ComPtr<IDXGIAdapter> spDXGIAdapter;
   ComPtr<ID3D12DeviceFactory> spFactory;

   CHECKHR_GOTO( CreateDXGIFactory1( IID_PPV_ARGS( &spDXGIFactory ) ), done );
   if( 0 /* m_inputParams.m_eGPU == GPU_WARP */ )
   {
      CHECKHR_GOTO( spDXGIFactory->EnumWarpAdapter( IID_PPV_ARGS( &spDXGIAdapter ) ), done );
      CHECKHR_GOTO( spDXGIAdapter->GetDesc( &adapterDesc ), done );
   }
   else
   {
      for( UINT i = 0; spDXGIFactory->EnumAdapters( i, &spDXGIAdapter ) != DXGI_ERROR_NOT_FOUND; ++i )
      {
         CHECKHR_GOTO( spDXGIAdapter->GetDesc( &adapterDesc ), done );
      }
   }
   CHECKHR_GOTO( MFCreateDXGIDeviceManager( &resetToken, ppMgr ), done );

   if( SDKVersion != 0 )
   {
      CHECKHR_GOTO( CreateD3D12DeviceFactory( &spFactory, SDKVersion ), done );

      /*
      if( m_inputParams.m_bD3DDebugLayer )
      {
         ComPtr<ID3D12Debug> spDebugController;
         CHECKHR_GOTO( spFactory->GetConfigurationInterface( CLSID_D3D12Debug, IID_PPV_ARGS( &spDebugController ) ), done );
         spDebugController->EnableDebugLayer();
      }
      */

      CHECKHR_GOTO( spFactory->CreateDevice( spDXGIAdapter.Get(), D3D_FEATURE_LEVEL_12_1, IID_PPV_ARGS( &spD3D12Device ) ), done );
   }
   else
   {
      CHECKHR_GOTO( D3D12CreateDevice( spDXGIAdapter.Get(), D3D_FEATURE_LEVEL_12_1, IID_PPV_ARGS( &spD3D12Device ) ), done );
   }

   CHECKHR_GOTO( ( *ppMgr )->ResetDevice( spD3D12Device.Get(), resetToken ), done );

done:
   return hr;
}

IMFMediaType *
CreateVideoMT( ULONG32 Width, ULONG32 Height, DWORD fourCC, BOOL interlaced, ULONG32 frNum, ULONG32 frDenom, UINT32 bitRate )
{
   HRESULT hr = S_OK;
   ComPtr<IMFMediaType> spVideoType {};
   GUID guid = { 0x00000000, 0x0000, 0x0010, 0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71 };
   guid.Data1 = fourCC;

   CHECKHR_GOTO( MFCreateMediaType( &spVideoType ), done );
   CHECKHR_GOTO( spVideoType->SetGUID( MF_MT_MAJOR_TYPE, MFMediaType_Video ), done );

   CHECKHR_GOTO( spVideoType->SetGUID( MF_MT_SUBTYPE, guid ), done );

   CHECKHR_GOTO( MFSetAttributeRatio( spVideoType.Get(), MF_MT_FRAME_RATE, frNum, frDenom ), done );
   CHECKHR_GOTO( MFSetAttributeSize( spVideoType.Get(), MF_MT_FRAME_SIZE, Width, Height ), done );
   CHECKHR_GOTO( spVideoType->SetUINT32( MF_MT_AVG_BITRATE, ( Width * Height ) / 8 ), done );
   CHECKHR_GOTO( spVideoType->SetUINT32( MF_MT_INTERLACE_MODE, MFVideoInterlace_Progressive ), done );

   if( bitRate > 0 )
   {
      CHECKHR_GOTO( spVideoType->SetUINT32( MF_MT_AVG_BITRATE, bitRate ), done );
   }
done:
   if( FAILED( hr ) )
   {
      return nullptr;
   }
   return spVideoType.Detach();
}

CFrameGenerator::CFrameGenerator()
{ }

CFrameGenerator::~CFrameGenerator()
{ }

HRESULT
CFrameGenerator::GenerateSoftwareFrame( IMFSample **pFrame, bool *endOfStream )
{
   HRESULT hr = S_OK;
   const uint32_t inLength = m_uiWidth * m_uiHeight * 3 / 2;
   ComPtr<IMFSample> spInSample;
   ComPtr<IMFMediaBuffer> spInBuffer;
   uint8_t *pBuf;
   uint32_t offsetX = m_uiOffsetX;
   uint32_t offsetY = m_uiOffsetY;

   const uint8_t yellowY = 210;
   const uint8_t yellowU = 16;
   const uint8_t yellowV = 146;

   const uint8_t tealY = 93;
   const uint8_t tealU = 146;
   const uint8_t tealV = 71;

   const uint32_t boxWidth = 40;
   const uint32_t boxHeight = 80;

   *endOfStream = false;
   if( m_uiFrameCount >= m_maxFrameCount )
   {
      *endOfStream = true;
      return hr;
   }

   CHECKHR_GOTO( MFCreateSample( &spInSample ), done );
   CHECKHR_GOTO( MFCreateMemoryBuffer( inLength, &spInBuffer ), done );
   spInSample->AddBuffer( spInBuffer.Get() );

   spInBuffer->Lock( &pBuf, NULL, NULL );
   {
      BYTE *pNV12 = pBuf;
      for( uint32_t i = 0; i < m_uiHeight; i++ )
      {
         memset( pNV12 + i * m_uiWidth, yellowY, m_uiWidth );
      }

      pNV12 += m_uiWidth * m_uiHeight;
      for( uint32_t i = 0; i < m_uiHeight / 2; i++ )
      {
         for( uint32_t j = 0; j < m_uiWidth; j += 2 )
         {
            pNV12[i * m_uiWidth + j] = yellowU;
            pNV12[i * m_uiWidth + j + 1] = yellowV;
         }
      }

      pNV12 = pBuf + offsetY * m_uiWidth + offsetX;
      for( uint32_t i = 0; i < boxHeight; i++ )
      {
         memset( pNV12 + i * m_uiWidth, tealY, boxWidth );
      }

      pNV12 = pBuf + m_uiWidth * m_uiHeight + ( offsetY ) * ( m_uiWidth / 2 ) + ( offsetX );
      for( uint32_t i = 0; i < boxHeight / 2; i++ )
      {
         for( uint32_t j = 0; j < boxWidth; j += 2 )
         {
            pNV12[i * m_uiWidth + j] = tealU;
            pNV12[i * m_uiWidth + j + 1] = tealV;
         }
      }

      m_uiOffsetX += 8;
      if( m_uiOffsetX >= m_uiWidth - boxWidth )
      {
         m_uiOffsetX = 0;
      }
   }
   spInBuffer->Unlock();

   spInSample->SetSampleTime( ( m_uiFrameCount * 10000000i64 ) / m_uiNum );
   spInSample->SetSampleDuration( 10000000i64 / m_uiNum );
   m_uiFrameCount++;

   *pFrame = spInSample.Detach();
done:
   return hr;
}
