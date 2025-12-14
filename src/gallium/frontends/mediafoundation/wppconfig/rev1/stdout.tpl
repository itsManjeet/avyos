`**********************************************************************`
`* This is an include template file for the tracewpp preprocessor.    *`
`*                                                                    *`
`*    Copyright (c) Microsoft Corporation. All rights reserved.       *`
`**********************************************************************`
// template `TemplateFile`

`FORALL f IN Funcs`
#ifndef WPP`f.GooId`_LOGGER
#define WPP`f.GooId`_LOGGER(`f.GooArgs`) // `f.Name`
#endif
`ENDFOR`

#ifndef WPP_LOGGER_ARG
#  define WPP_LOGGER_ARG
#endif

#ifndef WPP_GET_LOGGER
  #define WPP_GET_LOGGER WppGetLogger()
  __inline TRACEHANDLE WppGetLogger()
  {
      static TRACEHANDLE Logger;
      if (Logger) {return Logger;}
      return Logger = WppQueryLogger(0);
  }
#endif

#define WPP_GLUE(a, b)  a ## b
#define _WPPW(x) WPP_GLUE(L, x)

#ifndef WPP_CHECK_INIT
#define WPP_CHECK_INIT
#endif

#ifndef WPP_ENABLED
#  define WPP_ENABLED() 1
#endif

#ifndef WPP_LEVEL_ENABLED
#  define WPP_LEVEL_ENABLED(LEVEL) 1
#endif
