`**********************************************************************`
`* This is an include template file for the tracewpp preprocessor.    *`
`*                                                                    *`
`*    Copyright (c) Microsoft Corporation. All rights reserved.       *`
`**********************************************************************`
// template `TemplateFile`

//
//     Defines a set of functions that simplifies
//     kernel mode registration for tracing
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

WPPINIT_EXPORT
NTSTATUS
WppTraceCallback(
    _In_ UCHAR MinorFunction,
    _In_opt_ PVOID DataPath,
    _In_ ULONG BufferLength,
    _Inout_updates_bytes_(BufferLength) PVOID Buffer,
    _Inout_ PVOID Context,
    _Out_ PULONG Size
    );

WPPINIT_EXPORT
VOID
WppInitKm(
    _In_opt_ PVOID DriverObject,
    _In_ PVOID  InitInfo
    );

#ifdef ALLOC_PRAGMA
    #pragma alloc_text( PAGE, WppTraceCallback)
    #pragma alloc_text( PAGE, WppInitKm)
    #pragma alloc_text( PAGE, WppCleanupKm)
#endif // ALLOC_PRAGMA

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

#define WPP_NEXT(Name) ((WPP_TRACE_CONTROL_BLOCK*) \
    (WPP_XGLUE(WPP_CTL_, WPP_EVAL(Name)) + 1 == WPP_LAST_CTL ? 0:WPP_MAIN_CB + WPP_XGLUE(WPP_CTL_, WPP_EVAL(Name)) + 1))

WPP_CB_TYPE WPP_MAIN_CB[WPP_LAST_CTL];

__inline void WPP_INIT_CONTROL_ARRAY(WPP_CB_TYPE* Arr) {
#define WPP_DEFINE_CONTROL_GUID(Name,Guid,Bits)                                         \
   Arr->Control.Callback = NULL;                                                        \
   Arr->Control.ControlGuid = WPP_XGLUE4(&WPP_, ThisDir, _CTLGUID_, WPP_EVAL(Name));    \
   Arr->Control.Next = WPP_NEXT(WPP_EVAL(Name));                                        \
   Arr->Control.RegistryPath= NULL;                                                     \
   Arr->Control.FlagsLen = WPP_FLAG_LEN;                                                \
   Arr->Control.Level = 0;                                                              \
   Arr->Control.Reserved = 0;                                                           \
   Arr->Control.Flags[0] = 0;                                                           \
   ++Arr;
#define WPP_DEFINE_BIT(BitName) L" " L ## #BitName
WPP_CONTROL_GUIDS
#undef WPP_DEFINE_BIT
#undef WPP_DEFINE_CONTROL_GUID
}

#undef WPP_INIT_STATIC_DATA
#define WPP_INIT_STATIC_DATA WPP_INIT_CONTROL_ARRAY(WPP_MAIN_CB)

// define WPP_INIT_TRACING.  For performance reasons turn off during
// static analysis compilation with Static Driver Verifier (SDV).
#ifndef _SDV_
#define WPP_INIT_TRACING(DriverObject, RegPath, InitInfo)                   \
    {                                                                       \
      WppDebug(0,("WPP_INIT_TRACING: &WPP_CB[0] %p\n", &WPP_MAIN_CB[0]));   \
      WPP_INIT_STATIC_DATA;                                                 \
      ( WPP_CONTROL_ANNOTATION(),                                           \
        WPP_MAIN_CB[0].Control.RegistryPath = NULL,                         \
        UNREFERENCED_PARAMETER(RegPath),                                    \
        WppInitKm( DriverObject, InitInfo )                                 \
      );                                                                    \
    }
#else
#define WPP_INIT_TRACING(DriverObject, RegPath, InitInfo)
#endif

#define WMIREG_FLAG_CALLBACK  0x80000000 // not exposed in DDK

#ifndef WMIREG_FLAG_TRACE_PROVIDER
#define WMIREG_FLAG_TRACE_PROVIDER          0x00010000
#endif

__inline int WppIsEqualGuid(_In_ const GUID* g1, _In_ const GUID* g2)
{
    const ULONG* p1 = (const ULONG*)g1;
    const ULONG* p2 = (const ULONG*)g2;
    return p1[0] == p2[0] && p1[1] == p2[1] && p1[2] == p2[2] && p1[3] == p2[3];
}

#define WPP_MAX_COUNT_REGISTRATION_GUID 63

WPPINIT_EXPORT
NTSTATUS
WppTraceCallback(
    _In_ UCHAR MinorFunction,
    _In_opt_ PVOID DataPath,
    _In_ ULONG BufferLength,
    _Inout_updates_bytes_(BufferLength) PVOID Buffer,
    _Inout_ PVOID Context,
    _Out_ PULONG Size
    )
/*++

Routine Description:

    This function is the callback WMI calls when we register and when our
    events are enabled or disabled.

Arguments:

    MinorFunction - specifies the type of callback (register, event enable/disable)

    DataPath - varies depending on the ActionCode

    BufferLength - size of the Buffer parameter

    Buffer - in/out buffer where we read from or write to depending on the type
        of callback

    Context - the pointer private struct WPP_TRACE_CONTROL_BLOCK

    Size - output parameter to receive the amount of data written into Buffer

Return Value:

    NTSTATUS code indicating success/failure

Comments:

    if return value is STATUS_BUFFER_TOO_SMALL and BufferLength >= 4,
    then first ulong of buffer contains required size


--*/

{
    PWPP_TRACE_CONTROL_BLOCK    cntl;
    NTSTATUS                    Status = STATUS_SUCCESS;

    UNREFERENCED_PARAMETER(DataPath);

    WppDebug(0,("WppTraceCallBack 0x%08X %p\n", MinorFunction, Context));

    *Size = 0;

    switch(MinorFunction)
    {
        case IRP_MN_REGINFO:
        {
            PWMIREGINFOW     WmiRegInfo;
            PCUNICODE_STRING RegPath;
            PWCHAR           StringPtr;
            ULONG            RegistryPathOffset;
            ULONG            BufferNeeded;
            ULONG            GuidCount = 0;

            //
            // Initialize locals
            //

            cntl = (PWPP_TRACE_CONTROL_BLOCK)Context;
            WmiRegInfo = (PWMIREGINFO)Buffer;

            RegPath = cntl->RegistryPath;

            //
            // Count the number of guid to be identified.
            //
            while(cntl) { GuidCount++; cntl = cntl->Next; }

            if (GuidCount > WPP_MAX_COUNT_REGISTRATION_GUID){
                Status = STATUS_INVALID_PARAMETER;
                break;
            }

            WppDebug(0,("WppTraceCallBack: GUID count %d\n", GuidCount));

            //
            // Calculate buffer size need to hold all info.
            // Calculate offset to where RegistryPath parm will be copied.
            //

            if (RegPath == NULL)
            {

                RegistryPathOffset = 0;

                BufferNeeded = FIELD_OFFSET(WMIREGINFOW, WmiRegGuid) +
                               GuidCount * sizeof(WMIREGGUIDW);

            } else {

                RegistryPathOffset = FIELD_OFFSET(WMIREGINFOW, WmiRegGuid) +
                                     GuidCount * sizeof(WMIREGGUIDW);

                BufferNeeded = RegistryPathOffset +
                               RegPath->Length + sizeof(USHORT);
            }

            //
            // If the provided buffer is large enough, then fill with info.
            //

            if (BufferNeeded <= BufferLength)
            {
                ULONG  i;

                StorMemSet(Buffer, 0, BufferLength);

                //
                // Fill in the WMIREGINFO
                //

                WmiRegInfo->BufferSize   = BufferNeeded;
                WmiRegInfo->RegistryPath = RegistryPathOffset;
                WmiRegInfo->GuidCount    = GuidCount;

                if (RegPath != NULL) {
                    StringPtr    = (PWCHAR)((PUCHAR)Buffer + RegistryPathOffset);
                    *StringPtr++ = RegPath->Length;

                    StorMoveMemory(StringPtr, RegPath->Buffer, RegPath->Length);
                }

                //
                // Fill in the WMIREGGUID
                //

                cntl = (PWPP_TRACE_CONTROL_BLOCK) Context;

                for (i=0; i<GuidCount; i++) {

                    WmiRegInfo->WmiRegGuid[i].Guid  = *cntl->ControlGuid;
                    WmiRegInfo->WmiRegGuid[i].Flags = WMIREG_FLAG_TRACE_CONTROL_GUID |
                                                      WMIREG_FLAG_TRACED_GUID;
                    cntl->Level = 0;
                    cntl->Flags[0] = 0;
                    WppDebug(0,("Control GUID::%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x\n",
                                cntl->ControlGuid->Data1,
                                cntl->ControlGuid->Data2,
                                cntl->ControlGuid->Data3,
                                cntl->ControlGuid->Data4[0],
                                cntl->ControlGuid->Data4[1],
                                cntl->ControlGuid->Data4[2],
                                cntl->ControlGuid->Data4[3],
                                cntl->ControlGuid->Data4[4],
                                cntl->ControlGuid->Data4[5],
                                cntl->ControlGuid->Data4[6],
                                cntl->ControlGuid->Data4[7]
                        ));

                    cntl = cntl->Next;
                }

                Status = STATUS_SUCCESS;
                *Size  = BufferNeeded;

            } else {
                Status = STATUS_BUFFER_TOO_SMALL;

                if (BufferLength >= sizeof(ULONG)) {
                    *((PULONG)Buffer) = BufferNeeded;
                    *Size = sizeof(ULONG);
                }
            }

#ifdef WPP_GLOBALLOGGER
            // Check if Global logger is active

            cntl = (PWPP_TRACE_CONTROL_BLOCK) Context;
            while(cntl) {
                StorWppInitGlobalLogger(
                                    cntl->ControlGuid,
                                    (PTRACEHANDLE)&cntl->Logger,
                                    &cntl->Flags[0],
                                    &cntl->Level);
                cntl = cntl->Next;
            }
#endif  //#ifdef WPP_GLOBALLOGGER

            break;
        }

        case IRP_MN_ENABLE_EVENTS:
        case IRP_MN_DISABLE_EVENTS:
        {
            PWNODE_HEADER             Wnode;
            ULONG                     Level;
            ULONG                     ReturnLength;
            ULONG                     index;

            if (Context == NULL ) {
                Status = STATUS_WMI_GUID_NOT_FOUND;
                break;
            }

            if (BufferLength < sizeof(WNODE_HEADER)) {
                Status = STATUS_INVALID_PARAMETER;
                break;
            }

            //
            // Initialize locals
            //
            Wnode = (PWNODE_HEADER)Buffer;

            //
            // Traverse this ProjectControlBlock's ControlBlock list and
            // find the "cntl" ControlBlock which matches the Wnode GUID.
            //
            cntl  = (PWPP_TRACE_CONTROL_BLOCK) Context;
            index = 0;
            while(cntl) {
                if (WppIsEqualGuid(cntl->ControlGuid, &Wnode->Guid )) {
                    break;
                }
                index++;
                cntl = cntl->Next;
            }

            if (cntl == NULL) {
                Status = STATUS_WMI_GUID_NOT_FOUND;
                break;
            }

            //
            // Do the requested event action
            //
            Status = STATUS_SUCCESS;

            if (MinorFunction == IRP_MN_DISABLE_EVENTS) {

                WppDebug(0,("WppTraceCallBack: DISABLE_EVENTS\n"));

                cntl->Level    = 0;
                cntl->Flags[0] = 0;
                cntl->Logger   = 0;

            } else {

                TRACEHANDLE  lh;

                lh = (TRACEHANDLE)( Wnode->HistoricalContext );
                cntl->Logger = lh;

                Status = StorWmiQueryTraceInformation( TraceEnableLevelClass,
                                                      &Level,
                                                      sizeof(Level),
                                                      &ReturnLength,
                                                      (PVOID)Wnode);

                if (Status == STATUS_SUCCESS) {
                    cntl->Level = (UCHAR)Level;
                }

                Status = StorWmiQueryTraceInformation( TraceEnableFlagsClass,
                                                      &cntl->Flags[0],
                                                      sizeof(cntl->Flags[0]),
                                                      &ReturnLength,
                                                      (PVOID) Wnode );

                WppDebug(0,("WppTraceCallBack: ENABLE_EVENTS "
                            "LoggerId %d, Flags 0x%08X, Level 0x%02X\n",
                            (USHORT) cntl->Logger,
                            cntl->Flags[0],
                            cntl->Level));

            }

#ifdef WPP_PRIVATE_ENABLE_CALLBACK
            //
            // Notify changes to flags, level for GUID
            //
                WPP_PRIVATE_ENABLE_CALLBACK( cntl->ControlGuid,
                                             cntl->Logger,
                                             (MinorFunction != IRP_MN_DISABLE_EVENTS) ? TRUE:FALSE,
                                             cntl->Flags[0],
                                             cntl->Level );
#endif

            break;
        }

        case IRP_MN_ENABLE_COLLECTION:
        case IRP_MN_DISABLE_COLLECTION:
        {
            Status = STATUS_SUCCESS;
            break;
        }

        case IRP_MN_QUERY_ALL_DATA:
        case IRP_MN_QUERY_SINGLE_INSTANCE:
        case IRP_MN_CHANGE_SINGLE_INSTANCE:
        case IRP_MN_CHANGE_SINGLE_ITEM:
        case IRP_MN_EXECUTE_METHOD:
        {
            Status = STATUS_INVALID_DEVICE_REQUEST;
            break;
        }

        default:
        {
            Status = STATUS_INVALID_DEVICE_REQUEST;
            break;
        }

    }
    return(Status);
}

#pragma warning(push)
#pragma warning(disable:4068)
WPPINIT_EXPORT
VOID
WppInitKm(
    _In_opt_ PVOID DriverObject,
    _In_ PVOID  InitInfo
    )

/*++

Routine Description:

    This function registers a driver with ETW as a provider of trace
    events from the defined GUIDs.

Arguments:

    DriverObject - Pointer to a driver object. This is required for WppRecorder
                   and is optional otherwise (not used unless it's for
                   WppRecorder).

    InitInfo - Pointer to STORAGE_TRACE_INIT_INFO.

Remarks:

   This function is called by the
   WPP_INIT_TRACING(DriverObject, RegPath, InitInfo) macro.

--*/

{
    C_ASSERT(WPP_MAX_FLAG_LEN_CHECK);

    NTSTATUS Status;
    PWPP_TRACE_CONTROL_BLOCK WppReg = NULL;

    UNREFERENCED_PARAMETER(DriverObject);

    if (WPP_CB != WPP_MAIN_CB) {

        WPP_CB = WPP_MAIN_CB;

    } else {
      //
      // WPP_INIT_TRACING already called
      //
      WppDebug(0,("Warning : WPP_INIT_TRACING already called, ignoring this one"));
      return;
    }

    WppReg = &WPP_CB[0].Control;

    WppDebug(0,("WPP Init.\n"));


    if (StorInitTracing(InitInfo) == STATUS_SUCCESS) {

        WppReg -> Callback = WppTraceCallback;

        Status = StorIoWMIRegistrationControl(
                                    WppReg,
                                    WMIREG_ACTION_REGISTER  |
                                    WMIREG_FLAG_CALLBACK    |
                                    WMIREG_FLAG_TRACE_PROVIDER
                                    );

        if (!NT_SUCCESS(Status)) {
            WppDebug(0,("StorIoWMIRegistrationControl Status = %08X\n",Status));
        }

    }

}


WPPINIT_EXPORT
VOID
WppCleanupKm(
    _In_ PVOID TraceContext
    )

/*++

Routine Description:

    This function deregisters a driver from ETW as provider of trace
    events.

Arguments:

    TraceContext - The STORAGE_TRACE_INIT_INFO.TraceContext value.

Remarks:

    This function is called by the WPP_CLEANUP(DriverObject, TraceContext) macro.

--*/

{
    StorCleanupTracing(TraceContext);

    if (WPP_CB == (WPP_CB_TYPE*)&WPP_CB){
        //
        // WPP_INIT_TRACING macro has not been called
        //
        WppDebug(0,("Warning : WPP_CLEANUP already called, or called with out WPP_INIT_TRACING first"));
        return;
    }

    PWPP_TRACE_CONTROL_BLOCK WppReg = &WPP_CB[0].Control;

    StorIoWMIRegistrationControl(WppReg,
                                 WMIREG_ACTION_DEREGISTER |
                                 WMIREG_FLAG_CALLBACK );

    WPP_CB = (WPP_CB_TYPE*)&WPP_CB;
}

#pragma warning(pop)

#define WPP_SYSTEMCONTROL(PDO)
#define WPP_SYSTEMCONTROL2(PDO, offset)

#ifdef __cplusplus
} // extern "C"
#endif
