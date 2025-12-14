/*
 * Copyright 2024-2025 Intel Corporation
 * SPDX-License-Identifier: MIT
 */

#include "error_decode_lib.h"

#include <zlib.h>
#include <stdlib.h>
#include <string.h>

const char *
ascii85_decode_char(const char *in, uint32_t *v)
{
   *v = 0;

   if (*in == 'z') {
      in++;
   } else {
      *v += in[0] - 33; *v *= 85;
      *v += in[1] - 33; *v *= 85;
      *v += in[2] - 33; *v *= 85;
      *v += in[3] - 33; *v *= 85;
      *v += in[4] - 33;
      in += 5;
   }

   return in;
}

int zlib_inflate(uint32_t **ptr, int len)
{
   struct z_stream_s zstream;
   void *out;
   const uint32_t out_size = 128*4096;  /* approximate obj size */

   memset(&zstream, 0, sizeof(zstream));

   zstream.next_in = (unsigned char *)*ptr;
   zstream.avail_in = 4*len;

   if (inflateInit(&zstream) != Z_OK)
      return 0;

   out = malloc(out_size);
   zstream.next_out = out;
   zstream.avail_out = out_size;

   do {
      switch (inflate(&zstream, Z_SYNC_FLUSH)) {
      case Z_STREAM_END:
         goto end;
      case Z_OK:
         break;
      default:
         free(out);
         inflateEnd(&zstream);
         return 0;
      }

      if (zstream.avail_out)
         break;

      out = realloc(out, 2*zstream.total_out);
      if (out == NULL) {
         inflateEnd(&zstream);
         return 0;
      }

      zstream.next_out = (unsigned char *)out + zstream.total_out;
      zstream.avail_out = zstream.total_out;
   } while (1);
 end:
   inflateEnd(&zstream);
   free(*ptr);
   *ptr = out;
   return zstream.total_out / 4;
}

int ascii85_decode(const char *in, uint32_t **out, bool inflate)
{
   int len = 0, size = 1024;

   *out = realloc(*out, sizeof(uint32_t)*size);
   if (*out == NULL)
      return 0;

   while (*in >= '!' && *in <= 'z') {
      uint32_t v = 0;

      if (len == size) {
         size *= 2;
         *out = realloc(*out, sizeof(uint32_t)*size);
         if (*out == NULL)
            return 0;
      }

      in = ascii85_decode_char(in, &v);
      (*out)[len++] = v;
   }

   if (!inflate)
      return len;

   return zlib_inflate(out, len);
}
