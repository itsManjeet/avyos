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

#include "../hmft_entrypoints.h"

const char *
TryFindD3D12CoreNextToSelf( char *path, DWORD path_arr_size );

HRESULT
CreateD3D12DeviceFactory( ID3D12DeviceFactory **ppFactory, UINT32 SDKVersion );

HRESULT
CreateD3D12Manager( IMFDXGIDeviceManager **ppMgr, UINT32 SDKVersion );

IMFMediaType *
CreateVideoMT( ULONG32 Width, ULONG32 Height, DWORD fourCC, BOOL interlaced, ULONG32 frNum, ULONG32 frDenon, UINT32 bitRate = 0);

class MFStartupHelper
{
 public:
   MFStartupHelper(HRESULT& hr)
   {
      hr = S_OK;
      if( SUCCEEDED( hr = CoInitializeEx( nullptr, COINIT_APARTMENTTHREADED ) ) )
      {
         hr = MFStartup( MF_VERSION );
      }
   }
   ~MFStartupHelper()
   {
      MFShutdown();
      // Free any unreferenced modules to catch potential leaks.
      CoFreeUnusedLibrariesEx( 0, 0 );
      CoUninitialize();

      // CheckMFPlatStartupLeaks();
   }
};

class CFrameGenerator
{
 public:
   CFrameGenerator();
   ~CFrameGenerator();

   HRESULT GenerateSoftwareFrame( IMFSample **pFrame, bool *endOfStream );

 public:
   const uint32_t m_uiDiv = 1;
   const uint32_t m_uiNum = 30;
   const uint32_t m_uiWidth = 320;
   const uint32_t m_uiHeight = 240;

   const uint32_t m_maxFrameCount = 88;
   uint32_t m_uiBitRate = 150;
   uint32_t m_uiFrameCount = 0;
   uint32_t m_uiOffsetX = 0;
   uint32_t m_uiOffsetY = 0;
};
