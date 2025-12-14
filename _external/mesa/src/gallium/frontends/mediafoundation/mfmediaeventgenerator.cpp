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

// IMFMediaEventGenerator::BeginGetEvent
// https://learn.microsoft.com/en-us/windows/win32/api/mfobjects/nf-mfobjects-imfmediaeventgenerator-begingetevent
HRESULT
CDX12EncHMFT::BeginGetEvent( IMFAsyncCallback *pCallback, IUnknown *punkState )
{
   HRESULT hr = S_OK;
   std::lock_guard<std::mutex> lock( m_lock );

   CHECKHR_GOTO( CheckShutdown(), done );
   CHECKHR_GOTO( m_spEventQueue->BeginGetEvent( pCallback, punkState ), done );

done:
   return hr;
}

// IMFMediaEventGenerator::EndGetEvent
// https://learn.microsoft.com/en-us/windows/win32/api/mfobjects/nf-mfobjects-imfmediaeventgenerator-endgetevent
HRESULT
CDX12EncHMFT::EndGetEvent( IMFAsyncResult *pResult, IMFMediaEvent **ppEvent )
{
   HRESULT hr = S_OK;
   MediaEventType met = MEUnknown;
   std::lock_guard<std::mutex> lock( m_lock );

   CHECKHR_GOTO( CheckShutdown(), done );
   CHECKHR_GOTO( m_spEventQueue->EndGetEvent( pResult, ppEvent ), done );
   ( *ppEvent )->GetType( &met );
   debug_printf( "[dx12 hmft 0x%p] EndGetEvent - SUCCESS, type = 0x%x\n", this, met );
   return hr;

done:
   debug_printf( "[dx12 hmft 0x%p] EndGetEvent - FAILED 0x%x\n", this, hr );
   return hr;
}

// IMFMediaEventGenerator::GetEvent
// https://learn.microsoft.com/en-us/windows/win32/api/mfobjects/nf-mfobjects-imfmediaeventgenerator-getevent
HRESULT
CDX12EncHMFT::GetEvent( DWORD dwFlags, IMFMediaEvent **ppEvent )
{
   HRESULT hr = S_OK;

   CHECKHR_GOTO( CheckShutdown(), done );
   CHECKHR_GOTO( m_spEventQueue->GetEvent( dwFlags, ppEvent ), done );

done:
   return hr;
}

// IMFMediaEventGenerator::QueueEvent
// https://learn.microsoft.com/en-us/windows/win32/api/mfobjects/nf-mfobjects-imfmediaeventgenerator-queueevent
HRESULT
CDX12EncHMFT::QueueEvent( MediaEventType met, REFGUID guidExtendedType, HRESULT hrStatus, const PROPVARIANT *pvValue )
{
   HRESULT hr = S_OK;
   ComPtr<IMFMediaEvent> spEvent;

   CHECKHR_GOTO( CheckShutdown(), done );
   CHECKHR_GOTO( MFCreateMediaEvent( met, guidExtendedType, hrStatus, pvValue, &spEvent ), done );
   switch( met )
   {
      case METransformDrainComplete:
         spEvent->SetUINT32( MF_EVENT_MFT_INPUT_STREAM_ID, 0 );
         break;
   }
   CHECKHR_GOTO( m_spEventQueue->QueueEvent( spEvent.Get() ), done );
done:
   return hr;
}
