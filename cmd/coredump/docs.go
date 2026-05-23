/*
`coredump` is a kernel core_pattern helper used to capture crash metadata.

It is intended to be invoked by the kernel, not by users directly. The helper
reads the core stream from stdin, stores metadata under `/cache/crash`, and
tries to notify the desktop service about the crash.
*/
package main
