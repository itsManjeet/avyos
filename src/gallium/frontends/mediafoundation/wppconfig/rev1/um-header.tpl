`**********************************************************************`
`* This is an include template file for the tracewpp preprocessor.    *`
`*                                                                    *`
`*    Copyright (c) Microsoft Corporation. All rights reserved.       *`
`**********************************************************************`
// template `TemplateFile`

#ifdef  WPP_THIS_FILE
// included twice
#       define  WPP_ALREADY_INCLUDED
#       undef   WPP_THIS_FILE
#endif  // #ifdef WPP_THIS_FILE

#define WPP_THIS_FILE `SourceFile.CanonicalName`

#ifndef WPP_ALREADY_INCLUDED

`* Dump the definitions specified via -D on the command line to WPP *`
`FORALL def IN MacroDefinitions`
#define `def.Name` `def.Alias`
`ENDFOR`

#ifdef BUILD_MINWIN
#include <windowscore.h>
#else
#include <windows.h>
#endif

#include <wmistr.h>
#include <evntrace.h>
#ifndef WPP_TRACE_W2K_COMPATABILITY
#include <sddl.h>
#endif

#ifdef __cplusplus
extern "C" {
#endif

// LEGACY: This structure was used by Win2k RpcRt4 and cluster tracing

typedef struct _WPP_WIN2K_CONTROL_BLOCK {
    TRACEHANDLE Logger;
    ULONG Flags;
    ULONG Level;
    LPCGUID ControlGuid;
} WPP_WIN2K_CONTROL_BLOCK, *PWPP_WIN2K_CONTROL_BLOCK;

#pragma warning(push)
#pragma warning(disable:4201) // nonstandard extension used nameless struct/union

typedef struct _WPP_TRACE_CONTROL_BLOCK
{
    struct _WPP_TRACE_CONTROL_BLOCK *Next;
    TRACEHANDLE     UmRegistrationHandle;
    union {
        TRACEHANDLE              Logger;
        PWPP_WIN2K_CONTROL_BLOCK Win2kCb;
        PVOID                    Ptr;
        struct _WPP_TRACE_CONTROL_BLOCK *Cb;
    };

    UCHAR           FlagsLen;
    UCHAR           Level;
    USHORT          Options;
    ULONG           Flags[1];
    LPCGUID         ControlGuid;
#if ENABLE_WPP_RECORDER
    PVOID           AutoLogContext;
    USHORT          AutoLogVerboseEnabled;
    USHORT          AutoLogAttachToMiniDump;
#endif
} WPP_TRACE_CONTROL_BLOCK, *PWPP_TRACE_CONTROL_BLOCK;
#pragma warning(pop)

#ifdef WPP_USE_NTDLL_FUNCTIONS
// Not supported.
#include <wpp_use_ntdll_functions.h>
#endif

#define WPP_IsValidSid IsValidSid
#define WPP_GetLengthSid GetLengthSid

#ifndef WPP_TRACE
#define WPP_TRACE TraceMessage
#endif

#ifndef WPP_UNREGISTER_TRACE_GUIDS
#define WPP_UNREGISTER_TRACE_GUIDS  UnregisterTraceGuids
#endif


#ifndef WPP_REGISTER_TRACE_GUIDS
#define WPP_REGISTER_TRACE_GUIDS  RegisterTraceGuids
#endif


#ifndef WPP_GET_TRACE_LOGGER_HANDLE
#define WPP_GET_TRACE_LOGGER_HANDLE  GetTraceLoggerHandle
#endif

#ifndef WPP_GET_TRACE_ENABLE_LEVEL
#define WPP_GET_TRACE_ENABLE_LEVEL  GetTraceEnableLevel
#endif

#ifndef WPP_GET_TRACE_ENABLE_FLAGS
#define WPP_GET_TRACE_ENABLE_FLAGS  GetTraceEnableFlags
#endif

enum {
    WPP_VER_WIN2K_CB_FORWARD_PTR    = 0x01,
    WPP_VER_WHISTLER_CB_FORWARD_PTR = 0x02,
    WPP_VER_LH_CB_FORWARD_PTR = 0x03
};

#if ENABLE_WPP_RECORDER

#ifndef WPP_RECORDER
#define WPP_RECORDER WppAutoLogTrace
#endif

//
// This setting is only applicable when IFR is enabled.
// Setting this to 1 will allow a WPP trace session to 
// capture trace messages as usual i.e it will require the
// user to provide WPP trace ENABLED and LOGGER macro. If 
// this is set to 0 by default the IFR trace filter also
// affects which trace messages land in the WPP trace session.
//
#if !defined(ENABLE_WPP_TRACE_FILTERING_WITH_WPP_RECORDER)
#define ENABLE_WPP_TRACE_FILTERING_WITH_WPP_RECORDER 0
#endif

#if !defined(WPP_RECORDER_LEVEL_FLAGS_ARGS)
#define WPP_RECORDER_LEVEL_FLAGS_ARGS(lvl, flags) WPP_CONTROL(WPP_BIT_ ## flags).AutoLogContext, lvl, WPP_BIT_ ## flags
#define WPP_RECORDER_LEVEL_FLAGS_FILTER(lvl,flags) (lvl < TRACE_LEVEL_VERBOSE || WPP_CONTROL(WPP_BIT_ ## flags).AutoLogVerboseEnabled)
#endif

#if !defined(WPP_RECORDER_FLAGS_LEVEL_ARGS)
#define WPP_RECORDER_FLAGS_LEVEL_ARGS(flags, lvl) WPP_RECORDER_LEVEL_FLAGS_ARGS(lvl, flags)
#define WPP_RECORDER_FLAGS_LEVEL_FILTER(flags, lvl) WPP_RECORDER_LEVEL_FLAGS_FILTER(lvl, flags)
#endif

#if !defined(WPP_RECORDER_LEVEL_ARGS)
#define WPP_RECORDER_LEVEL_ARGS(lvl) WPP_CONTROL(WPP_BIT_ ## lvl).AutoLogContext, 0, WPP_BIT_ ## lvl
#define WPP_RECORDER_LEVEL_FILTER(lvl) (WPP_CONTROL(WPP_BIT_ ## lvl).AutoLogVerboseEnabled)
#endif

NTSTATUS
WppAutoLogTrace(
    _In_ PVOID              AutoLogContext,
    _In_ UCHAR              MessageLevel,
    _In_ ULONG              MessageFlags,
    _In_ LPGUID             MessageGuid,
    _In_ USHORT             MessageNumber,
    _In_ ...
    );

#endif // ENABLE_WPP_RECORDER

//
// WPP_INIT_TRACING and WPP_CLEANUP macros are defined differently for kernel
// mode and user mode. In order to support mode-agnostic WDF drivers,
// WPP_INIT_TRACING and WPP_CLEANUP macros for UMDF 2.x user-mode drivers are
// being updated to be same as kernel mode macros. This difference is based
// upon the macro WPP_MACRO_USE_KM_VERSION_FOR_UM.
//

#ifdef WPP_MACRO_USE_KM_VERSION_FOR_UM

VOID WppCleanupUm( VOID );
#define WPP_CLEANUP(DrvObj) WppCleanupUm()
#else
VOID WppCleanupUm( VOID );
#define WPP_CLEANUP() WppCleanupUm()
#endif

#define WppLoadTracingSupport

//
// For getting private callback, implement a function with the signature below.
// To register the function: #define WPP_PRIVATE_ENABLE_CALLBACK MyPrivateCallback
// If multiple control guids are used, be sure to compare Guid to the one of interest.
//

#if defined(WPP_PRIVATE_ENABLE_CALLBACK)

typedef
VOID
(NTAPI *PFN_WPP_PRIVATE_ENABLE_CALLBACK)(
    _In_ LPCGUID Guid,
    _In_ TRACEHANDLE Logger,
    _In_ BOOLEAN Enable,
    _In_ ULONG Flags,
    _In_ UCHAR Level
    );

#endif

//
// For generating CAPTURE_STATE callback (Win7+), implement a function with the signature
// below. To register the function: #define WPP_CAPTURE_STATE_CALLBACK MyCaptureStateCallback
// If multiple control guids are used, be sure to compare Guid to the one of interest.
//

#if defined(WPP_CAPTURE_STATE_CALLBACK)

typedef
VOID
(NTAPI *PFN_WPP_CAPTURE_STATE_CALLBACK)(
    _In_ LPCGUID Guid,
    _In_ TRACEHANDLE Logger,
    _In_ ULONG Flags,
    _In_ UCHAR Level
    );

#endif

#ifdef __cplusplus
} // extern "C"
#endif

#endif  // #ifndef WPP_ALREADY_INCLUDED
