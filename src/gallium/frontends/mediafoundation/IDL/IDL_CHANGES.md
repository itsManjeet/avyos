# Making changes to IDL files on mediafoundation frontend

When changing any .idl on this folder, these must be recompiled with midlrt to produce the public, private headers and the .winmd files.

## How to build the modified IDL files

```
    midlrt.EXE <input_idl_file>.idl /metadata_dir C:\Windows\System32\WinMetadata
```