<?xml version="1.0" standalone="yes"?>
<!--
`**********************************************************************`
`* This is a template file for the tracewpp preprocessor.             *`
`* If you need to use a custom version of this file in your project,  *`
`* please clone it from this one and point WPP to it by specifying    *`
`* -gen:{yourfile}*.tmh on the RUN_WPP line in your sources file.     *`
`*                                                                    *`
`*    Copyright (c) Microsoft Corporation. All rights reserved.       *`
`**********************************************************************`
Checksum="`Compiler.Checksum`" Generated file. Do not edit.
File created by `Compiler.Name` compiler version `Compiler.Version`
from template `TemplateFile`
`ENCODING UTF-8`
***************************************************************************
PREVIEW: Behavior of the -man option, the `TemplateFile` template, and the
associated configuration options may change in future versions of tracewpp.
***************************************************************************
-->
<instrumentationManifest
    xmlns="http://schemas.microsoft.com/win/2004/08/events"
    xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:win="http://manifests.microsoft.com/win/2004/08/windows/events"
    >
  <instrumentation>
    <events>
`FORALL ControlGuid IN ControlGuids`
`  FORALL TraceGuid IN ControlGuid.TraceGuids`
      <provider
          name="`ControlGuid.Name`_`TraceGuid.Normalized`"
          controlGuid="{`ControlGuid.Text`}"
          guid="{`TraceGuid.Text`}"
          symbol="`ControlGuid.Symbol``IF TraceGuid No`_`TraceGuid.No``ENDIF`"
          resourceFileName="`Manifest.ResourceFileName`"
          messageFileName="`Manifest.MessageFileName`"
          >
`    IF ControlGuid Levels`
        <levels>
`      FORALL Level IN ControlGuid.Levels`
          <level value="`Level.Value`" name="`Level.Name`" symbol="`Level.Symbol`" />
`      ENDFOR Level`
        </levels>
`    ENDIF ControlGuid Levels`
`    IF ControlGuid Keywords`
        <keywords>
`      FORALL Keyword IN ControlGuid.Keywords`
          <keyword mask="`Keyword.Mask`" name="`Keyword.Name`" symbol="`Keyword.Symbol`" />
`      ENDFOR Keyword`
        </keywords>
`    ENDIF ControlGuid Keywords`
`    IF TraceGuid Maps`
        <maps>
`      FORALL Type IN TraceGuid.Types WHERE ItemSet`
          <bitMap name="`Type.Name`">
`        FORALL Item IN Type.MapItems`
            <map value="0x`Item.HexValue`" message="$(string.S`Item.MessageId`)" />
`        ENDFOR Item`
          </bitMap>
`      ENDFOR Type`
`      FORALL Type IN TraceGuid.Types WHERE ItemList`
          <valueMap name="`Type.Name`">
`        FORALL Item IN Type.MapItems`
            <map value="`Item.Value`" message="$(string.S`Item.MessageId`)" />
`        ENDFOR Item`
          </valueMap>
`      ENDFOR Type`
        </maps>
`    ENDIF TraceGuid Maps`
        <events>
`    FORALL Message IN TraceGuid.Messages`
          <event
`      IF Message EventName`
              name="`Message.XmlEventName`"
`      ENDIF Message EventName`
              attributes='`Message.Attributes`'
              value="`Message.MsgNo`"
`      IF Message Version`
              version="`Message.Version`"
`      ENDIF Message Version`
              level="`Message.Level`"
              keywords="`Message.Keywords`"
              symbol="`Message.Name`"
              message="$(string.S`Message.ManifestTextId`)"
`      IF Message Arguments`
              template="T`Message.TemplateId`"
`      ENDIF Message Arguments`
              />
`    ENDFOR Message`
        </events>
        <templates>
`    FORALL Template IN TraceGuid.Templates`
          <template tid="T`Template.Id`">
`        FORALL Argument IN Template.Arguments`
            <data
`          IF Argument NameSpecified`
                name="`Argument.XmlName`"
`          ENDIF Argument NameSpecified`
`          IF Argument !NameSpecified`
                name="A`Argument.No`_`Argument.XmlName`"
`          ENDIF Argument !NameSpecified`
                inType="`Argument.InType`"
`          IF Argument MapName`
                map="`Argument.MapName`"
`          ENDIF Argument MapName`
`          IF Argument OutType`
                outType="`Argument.OutType`"
`          ENDIF Argument OutType`
`          IF Argument Length`
                length="`Argument.Length`"
`          ENDIF Argument Length`
                />
`        ENDFOR Argument`
          </template>
`    ENDFOR Template`
        </templates>
      </provider>
`  ENDFOR TraceGuid`
`ENDFOR ControlGuid`
    </events>
  </instrumentation>
  <localization>
    <resources culture="`Manifest.Culture`">
      <stringTable>
`FORALL NameId IN StringTable`
        <string
            id="S`NameId.Id`"
            value="`NameId.XmlName`"
            />
`ENDFOR Message`
      </stringTable>
    </resources>
  </localization>
</instrumentationManifest>
