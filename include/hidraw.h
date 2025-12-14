#ifndef _SEATD_HIDRAW_H
#define _SEATD_HIDRAW_H

int hidraw_revoke(int fd);
int path_is_hidraw(const char *path);

#endif
