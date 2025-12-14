#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/types.h>

#if defined(__linux__)
#include <linux/hidraw.h>
#endif

#include "hidraw.h"
#include "log.h"

#define STRLEN(s) ((sizeof(s) / sizeof(s[0])) - 1)

#if defined(__linux__) && defined(HIDIOCREVOKE)
int path_is_hidraw(const char *path) {
	static const char prefix[] = "/dev/hidraw";
	static const size_t prefixlen = STRLEN(prefix);
	return strncmp(prefix, path, prefixlen) == 0;
}

int hidraw_revoke(int fd) {
	return ioctl(fd, HIDIOCREVOKE, NULL);
}
#else
int path_is_hidraw(const char *path) {
	(void)path;
	return 0;
}
int hidraw_revoke(int fd) {
	(void)fd;
	return 0;
}
#endif
