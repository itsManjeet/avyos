`**********************************************************************`
`* This is an include template file for the tracewpp preprocessor.    *`
`*                                                                    *`
`*    Copyright (c) Microsoft Corporation. All rights reserved.       *`
`**********************************************************************`
// template `TemplateFile`

//
//     Defines a set of functions that simplifies
//     user mode registration for tracing
//

#ifdef __cplusplus
extern "C" {
#endif

#ifndef WPPINIT_EXPORT
#define WPPINIT_EXPORT
#endif

#ifndef WppDebug
#define WppDebug(a,b)
#endif

#define __WARNING_BANNED_LEGACY_INSTRUMENTATION_API_USAGE 28735

#if ENABLE_WPP_RECORDER
#ifdef WPP_MACRO_USE_KM_VERSION_FOR_UM
WPPINIT_EXPORT
VOID
__cdecl
WppAutoLogStart(
    _In_ WPP_CB_TYPE * WppCb,
    _In_ PDRIVER_OBJECT DrvObj,
    _In_ PCUNICODE_STRING RegPath
    );
#else
WPPINIT_EXPORT
VOID
__cdecl
WppAutoLogStart(
    _In_ WPP_CB_TYPE * WppCb,
    _In_ PVOID DrvObj,
    _In_ PCUNICODE_STRING RegPath
    );
#endif // WPP_MACRO_USE_KM_VERSION_FOR_UM

WPPINIT_EXPORT
VOID
__cdecl
WppAutoLogStop(
    _In_ WPP_CB_TYPE * WppCb
    );

#endif // ENABLE_WPP_RECORDER

// define annotation record that will carry control information to pdb (in case somebody needs it)
WPP_FORCEINLINE void WPP_CONTROL_ANNOTATION() {
#if !defined(WPP_NO_ANNOTATIONS)

#ifndef WPP_TMC_ANNOT_SUFIX
#ifdef WPP_PUBLIC_TMC
    #define WPP_TMC_ANNOT_SUFIX ,L"PUBLIC_TMF:"
#else
    #define WPP_TMC_ANNOT_SUFIX
#endif
#endif

#  define WPP_DEFINE_CONTROL_GUID(Name,Guid,Bits) __annotation(L"TMC:", WPP_GUID_WTEXT Guid, _WPPW(WPP_STRINGIZE(Name)) Bits WPP_TMC_ANNOT_SUFIX);
#  define WPP_DEFINE_BIT(Name) , _WPPW(#Name)

    WPP_CONTROL_GUIDS
#  undef WPP_DEFINE_BIT
#  undef WPP_DEFINE_CONTROL_GUID
#endif
}

LPCGUID WPP_REGISTRATION_GUIDS[WPP_LAST_CTL];

WPP_CB_TYPE WPP_MAIN_CB[WPP_LAST_CTL];

#define WPP_NEXT(Name) ((WPP_TRACE_CONTROL_BLOCK*) \
    (WPP_XGLUE(WPP_CTL_, WPP_EVAL(Name)) + 1 == WPP_LAST_CTL ? 0:WPP_MAIN_CB + WPP_XGLUE(WPP_CTL_, WPP_EVAL(Name)) + 1))

#if ENABLE_WPP_RECORDER
#define INIT_WPP_RECORDER(Arr)                  \
   Arr->Control.AutoLogContext = NULL;          \
   Arr->Control.AutoLogVerboseEnabled = 0x0;    \
   Arr->Control.AutoLogAttachToMiniDump = 0x0;
#else
#define INIT_WPP_RECORDER(Arr)
#endif

__inline void WPP_INIT_CONTROL_ARRAY(WPP_CB_TYPE* Arr) {
#define WPP_DEFINE_CONTROL_GUID(Name,Guid,Bits)                        \
   Arr->Control.Ptr = NULL;                                            \
   Arr->Control.Next = WPP_NEXT(WPP_EVAL(Name));                       \
   Arr->Control.FlagsLen = WPP_FLAG_LEN;                               \
   Arr->Control.Level = 0;                                             \
   Arr->Control.Options = 0;                                           \
   Arr->Control.Flags[0] = 0;                                          \
   INIT_WPP_RECORDER(Arr)                                              \
   ++Arr;
#define WPP_DEFINE_BIT(BitName) L" " L ## #BitName
WPP_CONTROL_GUIDS
#undef WPP_DEFINE_BIT
#undef WPP_DEFINE_CONTROL_GUID
}

#undef WPP_INIT_STATIC_DATA
#define WPP_INIT_STATIC_DATA WPP_INIT_CONTROL_ARRAY(WPP_MAIN_CB)

__inline void WPP_INIT_GUID_ARRAY(LPCGUID* Arr) {
#define WPP_DEFINE_CONTROL_GUID(Name,Guid,Bits)                         \
   WPP_XGLUE4(*Arr = &WPP_, ThisDir, _CTLGUID_, WPP_EVAL(Name));        \
   ++Arr;
WPP_CONTROL_GUIDS
#undef WPP_DEFINE_CONTROL_GUID
}

VOID WppInitUm(_In_opt_ LPCWSTR AppName);
//
// WPP_INIT_TRACING and WPP_CLEANUP macros are defined differently for kernel
// mode and user mode. In order to support mode-agnostic WDF drivers,
// WPP_INIT_TRACING and WPP_CLEANUP macros for UMDF 2.x user-mode drivers are
// being updated to be same as kernel mode macros. This difference is based
// upon the macro WPP_MACRO_USE_KM_VERSION_FOR_UM.
//

#ifdef WPP_MACRO_USE_KM_VERSION_FOR_UM
VOID WppInitUmDriver(
                     _In_ PDRIVER_OBJECT DrvObject,
                     _In_ PCUNICODE_STRING RegPath
                     );

VOID WppInitUm(_In_opt_ LPCWSTR AppName);

#ifndef WPP_MACRO_USE_KM_VERSION_FOR_UM_IGNORE_VALIDATION

//
// To reduce confusion due to this breaking change, we have overloaded
// WPP_INIT_TRACING with some macro wizardry to notify the developer
// if (s)he is using the deprecated single argument version.
//
// Example of how this works:
//
// 1) WPP_INIT_TRACING is called with two parameters, X and Y.
// 2) FX_WPP_COUNT_ARGUMENTS is called, but due to a bug with VC++ __VA_ARGS__ remains as a single token.
// 3) FX_WPP_EXPAND_ARGUMENTS expands the argument list to (X, Y, 2, 1, 0)
// 4) FX_WPP_DETERMINE_MACRO_NAME puts X into _1_, Y into _2_, 2 into Count (the value we want),
//    and the rest into "..." which are discarded. Count is then combined with WPP_INIT_TRACING.
// 5) WPP_GLUE combines "WPP_INIT_TRACING2" with "(X, Y)" giving us our function call: WPP_INIT_TRACING2(X,Y)
//
#define FX_WPP_DETERMINE_MACRO_NAME(_1_, _2_, Count, ...) WPP_INIT_TRACING##Count
#define FX_WPP_EXPAND_ARGUMENTS(Args) FX_WPP_DETERMINE_MACRO_NAME Args
#define FX_WPP_COUNT_ARGUMENTS(...) FX_WPP_EXPAND_ARGUMENTS((__VA_ARGS__, 2, 1, 0))

#define WPP_INIT_TRACING(...) WPP_GLUE(FX_WPP_COUNT_ARGUMENTS(__VA_ARGS__),(__VA_ARGS__))

//
// _pragma(message("...")) doesn't work with newline characters. "ERROR..." is an
// undocumented keyword in visual studio that causes an error, but this is ignored by razzle.
// Instead we use an undeclared identifier.
//
#define WPP_INIT_TRACING1(AppName) \
        __pragma(message("ERROR: This version of WPP_INIT_TRACING has been deprecated from UMDF 2.15 onwards")) \
        __pragma(message("WPP_INIT_TRACING( ")) \
        __pragma(message("  _In_ PDRIVER_OBJECT DriverObject, ")) \
        __pragma(message("  _In_ PUNICODE_STRING RegistryPath ")) \
        __pragma(message("); ")) \
        __pragma(message("WPP_CLEANUP( ")) \
        __pragma(message("  _In_ PDRIVER_OBJECT DriverObject ")) \
        __pragma(message(");" )) \
        __pragma(message("Please refer to the MSDN documentation on WPP tracing for more information.")) \
        WPP_INIT_TRACING_FUNCTION_IS_DEPRECATED_PLEASE_REFER_TO_BUILD_LOG_FOR_MORE_INFORMATION;

#define WPP_INIT_TRACING2(DrvObj, RegPath)                                   \
                 WppLoadTracingSupport;                                      \
                 (WPP_CONTROL_ANNOTATION(),WPP_INIT_STATIC_DATA,             \
                  WPP_INIT_GUID_ARRAY((LPCGUID*)&WPP_REGISTRATION_GUIDS),    \
                  WPP_CB= WPP_MAIN_CB,                                       \
                  WppInitUmDriver(DrvObj, RegPath))

#else
#define WPP_INIT_TRACING(DrvObj, RegPath)                                    \
                 WppLoadTracingSupport;                                      \
                 (WPP_CONTROL_ANNOTATION(),WPP_INIT_STATIC_DATA,             \
                  WPP_INIT_GUID_ARRAY((LPCGUID*)&WPP_REGISTRATION_GUIDS),    \
                  WPP_CB= WPP_MAIN_CB,                                       \
                  WppInitUmDriver(DrvObj, RegPath))

#endif // WPP_MACRO_USE_KM_VERSION_FOR_UM_IGNORE_VALIDATION
#else
#define WPP_INIT_TRACING(AppName)                                           \
                WppLoadTracingSupport;                                      \
                (WPP_CONTROL_ANNOTATION(),WPP_INIT_STATIC_DATA,             \
                 WPP_INIT_GUID_ARRAY((LPCGUID*)&WPP_REGISTRATION_GUIDS),    \
                 WPP_CB= WPP_MAIN_CB,                                       \
                 WppInitUm(AppName))

#endif // WPP_MACRO_USE_KM_VERSION_FOR_UM
void WPP_Set_Dll_CB(
                    PWPP_TRACE_CONTROL_BLOCK Control,
                    VOID * DllControlBlock,
                    USHORT Flags)
{

    if (*(PVOID*)DllControlBlock != DllControlBlock){
        Control->Ptr = DllControlBlock;
    } else {
        if (Flags == WPP_VER_WHISTLER_CB_FORWARD_PTR ){
            memset(Control, 0, sizeof(WPP_TRACE_CONTROL_BLOCK));
            *(PWPP_TRACE_CONTROL_BLOCK*)DllControlBlock = Control;
            Control->Options = WPP_VER_LH_CB_FORWARD_PTR;

        }
    }

}


#define WPP_SET_FORWARD_PTR(CTL, FLAGS, PTR) (\
    (WPP_MAIN_CB[WPP_CTRL_NO(WPP_BIT_ ## CTL )].Control.Options = (FLAGS)));\
    WPP_Set_Dll_CB(&WPP_MAIN_CB[WPP_CTRL_NO(WPP_BIT_ ## CTL )].Control,(PTR),(USHORT)FLAGS)


#define DEFAULT_LOGGER_NAME             L"stdout"

#if !defined(WPPINIT_STATIC)
#  define WPPINIT_STATIC
#endif

#define WPP_GUID_FORMAT     "%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x"
#define WPP_GUID_ELEMENTS(p) \
    p->Data1,                 p->Data2,    p->Data3,\
    p->Data4[0], p->Data4[1], p->Data4[2], p->Data4[3],\
    p->Data4[4], p->Data4[5], p->Data4[6], p->Data4[7]

#define WPP_MAX_LEVEL 255
#define WPP_MAX_FLAGS 0xFFFFFFFF



#if defined (WPP_GLOBALLOGGER)

__inline
TRACEHANDLE
WppQueryLogger(
    _In_opt_ PCWSTR LoggerName
    )
{
    ULONG Status;
    EVENT_TRACE_PROPERTIES LoggerInfo;

    ZeroMemory(&LoggerInfo, sizeof(LoggerInfo));
    LoggerInfo.Wnode.BufferSize = sizeof(LoggerInfo);
    LoggerInfo.Wnode.Flags = WNODE_FLAG_TRACED_GUID;

    Status = ControlTraceW(0, LoggerName ? LoggerName : L"stdout", &LoggerInfo, EVENT_TRACE_CONTROL_QUERY);
    if (Status == ERROR_SUCCESS || Status == ERROR_MORE_DATA) {
        return (TRACEHANDLE) LoggerInfo.Wnode.HistoricalContext;
    }
    return 0;
}

#define WPP_REG_GLOBALLOGGER_FLAGS             L"Flags"
#define WPP_REG_GLOBALLOGGER_LEVEL             L"Level"
#define WPP_REG_GLOBALLOGGER_START             L"Start"

#define WPP_TEXTGUID_LEN  38
#define WPP_REG_GLOBALLOGGER_KEY            L"SYSTEM\\CurrentControlSet\\Control\\Wmi\\GlobalLogger"

WPPINIT_STATIC
void WppIntToHex(
    _Out_writes_(digits) PWCHAR Buf,
    unsigned int value,
    int digits
    )
{
    static LPCWSTR hexDigit = L"0123456789abcdef";
    while (--digits >= 0) {
        Buf[digits] = hexDigit[ value & 15 ];
        value /= 16;
    }
}

WPPINIT_EXPORT
void WppInitGlobalLogger(
        IN LPCGUID ControlGuid,
        IN PTRACEHANDLE LoggerHandle,
        OUT PULONG Flags,
        _Out_writes_(sizeof(UCHAR)) PUCHAR Level )
{
WCHAR    GuidBuf[WPP_TEXTGUID_LEN];
ULONG    CurrentFlags = 0;
ULONG    CurrentLevel = 0;
DWORD    Start = 0;
DWORD    DataSize ;
ULONG    Status ;
HKEY     GloblaLoggerHandleKey;
HKEY     ValueHandleKey ;



   WppDebug(0,("WPP checking Global Logger %S",WPP_REG_GLOBALLOGGER_KEY));

   if ((Status = RegOpenKeyExW(HKEY_LOCAL_MACHINE,
                        (LPWSTR)WPP_REG_GLOBALLOGGER_KEY,
                        0,
                        KEY_READ,
                        &GloblaLoggerHandleKey
                        )) != ERROR_SUCCESS) {
       WppDebug(0,("GlobalLogger key does not exist (0x%08X)",Status));
       return ;
   }

   DataSize = sizeof(DWORD);
   Status = RegQueryValueExW(GloblaLoggerHandleKey,
                             (LPWSTR)WPP_REG_GLOBALLOGGER_START,
                             0,
                             NULL,
                             (LPBYTE)&Start,
                             &DataSize);
    if (Status != ERROR_SUCCESS || Start == 0 ) {
        WppDebug(0,("Global Logger not started (0x%08X)",Status));
        goto Cleanup;
    }


   WppDebug(0,("Global Logger exists and is set to be started"));

   {
        static LPCWSTR hexDigit = L"0123456789abcdef";
        int i;

        WppIntToHex(GuidBuf, ControlGuid->Data1, 8);
        GuidBuf[8]  = '-';

        WppIntToHex(&GuidBuf[9], ControlGuid->Data2, 4);
        GuidBuf[13] = '-';

        WppIntToHex(&GuidBuf[14], ControlGuid->Data3, 4);
        GuidBuf[18] = '-';

        GuidBuf[19] =  hexDigit[(ControlGuid->Data4[0] & 0xF0) >> 4];
        GuidBuf[20] =  hexDigit[ControlGuid->Data4[0] & 0x0F ];
        GuidBuf[21] =  hexDigit[(ControlGuid->Data4[1] & 0xF0) >> 4];
        GuidBuf[22] =  hexDigit[ControlGuid->Data4[1] & 0x0F ];
        GuidBuf[23] = '-';

        for( i=2; i < 8 ; i++ ){
            GuidBuf[i*2+20] =  hexDigit[(ControlGuid->Data4[i] & 0xF0) >> 4];
            GuidBuf[i*2+21] =  hexDigit[ControlGuid->Data4[i] & 0x0F ];
        }
        GuidBuf[36] = 0;

    }

   //
   // Perform the query
   //

   if ((Status = RegOpenKeyExW(GloblaLoggerHandleKey,
                        (LPWSTR)GuidBuf,
                        0,
                        KEY_READ,
                        &ValueHandleKey
                        )) != ERROR_SUCCESS) {
       WppDebug(0,("Global Logger Key not set for this Control Guid %S (0x%08X)",GuidBuf,Status));
       goto Cleanup;
   }
   // Get the Flags Parameter
   DataSize = sizeof(DWORD);
   Status = RegQueryValueExW(ValueHandleKey,
                             (LPWSTR)WPP_REG_GLOBALLOGGER_FLAGS,
                             0,
                             NULL,
                             (LPBYTE)&CurrentFlags,
                             &DataSize);
    if (Status != ERROR_SUCCESS || CurrentFlags == 0 ) {
        WppDebug(0,("GlobalLogger for %S Flags not set (0x%08X)",GuidBuf,Status));
    }
   // Get the levels Parameter
   DataSize = sizeof(DWORD);
   Status = RegQueryValueExW(ValueHandleKey,
                             (LPWSTR)WPP_REG_GLOBALLOGGER_LEVEL,
                             0,
                             NULL,
                             (LPBYTE)&CurrentLevel,
                             &DataSize);
    if (Status != ERROR_SUCCESS || CurrentLevel == 0 ) {
        WppDebug(0,("GlobalLogger for %S Level not set (0x%08X)",GuidBuf,Status));
    }

    if (Start==1) {

       if ((*LoggerHandle= WppQueryLogger( L"GlobalLogger")) != (TRACEHANDLE)NULL) {
           *Flags = CurrentFlags & 0x7FFFFFFF ;
           *Level = (UCHAR)(CurrentLevel & 0xFF) ;
           WppDebug(0,("WPP Enabled via Global Logger Flags=0x%08X Level=0x%02X",CurrentFlags,CurrentLevel));
       } else {
           WppDebug(0,("GlobalLogger set for start but not running (Flags=0x%08X Level=0x%02X)",CurrentFlags,CurrentLevel));
       }

    }

   RegCloseKey(ValueHandleKey);
Cleanup:
   RegCloseKey(GloblaLoggerHandleKey);
}
#endif  //#ifdef WPP_GLOBALLOGGER

#ifdef WPP_MANAGED_CPP
#pragma managed(push, off)
#endif

ULONG
__stdcall
WppControlCallback(
    IN WMIDPREQUESTCODE RequestCode,
    IN PVOID Context,
    _Inout_ ULONG *InOutBufferSize,
    _Inout_ PVOID Buffer
    )
{
    PWPP_TRACE_CONTROL_BLOCK Ctx = (PWPP_TRACE_CONTROL_BLOCK)Context;
    TRACEHANDLE Logger;
    UCHAR Level;
    DWORD Flags;

    *InOutBufferSize = 0;

    switch (RequestCode)
    {
        case WMI_ENABLE_EVENTS:
        {
            Logger = WPP_GET_TRACE_LOGGER_HANDLE( Buffer );
            Level = WPP_GET_TRACE_ENABLE_LEVEL(Logger);
            Flags = WPP_GET_TRACE_ENABLE_FLAGS(Logger);

            WppDebug(1, ("[WppInit] WMI_ENABLE_EVENTS Ctx %p Flags %x"
                     " Lev %d Logger %I64x\n",
                     Ctx, Flags, Level, Logger) );
            break;
        }

        case WMI_DISABLE_EVENTS:
        {
            Logger = 0;
            Flags  = 0;
            Level  = 0;
            WppDebug(1, ("[WppInit] WMI_DISABLE_EVENTS Ctx 0x%08p\n", Ctx));
            break;
        }

#ifdef WPP_CAPTURE_STATE_CALLBACK

        case WMI_CAPTURE_STATE:
        {
            Logger = WPP_GET_TRACE_LOGGER_HANDLE(Buffer);
            Level = WPP_GET_TRACE_ENABLE_LEVEL(Logger);
            Flags = WPP_GET_TRACE_ENABLE_FLAGS(Logger);
            WPP_CAPTURE_STATE_CALLBACK(Ctx->ControlGuid,
                                       Logger,
                                       Flags,
                                       Level);

            return ERROR_SUCCESS;
        }

#endif

        default:
        {
            return ERROR_INVALID_PARAMETER;
        }
    }

    if (Ctx->Options & WPP_VER_WHISTLER_CB_FORWARD_PTR && Ctx->Cb) {
        Ctx = Ctx->Cb; // use forwarding address
    }

    Ctx->Logger   = Logger;
    Ctx->Level    = Level;
    Ctx->Flags[0] = Flags;

#ifdef WPP_PRIVATE_ENABLE_CALLBACK
    WPP_PRIVATE_ENABLE_CALLBACK(Ctx->ControlGuid,
                                Logger,
                                (RequestCode != WMI_DISABLE_EVENTS) ? TRUE : FALSE,
                                Flags,
                                Level);
#endif

    return(ERROR_SUCCESS);
}

#ifdef WPP_MANAGED_CPP
#pragma managed(pop)
#endif

#pragma warning(push)
#pragma warning(disable:4068)


#ifdef WPP_MACRO_USE_KM_VERSION_FOR_UM
WPPINIT_EXPORT
VOID WppInitUmDriver(
    _In_ PDRIVER_OBJECT DrvObject,
    _In_ PCUNICODE_STRING RegPath
    )
{
    WppInitUm(L"UMDF Driver");

#if ENABLE_WPP_RECORDER
    WppAutoLogStart(&WPP_CB[0], DrvObject, RegPath);
#else
    UNREFERENCED_PARAMETER(DrvObject);
    UNREFERENCED_PARAMETER(RegPath);
#endif // ENABLE_WPP_RECORDER
}

#endif // WPP_MACRO_USE_KM_VERSION_FOR_UM

WPPINIT_EXPORT
VOID WppInitUm(_In_opt_ LPCWSTR AppName)
{
    C_ASSERT(WPP_MAX_FLAG_LEN_CHECK);

    PWPP_TRACE_CONTROL_BLOCK Control = &WPP_CB[0].Control;
    TRACE_GUID_REGISTRATION TraceRegistration;
    LPCGUID *               RegistrationGuids = (LPCGUID *)&WPP_REGISTRATION_GUIDS;
    LPCGUID                 ControlGuid;

    ULONG Status;

#ifdef WPP_MOF_RESOURCENAME
#ifdef WPP_DLL
    HMODULE hModule = NULL;
#endif
    WCHAR ImagePath[MAX_PATH] = {UNICODE_NULL} ;
    WCHAR WppMofResourceName[] = WPP_MOF_RESOURCENAME ;
#else
    UNREFERENCED_PARAMETER(AppName);
#endif //#ifdef WPP_MOF_RESOURCENAME

    WppDebug(1, ("Registering %ws\n", AppName) );

    for(; Control; Control = Control->Next) {

        ControlGuid = *RegistrationGuids++;
        TraceRegistration.Guid = ControlGuid;
        TraceRegistration.RegHandle = 0;
        Control->ControlGuid = ControlGuid;

        WppDebug(1,(WPP_GUID_FORMAT " %ws : %d\n",
                    WPP_GUID_ELEMENTS(ControlGuid),
                    AppName,
                    Control->FlagsLen));


#ifdef WPP_MOF_RESOURCENAME
        if (AppName != NULL) {

#ifdef WPP_DLL
           if ((hModule = GetModuleHandleW(AppName)) != NULL) {
               Status = GetModuleFileNameW(hModule, ImagePath, MAX_PATH) ;
               ImagePath[MAX_PATH-1] = '\0';
               if (Status == 0) {
                  WppDebug(1,("RegisterTraceGuids => GetModuleFileName(DLL) Failed 0x%08X\n",GetLastError()));
               }
           } else {
               WppDebug(1,("RegisterTraceGuids => GetModuleHandleW failed for %ws (0x%08X)\n",AppName,GetLastError()));
           }
#else   // #ifdef WPP_DLL
           Status = GetModuleFileNameW(NULL,ImagePath,MAX_PATH);
           if (Status == 0) {
               WppDebug(1,("GetModuleFileName(EXE) Failed 0x%08X\n",GetLastError()));
           }
#endif  //  #ifdef WPP_DLL
        }
        WppDebug(1,("registerTraceGuids => registering with WMI, App=%ws, Mof=%ws, ImagePath=%ws\n",AppName,WppMofResourceName,ImagePath));

#pragma prefast(suppress:__WARNING_BANNED_LEGACY_INSTRUMENTATION_API_USAGE, "WPP generated, requires legacy providers");
        Status = RegisterTraceGuidsW(                   // Always use Unicode
#else   // ifndef WPP_MOF_RESOURCENAME

#pragma prefast(suppress:__WARNING_BANNED_LEGACY_INSTRUMENTATION_API_USAGE, "WPP generated, requires legacy providers");
        Status = WPP_REGISTER_TRACE_GUIDS(
#endif  // ifndef WPP_MOF_RESOURCENAME

            WppControlCallback,
            Control,              // Context for the callback
            ControlGuid,
            1,
            &TraceRegistration,
#ifndef WPP_MOF_RESOURCENAME
            0, //ImagePath,
            0, //ResourceName,
#else   // #ifndef WPP_MOF_RESOURCENAME
            ImagePath,
            WppMofResourceName,
#endif // #ifndef WPP_MOF_RESOURCENAME
            &Control->UmRegistrationHandle
        );


        if (Status != ERROR_SUCCESS) {

            WppDebug(1, ("RegisterTraceGuid failed %d\n", Status) );

        }

#if defined (WPP_GLOBALLOGGER)

        //
        // Check if Global logger is active if we have not been immediately activated
        //
        if (Control->Logger == (TRACEHANDLE)NULL) {
            WppInitGlobalLogger( ControlGuid, (PTRACEHANDLE)&Control->Logger, &Control->Flags[0], &Control->Level);
        }

#endif  //#if defined (WPP_GLOBALLOGGER)

    }

#if ENABLE_WPP_RECORDER
#ifndef WPP_MACRO_USE_KM_VERSION_FOR_UM
    UNICODE_STRING AppNameStr;
    if (NULL != AppName) {
        RtlInitUnicodeString( &AppNameStr, AppName);
    }
    WppAutoLogStart(&WPP_CB[0], NULL, AppName ? &AppNameStr : NULL);
#endif
    WPP_RECORDER_INITIALIZED = WPP_MAIN_CB;
#endif
}

WPPINIT_EXPORT
VOID WppCleanupUm(    VOID   )
{
    PWPP_TRACE_CONTROL_BLOCK Control;

    if (WPP_CB == (WPP_CB_TYPE*)&WPP_CB){
        //
        // WPP_INIT_TRACING macro has not been called
        //
        return;
    }
    WppDebug(1, ("Cleanup\n") );
    Control = &WPP_CB[0].Control;
    for(; Control; Control = Control->Next) {
        WppDebug(1,("UnRegistering %I64x\n", Control->UmRegistrationHandle) );
        if (Control->UmRegistrationHandle) {

#pragma prefast(suppress:__WARNING_BANNED_LEGACY_INSTRUMENTATION_API_USAGE, "WPP generated, requires legacy providers");
            WPP_UNREGISTER_TRACE_GUIDS(Control->UmRegistrationHandle);

            Control->UmRegistrationHandle = (TRACEHANDLE)NULL ;
        }
    }

#if ENABLE_WPP_RECORDER
    WppAutoLogStop(&WPP_CB[0]);

    WPP_RECORDER_INITIALIZED = (WPP_CB_TYPE*) &WPP_RECORDER_INITIALIZED;
#endif

    WPP_CB = (WPP_CB_TYPE*)&WPP_CB;
}

#pragma warning(pop)


#ifdef __cplusplus
} // extern "C"
#endif
