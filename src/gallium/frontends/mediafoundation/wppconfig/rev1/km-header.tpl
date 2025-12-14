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

#include <evntrace.h>
#include <stddef.h>
#include <stdarg.h>
#include <wmistr.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef
LONG
(*PFN_WPPQUERYTRACEINFORMATION) (
    IN  TRACE_INFORMATION_CLASS TraceInformationClass,
    OUT PVOID  TraceInformation,
    IN  ULONG  TraceInformationLength,
    OUT PULONG RequiredLength OPTIONAL,
    IN  PVOID  Buffer OPTIONAL
    );

typedef
LONG
(*PFN_WPPTRACEMESSAGE)(
    IN ULONG64  LoggerHandle,
    IN ULONG   MessageFlags,
    IN LPCGUID MessageGuid,
    IN USHORT  MessageNumber,
    IN ...
    );

typedef enum _WPP_TRACE_API_SUITE {
    WppTraceDisabledSuite,
    WppTraceWin2K,
    WppTraceWinXP,
    WppTraceTraceLH,
    WppTraceServer08,
    WppTraceMaxSuite
} WPP_TRACE_API_SUITE;

_IRQL_requires_same_
typedef
VOID
(NTAPI *PETW_CLASSIC_CALLBACK)(
    _In_ LPCGUID Guid,
    _In_ UCHAR ControlCode,
    _In_ PVOID EnableContext,
    _In_opt_ PVOID CallbackContext
    );

_IRQL_requires_same_
typedef
NTSTATUS
NTKERNELAPI
(FN_ETWREGISTERCLASSICPROVIDER)(
    _In_ LPCGUID ProviderGuid,
    _In_ ULONG Type,
    _In_ PETW_CLASSIC_CALLBACK EnableCallback,
    _In_opt_ PVOID CallbackContext,
    _Out_ PREGHANDLE RegHandle
    );

typedef FN_ETWREGISTERCLASSICPROVIDER *PFN_ETWREGISTERCLASSICPROVIDER;

typedef
BOOLEAN
NTKERNELAPI
(FN_WPPGETVERSION)(
    _Out_opt_ PULONG MajorVersion,
    _Out_opt_ PULONG MinorVersion,
    _Out_opt_ PULONG BuildNumber,
    _Out_opt_ PUNICODE_STRING CSDVersion
    );

typedef FN_WPPGETVERSION *PFN_WPPGETVERSION;

typedef
NTSTATUS
NTKERNELAPI
(FN_ETWUNREGISTER)(
    _In_ REGHANDLE RegHandle
    );

typedef FN_ETWUNREGISTER *PFN_ETWUNREGISTER;

#pragma prefast(suppress:__WARNING_ENCODE_GLOBAL_FUNCTION_POINTER, "this pointer can not be encoded");
__declspec(selectany) PFN_WPPQUERYTRACEINFORMATION   pfnWppQueryTraceInformation = NULL;

#pragma prefast(suppress:__WARNING_ENCODE_GLOBAL_FUNCTION_POINTER, "this pointer can not be encoded");
__declspec(selectany) PFN_WPPTRACEMESSAGE            pfnWppTraceMessage = NULL;

#pragma prefast(suppress:__WARNING_ENCODE_GLOBAL_FUNCTION_POINTER, "this pointer can not be encoded");
__declspec(selectany) PFN_ETWUNREGISTER              pfnEtwUnregister = NULL;

#pragma prefast(suppress:__WARNING_ENCODE_GLOBAL_FUNCTION_POINTER, "this pointer can not be encoded");
__declspec(selectany) PFN_ETWREGISTERCLASSICPROVIDER pfnEtwRegisterClassicProvider = NULL;

#pragma prefast(suppress:__WARNING_ENCODE_GLOBAL_FUNCTION_POINTER, "this pointer can not be encoded");
__declspec(selectany) PFN_WPPGETVERSION              pfnWppGetVersion = NULL;


__declspec(selectany) WPP_TRACE_API_SUITE            WPPTraceSuite = WppTraceDisabledSuite;

#if !defined(_NTRTL_)
#if !defined(_NTHAL_)
// fake RTL_TIME_ZONE_INFORMATION //
typedef int RTL_TIME_ZONE_INFORMATION;
#endif
#define _WMIKM_
#endif

#ifndef WPP_TRACE
#define WPP_TRACE pfnWppTraceMessage
#endif

#if ENABLE_WPP_RECORDER

#define _ENABLE_WPP_RECORDER TRUE

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


#if !defined(WPP_RECORDER_LEVEL_ARGS)
#define WPP_RECORDER_LEVEL_ARGS(lvl) WPP_CONTROL(WPP_BIT_ ## lvl).AutoLogContext, 0, WPP_BIT_ ## lvl
#define WPP_RECORDER_LEVEL_FILTER(lvl) (WPP_CONTROL(WPP_BIT_ ## lvl).AutoLogVerboseEnabled)
#endif

NTSTATUS
WppAutoLogTrace(
    IN PVOID              AutoLogContext,
    IN UCHAR              MessageLevel,
    IN ULONG              MessageFlags,
    IN LPGUID             MessageGuid,
    IN USHORT             MessageNumber,
    IN ...
    );

#else
#define _ENABLE_WPP_RECORDER FALSE
#endif

VOID
WppLoadTracingSupport(
    VOID
    );

NTSTATUS
WppTraceCallback(
    _In_ UCHAR MinorFunction,
    _In_opt_ PVOID DataPath,
    _In_ ULONG BufferLength,
    _Inout_updates_bytes_(BufferLength) PVOID Buffer,
    _Inout_ PVOID Context,
    _Out_ PULONG Size
    );

#if !defined(WPP_TRACE_CONTROL_NULL_GUID)
DEFINE_GUID(WPP_TRACE_CONTROL_NULL_GUID, 0x00000000L, 0x0000, 0x0000, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00);
#endif

#define WPP_TRACE_CONTROL(Function,Buffer,BufferSize,ReturnSize) WppTraceCallback(Function,NULL,BufferSize,Buffer,&WPP_CB[0],&ReturnSize);

__inline ULONG64 WppQueryLogger(_In_opt_ PCWSTR LoggerName)
{
    if (WppTraceWinXP != WPPTraceSuite) {
        return (ULONG64)0;
    }

    ULONG ReturnLength;
    LONG Status;
    ULONG64 TraceHandle;
    UNICODE_STRING Buffer;

    RtlInitUnicodeString(&Buffer, LoggerName ? LoggerName : L"stdout");

    Status = pfnWppQueryTraceInformation(TraceHandleByNameClass,
                                         (PVOID)&TraceHandle,
                                         sizeof(TraceHandle),
                                         &ReturnLength,
                                         (PVOID)&Buffer
                                         );
    if (Status != STATUS_SUCCESS) {
        return (ULONG64)0;
    }

    return TraceHandle;
}

typedef LONG (*WMIENTRY_NEW)(
    _In_ UCHAR MinorFunction,
    _In_opt_ PVOID DataPath,
    _In_ ULONG BufferLength,
    _Inout_updates_bytes_(BufferLength) PVOID Buffer,
    _In_ PVOID Context,
    _Out_ PULONG Size
    );

typedef struct _WPP_TRACE_CONTROL_BLOCK
{
    WMIENTRY_NEW                        Callback;
    LPCGUID                             ControlGuid;
    struct _WPP_TRACE_CONTROL_BLOCK    *Next;
    __int64                             Logger;
    PUNICODE_STRING                     RegistryPath;
    UCHAR                               FlagsLen;
    UCHAR                               Level;
    USHORT                              Reserved;
    ULONG                               Flags[1];
    ULONG                               ReservedFlags;
    REGHANDLE                           RegHandle;
#if ENABLE_WPP_RECORDER
    PVOID                               AutoLogContext;
    USHORT                              AutoLogVerboseEnabled;
    USHORT                              AutoLogAttachToMiniDump;
#endif
} WPP_TRACE_CONTROL_BLOCK, *PWPP_TRACE_CONTROL_BLOCK;

VOID WppCleanupKm(_When_(_ENABLE_WPP_RECORDER, _In_) _When_(!_ENABLE_WPP_RECORDER, _In_opt_) PDRIVER_OBJECT DriverObject);

#define WPP_CLEANUP(DriverObject) WppCleanupKm((PDRIVER_OBJECT)DriverObject)

#define WPP_IsValidSid RtlValidSid
#define WPP_GetLengthSid RtlLengthSid

//
// Callback routine to be defined by the driver, which will be called from WPP callback
// WPP will pass current valued of : GUID, Logger, Enable, Flags, and Level
//
// To activate driver must define WPP_PRIVATE_ENABLE_CALLBACK in their code, sample below
// #define WPP_PRIVATE_ENABLE_CALLBACK MyPrivateCallback;
//
typedef
VOID
(*PFN_WPP_PRIVATE_ENABLE_CALLBACK)(
    _In_ LPCGUID Guid,
    _In_ __int64 Logger,
    _In_ BOOLEAN Enable,
    _In_ ULONG Flags,
    _In_ UCHAR Level);

#ifdef __cplusplus
} // extern "C"
#endif

#endif  // #ifndef WPP_ALREADY_INCLUDED
