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
#include <Windows.h>
#include <mfapi.h>
#include <mferror.h>
#include <utility>
#include <wrl/client.h>
using namespace Microsoft::WRL;

// RAII class for safely taking IMFMediaBuffer locks.
//  Provides extended information about the buffer using
//  the IMFMediaType associated with the buffer
class VideoBufferLock
{
 public:
   VideoBufferLock()
   { }

   // Main ctor
   VideoBufferLock( IMFMediaBuffer *pBuffer, IMFMediaType *pmt ) : _pInputBuffer( pBuffer ), _pmt( pmt )
   { }

   // Shortcut ctor that calls GetBufferByIndex for you
   VideoBufferLock( IMFSample *pSamp, IMFMediaType *pmt, UINT index = 0 ) : _pmt( pmt )
   {
      // If this call fails, _pInputBuffer will be NULL which will fail lock()
      (void) pSamp->GetBufferByIndex( index, &_pInputBuffer );
   }

   // Only support move semantics
   VideoBufferLock( VideoBufferLock &&move )
   {
      *this = std::move( move );
   }

   // Only support move semantics
   VideoBufferLock &operator=( VideoBufferLock &&move );

   // Lock a video buffer
   HRESULT lock( __in MF2DBuffer_LockFlags flags = MF2DBuffer_LockFlags_ReadWrite );

   // Lock a video buffer, if the buffer does not match the given topDown and stride parameters the buffer
   // will be copied into a buffer matching those properties.
   HRESULT
   lockRemap( __in MF2DBuffer_LockFlags flags = MF2DBuffer_LockFlags_ReadWrite, bool topDown = true, LONG restride = 0 );

   // Unlock video buffer
   HRESULT unlock();

   ~VideoBufferLock()
   {
      unlock();
   }

   // Returns a pointer to the buffer
   BYTE *data() const
   {
      return _pData;
   }

   // Returns a pointer to the top of the buffer (no stride adjustment)
   BYTE *dataTop() const
   {
      return _pDataTop;
   }

   // Returns the size of the buffer in bytes
   UINT size() const
   {
      return _uiSize;
   }

   // Returns the size of one line of the buffer in bytes,
   //   may be negative for bottom-up images
   LONG stride() const
   {
      return _lPitch;
   }

   // Returns the number of lines in the image (count of stride in size)
   LONG lines() const
   {
      return _uiSize / abs( _lPitch );
   }

   // Validate if the given pointer is inside the buffer
   BOOL validate( BYTE *ptr ) const;

 private:
   ComPtr<IMFMediaType> _pmt;

   ComPtr<IMFMediaBuffer> _pInputBuffer;
   BOOL _bLockedBuffer = FALSE;

   ComPtr<IMF2DBuffer> _pInputBuffer2D;
   BOOL _bLocked2D = FALSE;

   ComPtr<IMF2DBuffer2> _pInputBuffer2D2;
   BOOL _bLocked2D2 = FALSE;

   BYTE *_pData = nullptr;
   BYTE *_pDataTop = nullptr;
   LONG _lPitch = 0;
   UINT _uiSize = 0;

   BOOL _localAlloc = FALSE;
};
