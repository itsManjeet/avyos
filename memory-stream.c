#include <stdlib.h>

#include "memory-stream.h"

bool
memory_stream_open(struct memory_stream *m)
{
	*m = (struct memory_stream){ 0 };
	m->fp = open_memstream(&m->str, &m->str_len);

	return m->fp != NULL;
}

char *
memory_stream_close(struct memory_stream *m)
{
	char *str;
	int ret;

	ret = fclose(m->fp);
	str = m->str;
	*m = (struct memory_stream){ 0 };

	if (ret != 0) {
		free(str);
		str = NULL;
	}

	return str;
}

void
memory_stream_cleanup(struct memory_stream *m)
{
	free(memory_stream_close(m));
}
