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
#define CHECKHR_GOTO( hresult, label )                                                                                             \
   {                                                                                                                               \
      hr = hresult;                                                                                                                \
      if( FAILED( hr ) )                                                                                                           \
      {                                                                                                                            \
         debug_printf( "\nerror in %s, line=%u in %s hr=0x%08x\n", __FUNCTION__, __LINE__, __FILE__, hr );                         \
         goto label;                                                                                                               \
      }                                                                                                                            \
   }
#define CHECKHR_HRGOTO( hresult, newhresult, label )                                                                               \
   {                                                                                                                               \
      hr = hresult;                                                                                                                \
      if( FAILED( hr ) )                                                                                                           \
      {                                                                                                                            \
         CHECKHR_GOTO( newhresult, label );                                                                                        \
      }                                                                                                                            \
   }
#define CHECKBOOL_GOTO( exp, err, label )                                                                                          \
   if( !( exp ) )                                                                                                                  \
   {                                                                                                                               \
      CHECKHR_GOTO( err, label );                                                                                                  \
   }
#define CHECKNULL_GOTO( exp, err, label ) CHECKBOOL_GOTO( ( exp ), err, label )
#define SAFE_RELEASE( x )                                                                                                          \
   if( ( x ) )                                                                                                                     \
      ( x )->Release();
#define SAFE_DELETE( x )                                                                                                           \
   if( ( x ) )                                                                                                                     \
   {                                                                                                                               \
      delete ( x );                                                                                                                \
      ( x ) = nullptr;                                                                                                             \
   }
#define SAFE_CLOSEHANDLE( x )                                                                                                      \
   if( ( x ) )                                                                                                                     \
   {                                                                                                                               \
      CloseHandle( ( x ) );                                                                                                        \
      ( x ) = nullptr;                                                                                                             \
   }
