`**********************************************************************`
`* This is an include template file for the tracewpp preprocessor.    *`
`*                                                                    *`
`*    Copyright (c) Microsoft Corporation. All rights reserved.       *`
`**********************************************************************`
// template `TemplateFile`

//
//     Defines a set of macro that expand control model specified
//     with WPP_CONTROL_GUIDS (example shown below)
//     into an enum of trace levels and required structures that
//     contain the mask of levels, logger handle and some information
//     required for registration.
//

#ifndef WPP_ALREADY_INCLUDED

#define WPP_EVAL(x) x
#define WPP_STR(x)  #x
#define WPP_STRINGIZE(x) WPP_STR(x)
#define WPP_GLUE(a, b)  a ## b
#define WPP_GLUE3(a, b, c)  a ## b ## c
#define WPP_GLUE4(a, b, c, d)  a ## b ## c ## d
#define WPP_XGLUE(a, b) WPP_GLUE(a, b)
#define WPP_XGLUE3(a, b, c) WPP_GLUE3(a, b, c)
#define WPP_XGLUE4(a, b, c, d) WPP_GLUE4(a, b, c, d)

///////////////////////////////////////////////////////////////////////////////////
//
// #define WPP_CONTROL_GUIDS \
//     WPP_DEFINE_CONTROL_GUID(Regular,(81b20fea,73a8,4b62,95bc,354477c97a6f), \
//       WPP_DEFINE_BIT(Error)      \
//       WPP_DEFINE_BIT(Unusual)    \
//       WPP_DEFINE_BIT(Noise)      \
//    )        \
//    WPP_DEFINE_CONTROL_GUID(HiFreq,(91b20fea,73a8,4b62,95bc,354477c97a6f), \
//       WPP_DEFINE_BIT(Entry)      \
//       WPP_DEFINE_BIT(Exit)       \
//       WPP_DEFINE_BIT(ApiCalls)   \
//       WPP_DEFINE_BIT(RandomJunk) \
//       WPP_DEFINE_BIT(LovePoem)   \
//    )

#ifdef __cplusplus
extern "C" {
#endif

#ifndef WPP_NO_CONTROL_GUIDS

#ifdef WPP_DEFAULT_CONTROL_GUID
#  ifdef WPP_CONTROL_GUIDS
#     error WPP_DEFAULT_CONTROL_GUID cannot be used together with WPP_CONTROL_GUIDS.
#  else // WPP_CONTROL_GUIDS
#     define WPP_CONTROL_GUIDS \
         WPP_DEFINE_CONTROL_GUID(Default,(WPP_DEFAULT_CONTROL_GUID), \
         WPP_DEFINE_BIT(Error)   \
         WPP_DEFINE_BIT(Unusual) \
         WPP_DEFINE_BIT(Noise)   \
      )
#  endif // WPP_CONTROL_GUIDS
#endif // WPP_DEFAULT_CONTROL_GUID

#ifndef WPP_CONTROL_GUIDS
#  pragma message(__FILE__ " : error : Please define control model via WPP_CONTROL_GUIDS or WPP_DEFAULT_CONTROL_GUID macros")
#  pragma message(__FILE__ " : error : don't forget to call WPP_INIT_TRACING and WPP_CLEANUP in your main, DriverEntry or DllInit")
#  pragma message(__FILE__ " : error : see tracewpp.doc for further information")
#  error WPP_CONTROL_GUIDS not defined.
#endif // WPP_CONTROL_GUIDS
// a set of macro to convert a guid in a form x(81b20fea,73a8,4b62,95bc,354477c97a6f)
// into either a a struct or text string

#define _WPPW(x) WPP_GLUE(L, x)

#define WPP_GUID_NORM(l,w1,w2,w3,ll) l ## w1 ## w2 ## w3 ## ll
#define WPP_GUID_TEXT(l,w1,w2,w3,ll) #l "-" #w1 "-" #w2 "-" #w3 "-" #ll
#define WPP_GUID_WTEXT(l,w1,w2,w3,ll) _WPPW(#l) L"-" _WPPW(#w1) L"-" _WPPW(#w2) L"-" _WPPW(#w3) L"-" _WPPW(#ll)
#define WPP_EXTRACT_BYTE(val,n) (((ULONGLONG)(0x ## val) >> (8 * n)) & 0xFF)
#define WPP_GUID_STRUCT(l,w1,w2,w3,ll) {0x ## l, 0x ## w1, 0x ## w2,\
     {WPP_EXTRACT_BYTE(w3, 1), WPP_EXTRACT_BYTE(w3, 0),\
      WPP_EXTRACT_BYTE(ll, 5), WPP_EXTRACT_BYTE(ll, 4),\
      WPP_EXTRACT_BYTE(ll, 3), WPP_EXTRACT_BYTE(ll, 2),\
      WPP_EXTRACT_BYTE(ll, 1), WPP_EXTRACT_BYTE(ll, 0)} }

#ifndef WPP_FORCEINLINE
#define WPP_FORCEINLINE __forceinline
#endif

// define an enum of control block names
//////
#define WPP_DEFINE_CONTROL_GUID(Name,Guid,Bits) WPP_XGLUE(WPP_CTL_, WPP_EVAL(Name)),
enum WPP_CTL_NAMES { WPP_CONTROL_GUIDS WPP_LAST_CTL};
#undef WPP_DEFINE_CONTROL_GUID

// define control guids
//////
#define WPP_DEFINE_CONTROL_GUID(Name,Guid,Bits) \
extern __declspec(selectany) const GUID WPP_XGLUE4(WPP_, ThisDir, _CTLGUID_, WPP_EVAL(Name)) = WPP_GUID_STRUCT Guid;
WPP_CONTROL_GUIDS
#undef WPP_DEFINE_CONTROL_GUID

// define enums of individual bits
/////
#define WPP_DEFINE_CONTROL_GUID(Name,Guid,Bits) \
    WPP_XGLUE(WPP_BLOCK_START_, WPP_EVAL(Name)) = WPP_XGLUE(WPP_CTL_, WPP_EVAL(Name)) * 0x10000, Bits WPP_XGLUE(WPP_BLOCK_END_, WPP_EVAL(Name)),
# define WPP_DEFINE_BIT(Name) WPP_BIT_ ## Name,
enum WPP_DEFINE_BIT_NAMES { WPP_CONTROL_GUIDS };
# undef WPP_DEFINE_BIT
#undef WPP_DEFINE_CONTROL_GUID

#define WPP_MASK(CTL)    (1 << ( ((CTL)-1) & 31 ))
#define WPP_FLAG_NO(CTL) ( (0xFFFF & ((CTL)-1) ) / 32)
#define WPP_CTRL_NO(CTL) ((CTL) >> 16)

// calculate how many DWORDs we need to get the required number of bits
// upper estimate. Sometimes will be off by one
#define WPP_DEFINE_CONTROL_GUID(Name,Guid,Bits) | WPP_XGLUE(WPP_BLOCK_END_, WPP_EVAL(Name))
enum _WPP_FLAG_LEN_ENUM { WPP_FLAG_LEN = 1 | ((0 WPP_CONTROL_GUIDS) & 0xFFFF) / 32 };
#undef WPP_DEFINE_CONTROL_GUID

//
// Check that maximum number of flags does not exceed 32
//
#ifndef C_ASSERT
#define C_ASSERT(e) typedef char __C_ASSERT__[(e)?1:-1]
#endif

#define MAX_NUMBER_OF_ETW_FLAGS 34 // 32 flags plus 2 separators
#define WPP_DEFINE_CONTROL_GUID(Name,Guid,Bits) && ((WPP_XGLUE(WPP_BLOCK_END_, WPP_EVAL(Name) & 0xFFFF)) < MAX_NUMBER_OF_ETW_FLAGS)
enum _WPP_FLAG_LEN_ENUM_MAX { WPP_MAX_FLAG_LEN_CHECK = (1 WPP_CONTROL_GUIDS) };
#undef WPP_DEFINE_CONTROL_GUID

#ifndef WPP_CB
#define WPP_CB      WPP_GLOBAL_Control
#endif
#ifndef WPP_CB_TYPE
#define WPP_CB_TYPE WPP_PROJECT_CONTROL_BLOCK
#endif

#ifndef WPP_CHECK_INIT
#define WPP_CHECK_INIT (WPP_CB != (WPP_CB_TYPE*)&WPP_CB) &&
#endif

typedef union {
    WPP_TRACE_CONTROL_BLOCK Control;
    UCHAR ReserveSpace[ sizeof(WPP_TRACE_CONTROL_BLOCK) + sizeof(ULONG) * (WPP_FLAG_LEN - 1) ];
} WPP_CB_TYPE ;


extern __declspec(selectany) WPP_CB_TYPE *WPP_CB = (WPP_CB_TYPE*)&WPP_CB;

#if ENABLE_WPP_RECORDER
#ifndef WPP_RECORDER_CHECK_INIT
#define WPP_RECORDER_CHECK_INIT (WPP_RECORDER_INITIALIZED != (WPP_CB_TYPE*)&WPP_RECORDER_INITIALIZED) &&
#endif
// Global varaible used to track if WPP_RECORDER was initialized.
// It will be initialized on calling WPP_INIT_TRACING macro.
extern __declspec(selectany) WPP_CB_TYPE *WPP_RECORDER_INITIALIZED = (WPP_CB_TYPE*)&WPP_RECORDER_INITIALIZED;
#endif

#define WPP_CONTROL(CTL) (WPP_CB[WPP_CTRL_NO(CTL)].Control)

// Define the default WPP_LEVEL_LOGGER/WPP_LEVEL_ENABLED macros for the
// predefined DoTraceMessage(LEVEL) function.
#ifdef WPP_USE_TRACE_LEVELS

#ifndef WPP_LEVEL_LOGGER
#define WPP_LEVEL_LOGGER(lvl) (WPP_CONTROL(WPP_BIT_ ## DUMMY).Logger),
#endif
#ifndef WPP_LEVEL_ENABLED
#define WPP_LEVEL_ENABLED(lvl) (WPP_CONTROL(WPP_BIT_ ## DUMMY).Level >= lvl)
#endif

#else // WPP_USE_TRACE_LEVELS

// For historical reasons, the use of LEVEL means flags by default.
// This was a bad choice but very difficult to undo.
#ifndef WPP_LEVEL_LOGGER
#  define WPP_LEVEL_LOGGER(CTL)  (WPP_CONTROL(WPP_BIT_ ## CTL).Logger),
#endif
#ifndef WPP_LEVEL_ENABLED
#  define WPP_LEVEL_ENABLED(CTL) (WPP_CONTROL(WPP_BIT_ ## CTL).Flags[WPP_FLAG_NO(WPP_BIT_ ## CTL)] & WPP_MASK(WPP_BIT_ ## CTL))
#endif

#endif // WPP_USE_TRACE_LEVELS

// Define default WPP_FLAG_LOGGER/WPP_FLAG_ENABLED macros for convenience in
// defining a function that takes a FLAG parameter e.g. DoTrace(FLAG).
#ifndef WPP_FLAG_LOGGER
#  define WPP_FLAG_LOGGER(CTL)  (WPP_CONTROL(WPP_BIT_ ## CTL).Logger),
#endif
#ifndef WPP_FLAG_ENABLED
#  define WPP_FLAG_ENABLED(CTL) (WPP_CONTROL(WPP_BIT_ ## CTL).Flags[WPP_FLAG_NO(WPP_BIT_ ## CTL)] & WPP_MASK(WPP_BIT_ ## CTL))
#endif

#ifndef WPP_ENABLED
#  define WPP_ENABLED() 1
#endif
#ifndef WPP_LOGGER
#  define WPP_LOGGER() (WPP_CB[0].Control.Logger),
#endif

#endif // WPP_NO_CONTROL_GUIDS

#ifndef WPP_GET_LOGGER
#  define WPP_GET_LOGGER Logger
#endif

#ifndef WPP_LOGGER_ARG
#  define WPP_LOGGER_ARG TRACEHANDLE Logger,
#endif

#ifdef __cplusplus
} // extern "C"
#endif

#endif // WPP_ALREADY_INCLUDED
