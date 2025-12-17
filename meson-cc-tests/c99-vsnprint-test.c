#include <sys/types.h>
#include <stdio.h>
#include <stdarg.h>
#include <stdlib.h>

int
foo (char *format, ...)
{
    va_list va;
    char    c;
    int     ret = 0;

    va_start (va, format);
    if (vsnprintf (&c, 1, format, va) < 0) {
	ret = -1;
    }
    va_end (va);

    return ret;
}

int
main (void)
{
    char c;

    return foo("foo: %s", "bar");
}
