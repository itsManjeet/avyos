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

`FORALL Guid IN TraceGuids`
`Guid.Text` `Guid.Comment`
`FORALL Msg in Guid.Messages`
#typev `Msg.Name` `Msg.MsgNo` "`Msg.Text`"
{
`FORALL Arg IN Msg.Arguments`
  `Arg.Name`, `Arg.MofType` // `Arg.No`
`ENDFOR Arg`
}
`ENDFOR Msg`
`ENDFOR Guid`
