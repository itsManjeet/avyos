`**********************************************************************`
`* This is a template file for the tracewpp preprocessor.             *`
`* If you need to use a custom version of this file in your project,  *`
`* please clone it from this one and point WPP to it by specifying    *`
`* -gen:{yourfile}*.tmh on the RUN_WPP line in your sources file.     *`
`*                                                                    *`
`*    Copyright (c) Microsoft Corporation. All rights reserved.       *`
`**********************************************************************`
// `Compiler.Checksum` Generated file. Do not edit.
// File created by `Compiler.Name` compiler version `Compiler.Version`
// from template `TemplateFile`

// PLEASE NOTE:
//    Using simple.tpl for production environment without overriding
//    WPP_ENABLED and WPP_LOGGER macro is NOT RECOMMENDED.
//
//    If WPP_ENABLED is not provided, the logging is always on
//    If WPP_LOGGER is not provided, traces always go to the logger named stdout (if it is running)
//

`INCLUDE header.tpl`
`INCLUDE stdout.tpl`
`INCLUDE tracemacro.tpl`

`IF FOUND WPP_INIT_TRACING`
#ifndef WPP_INIT_TRACING
#define WPP_INIT_TRACING(...) ((void)0)
#endif
#ifndef WPP_CLEANUP
#define WPP_CLEANUP(...) ((void)0)
#endif
`ENDIF`
