`**********************************************************************`
`* This is an include template file for the tracewpp preprocessor.    *`
`*                                                                    *`
`*    Copyright (c) Microsoft Corporation. All rights reserved.       *`
`**********************************************************************`
// template `TemplateFile`

// This template expects:
//      WPP_THIS_FILE defined (see header.tpl)
//      WPP_LOGGER_ARG  defined
//      WPP_GET_LOGGER  defined
//      WPP_ENABLED() defined

#ifdef __cplusplus
extern "C" {
#endif

#ifdef WPP_ALREADY_INCLUDED

#undef WPP_LOCAL_TraceGuids
#undef WPP_INVOKE_WPP_DEBUG

#else // WPP_ALREADY_INCLUDED

#ifndef NO_CHECK_FOR_NULL_STRING
#ifndef WPP_CHECK_FOR_NULL_STRING
#define WPP_CHECK_FOR_NULL_STRING
#endif
#endif // NO_CHECK_FOR_NULL_STRING

#define WPP_FLATTEN(...) __VA_ARGS__
#define WPP_GLUE5(a, b, c, d, e)  a ## b ## c ## d ## e
#define WPP_XGLUE5(a, b, c, d, e)  WPP_GLUE5(a, b, c, d, e)
#define WPP_(Id) WPP_XGLUE5(WPP_, Id, _, WPP_THIS_FILE, __LINE__)

#ifndef WPP_INLINE
#define WPP_INLINE DECLSPEC_NOINLINE __inline
#endif

#ifndef WPP_FORCEINLINE
#define WPP_FORCEINLINE __forceinline
#endif

#endif // WPP_ALREADY_INCLUDED

#ifdef WPP_NO_ANNOTATIONS

#define WPP_ANNOTATE(x)

#else // WPP_NO_ANNOTATIONS

`FORALL Msg IN Messages WHERE MsgIsPrivate`
#ifdef WPP_PUBLIC_`Msg.FlagValue`
#define WPP_PUBLIC_ANNOT_`Msg.Name`
#endif
`ENDFOR`
`FORALL Msg IN Messages WHERE MsgIsPublic`
#define WPP_PUBLIC_ANNOT_`Msg.Name`
`ENDFOR`
#ifdef WPP_EMIT_FUNC_NAME
#define WPP_FUNC_NAME L" FUNC=" _WPPW(__FUNCTION__)
#else // WPP_EMIT_FUNC_NAME
#define WPP_FUNC_NAME
#endif // WPP_EMIT_FUNC_NAME
`FORALL Msg IN Messages`

#ifdef WPP_USER_MSG_GUID
# define WPP_ANNOTATE_`Msg.Name`_FINAL(P, File, Name, ...) __annotation( \
    L ## P, \
    WPP_GUID_WTEXT WPP_USER_MSG_GUID L"`CurrentDir` // SRC=" _WPPW(File) \
    L" MJ=`ENV MAJORCOMP` MN=`ENV MINORCOMP`", \
    L"#typev " _WPPW(Name) \
    L" `Msg.MsgNo` \"`Msg.Text`\" // `Msg.Indent` `Msg.GooPairs`" \
    WPP_FUNC_NAME, \
    __VA_ARGS__)
#else // WPP_USER_MSG_GUID
# define WPP_ANNOTATE_`Msg.Name`_FINAL(P, File, Name, ...) __annotation( \
    L ## P, \
    L"`Msg.Guid` `CurrentDir` // SRC=" _WPPW(File) \
    L" MJ=`ENV MAJORCOMP` MN=`ENV MINORCOMP`", \
    L"#typev " _WPPW(Name) \
    L" `Msg.MsgNo` \"`Msg.Text`\" // `Msg.Indent` `Msg.GooPairs`" \
    WPP_FUNC_NAME, \
    __VA_ARGS__)
#endif // WPP_USER_MSG_GUID

#ifdef WPP_PUBLIC_ANNOT_`Msg.Name`
# define WPP_ANNOTATE_`Msg.Name` WPP_ANNOTATE_`Msg.Name`_FINAL( \
    "TMF:", \
    "Unknown_cxx00", \
    "Unknown_cxx00", \
    L"{"`FORALL Arg IN Msg.Arguments`, \
    L"Arg, `Arg.MofType` -- `Arg.No`" `ENDFOR Arg`, \
    L"}", \
    L"PUBLIC_TMF:")
# ifndef WPP_PUBLIC_TMC
#  define WPP_PUBLIC_TMC // Adds "PUBLIC_TMF:" to the control guid annotation
# endif
#else // WPP_PUBLIC_ANNOT_`Msg.Name`
# define WPP_ANNOTATE_`Msg.Name` WPP_ANNOTATE_`Msg.Name`_FINAL( \
    "TMF:", \
    "`SourceFile.Name`", \
    "`Msg.Name`", \
    L"{"`FORALL Arg IN Msg.Arguments`, \
    L"`Arg.Name`, `Arg.MofType` -- `Arg.No`" `ENDFOR Arg`, \
    L"}")
#endif // WPP_PUBLIC_ANNOT_`Msg.Name`
`ENDFOR`

# define WPP_ANNOTATE(x) WPP_ANNOTATE_ ## x,

#endif // WPP_NO_ANNOTATIONS
`IF TraceGuids !Empty`

#ifdef WPP_USER_MSG_GUID

#define WPP_LOCAL_MSG_VAR(Guid) WPP_XGLUE3(WPP_, WPP_GUID_NORM Guid, _Traceguids)

#define WPP_LOCAL_MSG_GUID(Guid) \
extern const __declspec(selectany) GUID WPP_LOCAL_MSG_VAR(Guid)[] = { WPP_GUID_STRUCT Guid }

WPP_LOCAL_MSG_GUID(WPP_USER_MSG_GUID);
#define WPP_LOCAL_TraceGuids WPP_LOCAL_MSG_VAR(WPP_USER_MSG_GUID)

#else // WPP_USER_MSG_GUID

#define WPP_LOCAL_TraceGuids WPP_`FORALL Guid IN TraceGuids``Guid.Normalized`_`ENDFOR`Traceguids
extern const __declspec(selectany) GUID WPP_LOCAL_TraceGuids[] = {`FORALL Guid IN TraceGuids` `Guid.Struct`, `ENDFOR`};

#endif // WPP_USER_MSG_GUID
`ENDIF TraceGuids !Empty`

#ifndef WPP_ALREADY_INCLUDED

#ifndef WPP_TRACE_OPTIONS
enum { WPP_TRACE_OPTIONS =
    TRACE_MESSAGE_SEQUENCE   |
    TRACE_MESSAGE_GUID       |
    TRACE_MESSAGE_SYSTEMINFO |
    TRACE_MESSAGE_TIMESTAMP };
#endif // WPP_TRACE_OPTIONS

#ifndef WPP_LOGPAIR_SEPARATOR
# define WPP_LOGPAIR_SEPARATOR ,
#endif
#ifndef WPP_LOGPAIR_SIZET
# define WPP_LOGPAIR_SIZET SIZE_T
#endif
#ifndef WPP_LOGPAIR
# define WPP_LOGPAIR(_Size, _Addr)     (_Addr),((WPP_LOGPAIR_SIZET)(_Size))WPP_LOGPAIR_SEPARATOR
#endif

#define WPP_LOGTYPEVAL(_Type, _Value) WPP_LOGPAIR(sizeof(_Type), &(_Value))
#define WPP_LOGTYPEPTR(_Value)        WPP_LOGPAIR(sizeof(*(_Value)), (_Value))

// Marshalling macros.

#ifndef WPP_LOGASTR
# ifdef WPP_CHECK_FOR_NULL_STRING
#  define WPP_LOGASTR(_value)  WPP_LOGPAIR( \
    (_value) ? strlen(_value) + 1 : 5, \
    (_value) ?       (_value)     : "NULL" )
# else // WPP_CHECK_FOR_NULL_STRING
#  define WPP_LOGASTR(_value)  WPP_LOGPAIR( \
    strlen(_value) + 1, \
    _value )
# endif // WPP_CHECK_FOR_NULL_STRING
#endif // WPP_LOGASTR

#ifndef WPP_LOGWSTR
# ifdef WPP_CHECK_FOR_NULL_STRING
#  define WPP_LOGWSTR(_value)  WPP_LOGPAIR( \
    ((_value) ? wcslen(_value) + 1 : 5) * sizeof(WCHAR), \
     (_value) ?       (_value)     : L"NULL" )
# else // WPP_CHECK_FOR_NULL_STRING
#  define WPP_LOGWSTR(_value)  WPP_LOGPAIR( \
    (wcslen(_value) + 1) * sizeof(WCHAR), \
    _value )
# endif // WPP_CHECK_FOR_NULL_STRING
#endif // WPP_LOGWSTR

#ifndef WPP_LOGPGUID
# define WPP_LOGPGUID(_value) WPP_LOGPAIR( sizeof(GUID), (_value) )
#endif // WPP_LOGPGUID

#ifndef WPP_LOGPSID
# ifdef WPP_CHECK_FOR_NULL_STRING
# define WPP_LOGPSID(_value)  WPP_LOGPAIR( \
    (_value) && WPP_IsValidSid(_value) ? WPP_GetLengthSid(_value) : 5, \
    (_value) && WPP_IsValidSid(_value) ? (_value) : (void const*)"NULL")
# else // WPP_CHECK_FOR_NULL_STRING
# define WPP_LOGPSID(_value)  WPP_LOGPAIR( \
    WPP_GetLengthSid(_value), \
    (_value) )
#endif // WPP_CHECK_FOR_NULL_STRING
#endif // WPP_LOGPSID

#ifndef WPP_LOGCSTR
# define WPP_LOGCSTR(_x) \
    WPP_LOGPAIR( sizeof(USHORT),      &(_x).Length ) \
    WPP_LOGPAIR( (USHORT)(_x).Length, (USHORT)(_x).Length ? (_x).Buffer : "" )
#endif // WPP_LOGCSTR

#ifndef WPP_LOGUSTR
# define WPP_LOGUSTR(_x) \
    WPP_LOGPAIR( sizeof(USHORT),      &(_x).Length ) \
    WPP_LOGPAIR( (USHORT)(_x).Length, (USHORT)(_x).Length ? (_x).Buffer : L"" )
#endif // WPP_LOGUSTR

#ifndef WPP_LOGPUSTR
#ifdef WPP_CHECK_FOR_NULL_STRING
# define WPP_LOGPUSTR(_x) \
    WPP_LOGPAIR( \
        sizeof(USHORT), \
        (_x) ? &(_x)->Length : (void const*)L"\x08" ) \
    WPP_LOGPAIR( \
        (_x)                         ? (USHORT)(_x)->Length : 0x08, \
        (_x) && (USHORT)(_x)->Length ? (_x)->Buffer         : L"NULL" )
#else // WPP_CHECK_FOR_NULL_STRING
# define WPP_LOGPUSTR(_x) WPP_LOGUSTR(*(_x))
#endif // WPP_CHECK_FOR_NULL_STRING
#endif // WPP_LOGPUSTR

#ifndef WPP_LOGPCSTR
#ifdef WPP_CHECK_FOR_NULL_STRING
# define WPP_LOGPCSTR(_x) \
    WPP_LOGPAIR( \
        sizeof(USHORT), \
        (_x) ? &(_x)->Length : (void const*)L"\x04" ) \
    WPP_LOGPAIR( \
        (_x)                         ? (USHORT)(_x)->Length : 0x04, \
        (_x) && (USHORT)(_x)->Length ? (_x)->Buffer         : "NULL" )
#else // WPP_CHECK_FOR_NULL_STRING
# define WPP_LOGPCSTR(_x) WPP_LOGCSTR(*(_x))
#endif // WPP_CHECK_FOR_NULL_STRING
#endif // WPP_LOGPCSTR

#ifdef __cplusplus

#ifndef WPP_POINTER_TO_USHORT
struct WppPointerToUshort
{
    USHORT m_val;
    WPP_FORCEINLINE explicit WppPointerToUshort(USHORT val) : m_val(val) {}
    WPP_FORCEINLINE USHORT const* get() const { return &m_val; }
};
#define WPP_POINTER_TO_USHORT(val) (WppPointerToUshort((val)).get())
#endif // WPP_POINTER_TO_USHORT

#ifndef WPP_LOGCPPSTR
#define WPP_LOGCPPSTR(_value) \
    WPP_LOGPAIR( \
        sizeof(USHORT), \
        WPP_POINTER_TO_USHORT((USHORT)((_value).size()*sizeof(*(_value).c_str()))) ) \
    WPP_LOGPAIR( \
        (USHORT)((_value).size()*sizeof(*(_value).c_str())), \
        (_value).c_str() )
#endif // WPP_LOGCPPSTR

#ifndef WPP_LOGCPPVEC
#define WPP_LOGCPPVEC(_value) \
    WPP_LOGPAIR( \
        sizeof(USHORT), \
        WPP_POINTER_TO_USHORT((USHORT)((_value).size()*sizeof(*(_value).data()))) ) \
    WPP_LOGPAIR( \
        (USHORT)((_value).size()*sizeof(*(_value).data())), \
        (_value).data() + ((_value).data() == NULL) )
#endif // WPP_LOGCPPVEC

#endif // __cplusplus

#ifndef WPP_BINARY_def
# define WPP_BINARY_def
typedef struct tagWPP_BINARY
{
    _Field_size_bytes_(Length) void const* Buffer;
    USHORT Length;
} WPP_BINARY;
#endif // WPP_BINARY_def

#ifndef WPP_BINARY_func
# define WPP_BINARY_func
WPP_FORCEINLINE WPP_BINARY
WppBinary(_In_reads_bytes_(Length) void const* Buffer, USHORT Length)
{
    WPP_BINARY data;
    data.Buffer = Buffer;
    data.Length = Length;
    return data;
}
#endif // WPP_BINARY_func

#endif // WPP_ALREADY_INCLUDED

#ifndef WPP_ENABLE_FLAG_BIT
#define WPP_ENABLE_FLAG_BIT(flag) (WPP_CB[((flag) >> 16)].Control).Flags[( (0xFFFF & ((flag)-1) ) / 32)] & (1 << ( ((flag)-1) & 31 ))
#endif
`FORALL i IN TypeSigSet WHERE !UnsafeArgs`

#ifndef WPP_SF_`i.Name`_def
# define WPP_SF_`i.Name`_def
WPP_INLINE void WPP_SF_`i.Name`(WPP_LOGGER_ARG unsigned short id, LPCGUID TraceGuid`i.Arguments`)
{ WPP_TRACE(WPP_GET_LOGGER, WPP_TRACE_OPTIONS, (LPGUID)TraceGuid, id, `i.LogArgs` (void*)0); }
#endif // WPP_SF_`i.Name`_def

#if ENABLE_WPP_RECORDER

#if ENABLE_WPP_TRACE_FILTERING_WITH_WPP_RECORDER

//
// Generate the WPP_RECORDER_AND_TRACE_SF_`i.Name` function
//
#ifndef WPP_RECORDER_AND_TRACE_SF_`i.Name`_def
#define WPP_RECORDER_AND_TRACE_SF_`i.Name`_def
WPP_INLINE
VOID
WPP_RECORDER_AND_TRACE_SF_`i.Name`(
    WPP_LOGGER_ARG
    BOOLEAN  wppEnabled,
    BOOLEAN  recorderEnabled,
    PVOID    AutoLogContext,
    UCHAR    level,
    ULONG    flags,
    USHORT   id,
    LPCGUID  traceGuid
    `i.Arguments`
    )
{
    if (wppEnabled)
    {
        WPP_TRACE( WPP_GET_LOGGER,
                   WPP_TRACE_OPTIONS,
                   (LPGUID)traceGuid,
                   id,
                   `i.LogArgs` (void*)0);
    }

    if (recorderEnabled)
    {
        WPP_RECORDER( AutoLogContext, level, flags, (LPGUID) traceGuid, id, `i.LogArgs` (void*)0 );
    }
}
#endif // WPP_RECORDER_AND_TRACE_SF_`i.Name`_def

#else  // ENABLE_WPP_TRACE_FILTERING_WITH_WPP_RECORDER

//
// Generate the WPP_RECORDER_SP_`i.Name` function
//
#ifndef WPP_RECORDER_SF_`i.Name`_def
#define WPP_RECORDER_SF_`i.Name`_def
WPP_INLINE
VOID
WPP_RECORDER_SF_`i.Name`(
    PVOID    AutoLogContext,
    UCHAR    level,
    ULONG    flags,
    USHORT   id,
    LPCGUID  traceGuid
    `i.Arguments`
    )
{
    if (WPP_ENABLE_FLAG_BIT(flags) &&
        (WPP_CONTROL(flags).Level >= level))
    {
        WPP_TRACE(
            WPP_CONTROL(flags).Logger,
            WPP_TRACE_OPTIONS,
            (LPGUID)traceGuid,
            id,
            `i.LogArgs` (void*)0);
    }

    WPP_RECORDER(AutoLogContext, level, flags, (LPGUID) traceGuid, id, `i.LogArgs` (void*)0);
}
#endif // WPP_RECORDER_SF_`i.Name`_def

#endif // ENABLE_WPP_TRACE_FILTERING_WITH_WPP_RECORDER

#endif // ENABLE_WPP_RECORDER
`ENDFOR`
`FORALL i IN TypeSigSet WHERE UnsafeArgs`

#ifndef WPP_SF_`i.Name`_def
#define WPP_SF_`i.Name`_def
WPP_INLINE void WPP_SF_`i.Name`(WPP_LOGGER_ARG unsigned short id, LPCGUID TraceGuid, ...)
{
    va_list ap;
    va_start(ap, TraceGuid);
    UNREFERENCED_PARAMETER(ap);
    {
        `i.DeclVars`
        WPP_TRACE(
            WPP_GET_LOGGER,
            WPP_TRACE_OPTIONS,
            (LPGUID)TraceGuid,
            id,
            `i.LogArgs` (void*)0);
    }
}
#endif // WPP_SF_`i.Name`_def
`ENDFOR`

// WPP_LOG_ALWAYS:
// Called for each event: WPP_LOG_ALWAYS(EX, MSG, arg1, arg2, arg3...) Other()
// If defined, the definition needs to include a trailing comma or semicolon.
// In addition, you will need to define a WPP_EX_[args](args...) macro to
// extract any needed information from the other arguments (e.g. LEVEL).
#ifndef WPP_LOG_ALWAYS
#define WPP_LOG_ALWAYS(...)
#endif

// WPP_DEBUG:
// Called for each enabled event: WPP_DEBUG((MSG, arg1, arg2, arg3...)), Other()
// Potential definition: printf MsgArgs
// Definition should not include any trailing comma or semicolon.
#ifdef WPP_DEBUG
#define WPP_INVOKE_WPP_DEBUG(MsgArgs) WPP_DEBUG(MsgArgs)
#else // WPP_DEBUG
#define WPP_INVOKE_WPP_DEBUG(MsgArgs) (void)0
#endif // WPP_DEBUG
`FORALL i IN Messages WHERE !MsgArgs`

// WPP_CALL_`i.Name`
#ifndef WPP`i.GooId`_PRE
#  define WPP`i.GooId`_PRE(`i.GooArgs`)
#endif
#ifndef WPP`i.GooId`_POST
#  define WPP`i.GooId`_POST(`i.GooArgs`)
#endif
#if ENABLE_WPP_RECORDER
#if ENABLE_WPP_TRACE_FILTERING_WITH_WPP_RECORDER
#define WPP_CALL_`i.Name`(`i.FixedArgs``i.MacroArgs`) \
    WPP_LOG_ALWAYS(WPP_EX`i.GooId`(`i.GooVals`), `i.DbgMacroArgs`) \
    WPP`i.GooId`_PRE(`i.GooVals`) \
    do {\
        WPP_ANNOTATE(`i.Name`) 0; \
        BOOLEAN wppEnabled = WPP_CHECK_INIT `i.MsgVal`WPP`i.GooId`_ENABLED(`i.GooVals`); \
        BOOLEAN recorderEnabled = WPP_RECORDER_CHECK_INIT `i.MsgVal`WPP_RECORDER`i.GooId`_FILTER(`i.GooVals`); \
        if (wppEnabled || recorderEnabled) { \
            WPP_INVOKE_WPP_DEBUG((`i.DbgMacroArgs`)); \
            WPP_RECORDER_AND_TRACE_SF_`i.TypeSig`( \
                     WPP`i.GooId`_LOGGER(`i.GooVals`) \
                     wppEnabled, recorderEnabled, \
                     WPP_RECORDER`i.GooId`_ARGS(`i.GooVals`), \
                     `i.MsgNo`, \
                     WPP_LOCAL_TraceGuids+0`i.MacroExprs`);\
        } \
    } \
    while(0) \
    WPP`i.GooId`_POST(`i.GooVals`)
#else // ENABLE_WPP_TRACE_FILTERING_WITH_WPP_RECORDER
#define WPP_CALL_`i.Name`(`i.FixedArgs``i.MacroArgs`) \
    WPP_LOG_ALWAYS(WPP_EX`i.GooId`(`i.GooVals`), `i.DbgMacroArgs`) \
    WPP`i.GooId`_PRE(`i.GooVals`) \
    WPP_ANNOTATE(`i.Name`) \
    (( \
        WPP_RECORDER_CHECK_INIT `i.MsgVal`WPP_RECORDER`i.GooId`_FILTER(`i.GooVals`) \
        ?   WPP_INVOKE_WPP_DEBUG((`i.DbgMacroArgs`)), \
            WPP_RECORDER_SF_`i.TypeSig`( \
                WPP_RECORDER`i.GooId`_ARGS(`i.GooVals`), \
                `i.MsgNo`, \
                WPP_LOCAL_TraceGuids+0`i.MacroExprs`), \
            1 \
        :   0 \
    )) \
    WPP`i.GooId`_POST(`i.GooVals`)
#endif // ENABLE_WPP_TRACE_FILTERING_WITH_WPP_RECORDER
#else  // ENABLE_WPP_RECORDER
#define WPP_CALL_`i.Name`(`i.FixedArgs``i.MacroArgs`) \
    WPP_LOG_ALWAYS(WPP_EX`i.GooId`(`i.GooVals`), `i.DbgMacroArgs`) \
    WPP`i.GooId`_PRE(`i.GooVals`) \
    WPP_ANNOTATE(`i.Name`) \
    (( \
        WPP_CHECK_INIT `i.MsgVal`WPP`i.GooId`_ENABLED(`i.GooVals`) \
        ?   WPP_INVOKE_WPP_DEBUG((`i.DbgMacroArgs`)), \
            WPP_SF_`i.TypeSig`( \
                WPP`i.GooId`_LOGGER(`i.GooVals`) \
                `i.MsgNo`, \
                WPP_LOCAL_TraceGuids+0`i.MacroExprs`), \
            1 \
        :   0 \
    )) \
    WPP`i.GooId`_POST(`i.GooVals`)
#endif // ENABLE_WPP_RECORDER
`ENDFOR`
`FORALL i IN Messages WHERE MsgArgs`

// WPP_CALL_`i.Name`
#ifndef WPP`i.GooId`_PRE
#  define WPP`i.GooId`_PRE(`i.GooArgs`)
#endif
#ifndef WPP`i.GooId`_POST
#  define WPP`i.GooId`_POST(`i.GooArgs`)
#endif
#if ENABLE_WPP_RECORDER
#define WPP_CALL_`i.Name`(`i.FixedArgs` MSGARGS) \
    WPP_LOG_ALWAYS(WPP_EX`i.GooId`(`i.GooVals`), WPP_FLATTEN MSGARGS) \
    WPP`i.GooId`_PRE(`i.GooVals`) \
    WPP_ANNOTATE(`i.Name`) \
    (( \
        WPP_RECORDER_CHECK_INIT `i.MsgVal`WPP_RECORDER`i.GooId`_FILTER(`i.GooVals`) \
        ?   WPP_INVOKE_WPP_DEBUG(MSGARGS), \
            WPP_RECORDER_SF_`i.TypeSig`( \
                WPP_RECORDER`i.GooId`_ARGS(`i.GooVals`), \
                `i.MsgNo`, \
                WPP_LOCAL_TraceGuids+0 `i.SyntheticExprs`WPP_R`i.ReorderSig` MSGARGS), \
            1 \
        :   0 \
    )) \
    WPP`i.GooId`_POST(`i.GooVals`)
#else // ENABLE_WPP_RECORDER
#define WPP_CALL_`i.Name`(`i.FixedArgs` MSGARGS) \
    WPP_LOG_ALWAYS(WPP_EX`i.GooId`(`i.GooVals`), WPP_FLATTEN MSGARGS) \
    WPP`i.GooId`_PRE(`i.GooVals`) \
    WPP_ANNOTATE(`i.Name`) \
    (( \
        WPP_CHECK_INIT WPP`i.GooId`_ENABLED(`i.GooVals`) \
        ?   WPP_INVOKE_WPP_DEBUG(MSGARGS), \
            WPP_SF_`i.TypeSig`( \
                WPP`i.GooId`_LOGGER(`i.GooVals`) \
                `i.MsgNo`, \
                           WPP_LOCAL_TraceGuids+0 `i.SyntheticExprs`WPP_R`i.ReorderSig` MSGARGS),\
            1 \
        :   0 \
    )) \
    WPP`i.GooId`_POST(`i.GooVals`)
#endif // ENABLE_WPP_RECORDER
`ENDFOR`

// Functions
`FORALL f IN Funcs WHERE !DoubleP && !MsgArgs && !NoMsg`
#undef `f.Name`
#ifdef __INTELLISENSE__
#define `f.Name`(`f.FixedArgs`MSG, ...) ((void)(MSG, ## __VA_ARGS__))
#else
#define `f.Name` WPP_(CALL)
#endif
`ENDFOR`
`FORALL f IN Funcs WHERE !DoubleP && !MsgArgs && NoMsg`
#undef `f.Name`
#ifdef __INTELLISENSE__
#define `f.Name`(`f.FixedArgs`) ((void)0)
#else
#define `f.Name` WPP_(CALL)
#endif
`ENDFOR`
`FORALL f IN Funcs WHERE DoubleP && !MsgArgs`
#undef `f.Name`
#ifdef __INTELLISENSE__
#define `f.Name`(ARGS) ((void)ARGS)
#else
#define `f.Name`(ARGS) WPP_(CALL) ARGS
#endif
`ENDFOR`
`FORALL f IN Funcs WHERE MsgArgs`
#undef `f.Name`
#ifdef __INTELLISENSE__
#define `f.Name`(`f.FixedArgs`MSGARGS) ((void)(MSGARGS))
#else
#define `f.Name`(`f.FixedArgs`MSGARGS) WPP_(CALL)(`f.FixedArgs`MSGARGS)
#endif
`ENDFOR`
`FORALL r IN Reorder`
#undef  WPP_R`r.Name`
#define WPP_R`r.Name`(`r.Arguments`) `r.Permutation`
`ENDFOR`

#ifdef __cplusplus
} // extern "C"
#endif
