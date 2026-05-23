/*
`notify` sends a notification to the desktop service.

It is useful for scripts, background jobs, or apps that want to surface a
title/message pair in the system notification center.

Example usage:
```sh
notify "Build finished" "All checks passed"
notify -app-name CI -icon emblem-ok -title "Pipeline" -message "Deploy complete"
echo "Long message body" | notify -title "Notes" -stdin
```
*/
package main
