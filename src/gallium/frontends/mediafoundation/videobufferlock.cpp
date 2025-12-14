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

#include "videobufferlock.h"
#include "mfbufferhelp.h"

HRESULT
VideoBufferLock::lock( __in MF2DBuffer_LockFlags flags )
{
   HRESULT hr = S_OK;
   DWORD maxSize = 0;
   BYTE *pbBufferStart = NULL;
   DWORD dwBufferLength = 0;
   UINT32 dwMinSize = 0;
   LONG lDefaultStride = 0;

   if( _bLocked2D || _bLocked2D2 || _bLockedBuffer )
   {
      return S_FALSE;
   }

   if( _pInputBuffer == nullptr )
   {
      return MF_E_NOT_INITIALIZED;
   }

   GUID format;
   UINT32 unWidth = 0, unHeight = 0;
   hr = _pmt->GetGUID( MF_MT_SUBTYPE, &format );
   if( SUCCEEDED( hr ) )
   {
      hr = MFGetAttributeSize( _pmt.Get(), MF_MT_FRAME_SIZE, &unWidth, &unHeight );
      if( SUCCEEDED( hr ) )
      {
         hr = MFGetStrideForBitmapInfoHeader( format.Data1, unWidth, &lDefaultStride );
      }
   }
   if( FAILED( hr ) )
   {
      lDefaultStride = 0;
   }

   hr = MFCalculateImageSize( format, unWidth, unHeight, &dwMinSize );
   if( FAILED( hr ) )
   {
      dwMinSize = 0;
      (void) _pInputBuffer->GetCurrentLength( (DWORD *) &dwMinSize );
   }

   hr = _pInputBuffer.As( &_pInputBuffer2D2 );
   if( SUCCEEDED( hr ) )
   {
      hr = _pInputBuffer2D2->Lock2DSize( flags, &_pData, &_lPitch, &pbBufferStart, &dwBufferLength );
      if( SUCCEEDED( hr ) )
      {
         if( dwBufferLength < dwMinSize || abs( _lPitch ) < abs( lDefaultStride ) )
         {
            _pInputBuffer2D2->Unlock2D();
            return MF_E_BUFFERTOOSMALL;
         }
         _bLocked2D2 = TRUE;
         _uiSize = dwBufferLength;
         _pDataTop = _pData;
         _pData = pbBufferStart;
      }
   }

   if( !_bLocked2D2 )
   {
      hr = _pInputBuffer.As( &_pInputBuffer2D );
      if( SUCCEEDED( hr ) )
      {
         hr = _pInputBuffer2D->Lock2D( &_pData, &_lPitch );
         if( SUCCEEDED( hr ) )
         {
            hr = _pInputBuffer->GetMaxLength( (DWORD *) &_uiSize );
            if( FAILED( hr ) || abs( _lPitch ) < abs( lDefaultStride ) || _uiSize < dwMinSize )
            {
               _pInputBuffer2D->Unlock2D();
               return MF_E_BUFFERTOOSMALL;
            }
            _uiSize = dwMinSize;
            _bLocked2D = TRUE;
            _pDataTop = _pData;
            if( _lPitch < 0 )
            {
               _pData += _uiSize - abs( _lPitch );
            }
         }
      }
   }

   if( !( _bLocked2D2 || _bLocked2D ) )
   {
      hr = _pInputBuffer->Lock( &_pData, &maxSize, NULL );

      if( SUCCEEDED( hr ) )
      {
         hr = _pInputBuffer->GetMaxLength( (DWORD *) &_uiSize );
         if( FAILED( hr ) || _uiSize < dwMinSize )
         {
            _pInputBuffer->Unlock();
            return MF_E_BUFFERTOOSMALL;
         }
         _uiSize = dwMinSize;
         _bLockedBuffer = TRUE;
         _lPitch = lDefaultStride;
         _pDataTop = _pData;
         if( _lPitch < 0 )
         {
            _pData += _uiSize - abs( _lPitch );
         }
      }
   }
   return hr;
}

HRESULT
VideoBufferLock::lockRemap( __in MF2DBuffer_LockFlags flags, bool topDown, LONG restride )
{
   HRESULT hr = S_OK;
   if( FAILED( hr = lock( flags ) ) )
   {
      return hr;
   }
   if( restride != 0 || topDown && _lPitch < 0 )
   {
      LONG newPitch = _lPitch;
      if( restride != 0 )
      {
         newPitch = restride;
      }
      if( topDown )
      {
         newPitch = abs( _lPitch );
      }
      BYTE *temp = new BYTE[newPitch * lines()];
      if( nullptr == temp )
      {
         return E_OUTOFMEMORY;
      }
      if( FAILED( hr = MFCopyImage( temp, newPitch, _pData, _lPitch, abs( _lPitch ), lines() ) ) )
      {
         delete[] temp;
         return hr;
      }
      _uiSize = newPitch * lines();
      _pData = temp;
      _pDataTop = temp;
      _lPitch = newPitch;
      _localAlloc = true;
   }
   return hr;
}

BOOL
VideoBufferLock::validate( BYTE *ptr ) const
{
   if( _lPitch >= 0 )
   {
      return ptr <= _pData + _uiSize && ptr >= _pData;
   }
   else
   {
      return ptr >= _pData - _uiSize + abs( _lPitch ) && ptr <= _pData + abs( _lPitch );
   }
}

VideoBufferLock &
VideoBufferLock::operator=( VideoBufferLock &&move )
{
   _bLockedBuffer = move._bLockedBuffer;
   _bLocked2D = move._bLocked2D;
   _bLocked2D2 = move._bLocked2D2;
   _localAlloc = move._localAlloc;
   _pDataTop = move._pDataTop;
   _pData = move._pData;
   _lPitch = move._lPitch;
   _pInputBuffer = move._pInputBuffer;
   _uiSize = move._uiSize;
   _pmt = move._pmt;
   move._bLocked2D = FALSE;
   move._bLocked2D2 = FALSE;
   move._bLockedBuffer = FALSE;
   return *this;
}

HRESULT
VideoBufferLock::unlock()
{
   HRESULT hr = S_OK;

   if( _bLockedBuffer && _pInputBuffer )
   {
      hr = _pInputBuffer->Unlock();
      _bLockedBuffer = FALSE;
      _pInputBuffer = nullptr;
   }
   if( _bLocked2D && _pInputBuffer2D )
   {
      hr = _pInputBuffer2D->Unlock2D();
      _bLocked2D = FALSE;
      _pInputBuffer2D = nullptr;
   }
   if( _bLocked2D2 && _pInputBuffer2D2 )
   {
      hr = _pInputBuffer2D2->Unlock2D();
      _bLocked2D2 = FALSE;
      _pInputBuffer2D2 = nullptr;
   }
   if( _localAlloc )
   {
      delete[] _pData;
      _localAlloc = false;
   }
   return hr;
}
