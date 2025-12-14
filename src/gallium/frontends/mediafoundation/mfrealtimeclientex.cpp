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
#include "wpptrace.h"

#include "mfrealtimeclientex.tmh"

// IMFRealTimeClientEx::RegisterThreadsEx
// https://learn.microsoft.com/en-us/windows/win32/api/mfidl/nf-mfidl-imfrealtimeclientex-registerthreadsex
HRESULT
CDX12EncHMFT::RegisterThreadsEx( DWORD *pdwTaskIndex, LPCWSTR wszClassName, LONG lBasePriority )
{
   std::lock_guard<std::mutex> lock( m_lock );

   if( !pdwTaskIndex )
   {
      return E_POINTER;
   }
   if( *pdwTaskIndex == 0 )
   {
      return E_INVALIDARG;
   }

   return E_NOTIMPL;
}

// IMFRealTimeClientEx::UnregisterThreads
// https://learn.microsoft.com/en-us/windows/win32/api/mfidl/nf-mfidl-imfrealtimeclientex-unregisterthreads
HRESULT
CDX12EncHMFT::UnregisterThreads()
{
   return E_NOTIMPL;
}

// IMFRealTimeClientEx::SetWorkQueueEx
// https://learn.microsoft.com/en-us/windows/win32/api/mfidl/nf-mfidl-imfrealtimeclientex-setworkqueueex
HRESULT
CDX12EncHMFT::SetWorkQueueEx( DWORD dwMultithreadedWorkQueueId, LONG lWorkItemBasePriority )
{
   return E_NOTIMPL;
}
