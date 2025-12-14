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

#include <stortrce.h>
#include <stddef.h>
#include <stdarg.h>
#include <wmistr.h>

#ifdef __cplusplus
extern "C" {
#endif

#if !defined(_NTRTL_)
#if !defined(_NTHAL_)
// fake RTL_TIME_ZONE_INFORMATION //
typedef int RTL_TIME_ZONE_INFORMATION;
#endif
#define _WMIKM_
#endif

#ifndef WPP_TRACE
#define WPP_TRACE StorWmiTraceMessage
#endif

#if ENABLE_WPP_RECORDER
#error ENABLE_WPP_RECORDER not supported by km-StorDefault.tpl
#endif

///////////////////////////////////////////////////////////////////////////////
//
// B O R R O W E D  D E F I N I T I O N S
//
///////////////////////////////////////////////////////////////////////////////

#if !defined(_NTDEF_)
typedef struct _UNICODE_STRING {
    USHORT Length;
    USHORT MaximumLength;
    PWSTR  Buffer;
} UNICODE_STRING;
typedef UNICODE_STRING *PUNICODE_STRING;
typedef const UNICODE_STRING *PCUNICODE_STRING;
#endif

#define TRACE_MESSAGE_SEQUENCE                1      // Message should include a sequence number
#define TRACE_MESSAGE_GUID                    2      // Message includes a GUID
#define TRACE_MESSAGE_COMPONENTID             4      // Message has no GUID, Component ID instead
#define TRACE_MESSAGE_TIMESTAMP               8      // Message includes a timestamp
#define TRACE_MESSAGE_PERFORMANCE_TIMESTAMP   16     // *Obsolete* Clock type is controlled by
                                                     // the logger
#define TRACE_MESSAGE_SYSTEMINFO              32     // Message includes system information TID,PID
#define TRACE_MESSAGE_FLAG_MASK               0xFFFF // Only the lower 16 bits of flags are
                                                     // placed in the message those above 16
                                                     // bits are reserved for local processing
#ifndef TRACE_MESSAGE_MAXIMUM_SIZE
#define TRACE_MESSAGE_MAXIMUM_SIZE  8*1024           // the maximum size allowed for a single trace
#endif                                               // message

#ifndef NTSTATUS
typedef LONG NTSTATUS;
#endif

#ifndef NT_SUCCESS
#define NT_SUCCESS(Status) ((NTSTATUS)(Status) >= 0)
#endif

#define STATUS_SUCCESS                   ((NTSTATUS)0x00000000L) // ntsubauth
#define STATUS_WMI_GUID_NOT_FOUND        ((NTSTATUS)0xC0000295L)
#define STATUS_BUFFER_TOO_SMALL          ((NTSTATUS)0xC0000023L)
#define STATUS_INVALID_PARAMETER         ((NTSTATUS)0xC000000DL)
#define STATUS_INVALID_DEVICE_REQUEST    ((NTSTATUS)0xC0000010L)

typedef ULONG64 TRACEHANDLE, *PTRACEHANDLE;

#ifndef TRACE_INFORMATION_CLASS_DEFINE
typedef enum _TRACE_INFORMATION_CLASS {
    TraceIdClass,
    TraceHandleClass,
    TraceEnableFlagsClass,
    TraceEnableLevelClass,
    GlobalLoggerHandleClass,
    EventLoggerHandleClass,
    AllLoggerHandlesClass,
    TraceHandleByNameClass
} TRACE_INFORMATION_CLASS;
#endif

//
// Action code for IoWMIRegistrationControl api
//

#define WMIREG_ACTION_REGISTER      1
#define WMIREG_ACTION_DEREGISTER    2
#define WMIREG_ACTION_REREGISTER    3
#define WMIREG_ACTION_UPDATE_GUIDS  4
#define WMIREG_ACTION_BLOCK_IRPS    5

///////////////////////////////////////////////////////////////////////////////

__inline ULONG64 WppQueryLogger(_In_opt_ PCWSTR LoggerName)
{
    ULONG ReturnLength;
    LONG Status;
    ULONG64 TraceHandle;
    UNICODE_STRING Buffer;

    StorRtlInitUnicodeString(&Buffer, LoggerName ? LoggerName : L"stdout");

    Status = StorWmiQueryTraceInformation(TraceHandleByNameClass,
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
} WPP_TRACE_CONTROL_BLOCK, *PWPP_TRACE_CONTROL_BLOCK;

VOID WppCleanupKm(_In_ PVOID TraceContext);

#define WPP_CLEANUP(DriverObject, TraceContext) WppCleanupKm(TraceContext)

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
