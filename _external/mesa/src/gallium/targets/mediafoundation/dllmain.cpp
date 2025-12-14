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
#define UNICODE
#include "gallium/frontends/mediafoundation/hmft_entrypoints.h"
#include "gallium/frontends/mediafoundation/macros.h"
#include <strsafe.h>
#include <initguid.h>

using namespace Microsoft::WRL;

extern "C" BOOL WINAPI
DllEntryPoint(HINSTANCE, ULONG, __inout_opt LPVOID);

#if MFT_CODEC_H264ENC
const wchar_t *g_pMFTFriendlyName =
   L"Microsoft AVC DX12 Encoder HMFT";
DEFINE_GUID(CLSID_CDX12EncoderHMFT, 0x8994db7c, 0x288a, 0x4c62, 0xa1, 0x36, 0xa3, 0xc3, 0xc2, 0xa2, 0x08, 0xa8);
#elif MFT_CODEC_H265ENC
const wchar_t *g_pMFTFriendlyName =
   L"Microsoft HEVC DX12 Encoder HMFT";
DEFINE_GUID(CLSID_CDX12EncoderHMFT, 0xe7ffb8eb, 0xfa0b, 0x4fb0, 0xac, 0xdf, 0x12, 0x2, 0xf6, 0x63, 0xcd, 0xe5);
#elif MFT_CODEC_AV1ENC
const wchar_t *g_pMFTFriendlyName =
   L"Microsoft AV1 DX12 Encoder HMFT";
DEFINE_GUID(CLSID_CDX12EncoderHMFT, 0x1a6f3150, 0xb121, 0x4ce9, 0x94, 0x97, 0x50, 0xfe, 0xdb, 0x3d, 0xcb, 0x70);
#else
#error MFT_CODEC_xxx must be defined
#endif

HINSTANCE g_hModule = nullptr;

void WppInit();
void WppClean();

#if !defined(__WRL_CLASSIC_COM__)
STDAPI
DllGetActivationFactory(_In_ HSTRING activatibleClassId, _COM_Outptr_ IActivationFactory **factory)
{
   return Module<InProc>::GetModule().GetActivationFactory(activatibleClassId, factory);
}
#endif

#if !defined(__WRL_WINRT_STRICT__)
STDAPI
DllGetClassObject(REFCLSID rclsid, REFIID riid, _COM_Outptr_ void **ppv)
{
   return Module<InProc>::GetModule().GetClassObject(rclsid, riid, ppv);
}
#endif

STDAPI
DllCanUnloadNow()
{
   return Module<InProc>::GetModule().Terminate() ? S_OK : S_FALSE;
}

STDAPI_(BOOL) DllMain(_In_opt_ HMODULE hModule, DWORD reason, _In_opt_ LPVOID lpReserved)
{
   BOOL bRetValue = TRUE;

   switch (reason) {
      case DLL_PROCESS_ATTACH:
         WppInit();
         if (hModule != nullptr) {
            g_hModule = hModule;
            DisableThreadLibraryCalls(hModule);
         } else {
            bRetValue = FALSE;
         }
         Module<InProc>::GetModule().Create();
         break;
      case DLL_PROCESS_DETACH:
         if (NULL == lpReserved) {
            Module<InProc>::GetModule().Terminate();
         }
         WppClean();
         break;
   }
   return bRetValue;
}

#ifndef BUILD_FOR_MSDK

HRESULT
RegisterMFT(REFIID riid)
{
   ComPtr<IMFAttributes> spAttributes;
   if(SUCCEEDED(MFCreateAttributes(&spAttributes, 4)))
   {
      spAttributes->SetString(MFT_ENUM_HARDWARE_VENDOR_ID_Attribute, L"VEN_1414");
      spAttributes->SetString(MFT_ENUM_HARDWARE_URL_Attribute, g_pMFTFriendlyName);
      spAttributes->SetUINT32(MF_SA_D3D11_AWARE, TRUE);
      spAttributes->SetUINT32(MF_SA_D3D12_AWARE, TRUE);
   }

   return MFTRegister(riid,
                      MFT_CATEGORY_VIDEO_ENCODER,
                      (LPWSTR) g_pMFTFriendlyName,
                      MFT_ENUM_FLAG_ASYNCMFT | MFT_ENUM_FLAG_HARDWARE,
                      ARRAYSIZE(rgInputInfo),
                      rgInputInfo,
                      1, // 1 output type
                      &rgOutputInfo,
                      spAttributes.Get());
}

HRESULT
UnregisterMFT(REFIID riid)
{
   return MFTUnregister(riid);
}

HRESULT
WriteClassToRegistry(__in PTSTR pwszFilename, REFIID riid)
{
   HRESULT hr = (pwszFilename != nullptr) ? S_OK : E_POINTER;
   if (SUCCEEDED(hr)) {
      WCHAR wszClsid[40];
      hr = (StringFromGUID2(riid, wszClsid, ARRAYSIZE(wszClsid)) > 0) ? S_OK : E_INVALIDARG;
      if (SUCCEEDED(hr)) {
         HKEY pkeyClass = nullptr;
         hr = HRESULT_FROM_WIN32(RegCreateKeyEx(HKEY_CLASSES_ROOT,
                                                L"CLSID",
                                                0,
                                                nullptr,
                                                REG_OPTION_NON_VOLATILE,
                                                KEY_WRITE | KEY_READ | KEY_CREATE_SUB_KEY | KEY_ENUMERATE_SUB_KEYS,
                                                nullptr,
                                                &pkeyClass,
                                                nullptr));
         if (SUCCEEDED(hr)) {
            HKEY pkeyMFT = nullptr;
            hr = HRESULT_FROM_WIN32(RegCreateKeyEx(pkeyClass,
                                                   wszClsid,
                                                   0,
                                                   nullptr,
                                                   REG_OPTION_NON_VOLATILE,
                                                   KEY_WRITE | KEY_READ | KEY_CREATE_SUB_KEY | KEY_ENUMERATE_SUB_KEYS,
                                                   nullptr,
                                                   &pkeyMFT,
                                                   nullptr));
            if (SUCCEEDED(hr)) {
               HKEY pkeyInproc = nullptr;
               hr =
                  HRESULT_FROM_WIN32(RegCreateKeyEx(pkeyMFT,
                                                    L"InprocServer32",
                                                    0,
                                                    nullptr,
                                                    REG_OPTION_NON_VOLATILE,
                                                    KEY_WRITE | KEY_READ | KEY_CREATE_SUB_KEY | KEY_ENUMERATE_SUB_KEYS,
                                                    nullptr,
                                                    &pkeyInproc,
                                                    nullptr));
               if (SUCCEEDED(hr)) {
                  WCHAR wszValue[_MAX_PATH];
                  DWORD dwValSize = _MAX_PATH * sizeof(WCHAR);
                  DWORD dwType;
                  hr = HRESULT_FROM_WIN32(
                     RegQueryValueEx(pkeyInproc, nullptr, nullptr, &dwType, (LPBYTE) wszValue, &dwValSize));
                  if (SUCCEEDED(hr)) {
                     if (_wcsicmp(wszValue, pwszFilename)) {
                        hr = E_FAIL;
                     }
                  }

                  // need to put the new dll name
                  if (!SUCCEEDED(hr)) {
                     // Safe to cast length of ptszFilename calculation since buffer is MAX_PATH (26) length
                     hr = HRESULT_FROM_WIN32(RegSetKeyValue(pkeyInproc,
                                                            nullptr,
                                                            nullptr,
                                                            REG_SZ,
                                                            pwszFilename,
                                                            (DWORD) (wcslen(pwszFilename) * sizeof(WCHAR))));
                     if (SUCCEEDED(hr)) {
                        PCTSTR pcwszThreadingModel = L"Both";
                        hr = HRESULT_FROM_WIN32(RegSetKeyValue(pkeyInproc,
                                                               nullptr,
                                                               L"ThreadingModel",
                                                               REG_SZ,
                                                               pcwszThreadingModel,
                                                               (DWORD) (wcslen(pcwszThreadingModel) * sizeof(WCHAR))));
                     }
                  }
                  RegCloseKey(pkeyInproc);
               }
               RegCloseKey(pkeyMFT);
            }
            RegCloseKey(pkeyClass);
         }
      }
   }
   return hr;
}

HRESULT
RemoveClassFromRegistry(REFIID riid)
{
   WCHAR wszClsid[40];
   WCHAR wszPath[80];
   HRESULT hr = S_OK;

   do {
      CHECKHR_GOTO(((StringFromGUID2(riid, wszClsid, ARRAYSIZE(wszClsid)) > 0) ? S_OK : E_INVALIDARG), done);
      CHECKHR_GOTO((StringCchPrintf(wszPath, 80, L"CLSID\\%s", wszClsid)), done);
      CHECKHR_GOTO(HRESULT_FROM_WIN32(RegDeleteTree(HKEY_CLASSES_ROOT, wszPath)), done);
   } while (FALSE);

done:
   return hr;
}
#endif

STDAPI
DllRegisterServer()
{
#ifndef BUILD_FOR_MSDK
   HINSTANCE hMod = g_hModule;
   if (hMod == nullptr) {
      hMod = GetModuleHandle(nullptr);
   }
   HRESULT hr = (hMod != nullptr) ? S_OK : E_UNEXPECTED;
   if (SUCCEEDED(hr)) {
      WCHAR szFilename[MAX_PATH];
      hr =
         (0 == GetModuleFileName(hMod, szFilename, ARRAYSIZE(szFilename))) ? S_OK : HRESULT_FROM_WIN32(GetLastError());
      if (SUCCEEDED(hr)) {
         hr = WriteClassToRegistry(szFilename, CLSID_CDX12EncoderHMFT);
         if (SUCCEEDED(hr)) {
            hr = RegisterMFT(CLSID_CDX12EncoderHMFT);
         }

         if (FAILED(hr)) {
            UnregisterMFT(CLSID_CDX12EncoderHMFT);
            RemoveClassFromRegistry(CLSID_CDX12EncoderHMFT);
         }
      }
   }
   return hr;
#else    // BUILD_FOR_MSDK
   return E_FAIL;
#endif   // BUILD_FOR_MSDK
}

STDAPI
DllUnregisterServer()
{
#ifndef BUILD_FOR_MSDK
   HRESULT hr = RemoveClassFromRegistry(CLSID_CDX12EncoderHMFT);
   if (SUCCEEDED(hr)) {
      hr = UnregisterMFT(CLSID_CDX12EncoderHMFT);
   }
   return hr;
#else    // BUILD_FOR_MSDK
   return E_FAIL;
#endif   // BUILD_FOR_MSDK
}
