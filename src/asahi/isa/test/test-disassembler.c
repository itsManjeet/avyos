/*
 * Copyright (C) 2021 Collabora, Ltd.
 * SPDX-License-Identifier: MIT
 */

#include <inttypes.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "asahi/isa/disasm.h"
#include "util/macros.h"

static inline uint8_t
parse_nibble(const char c)
{
   return (c >= 'a') ? 10 + (c - 'a') : (c - '0');
}

/* Given a little endian 8 byte hexdump, parse out the 64-bit value */
static uint64_t
parse_hex(const char *in)
{
   uint64_t v = 0;

   for (unsigned i = 0; i < 8; ++i) {
      uint8_t byte = (parse_nibble(in[0]) << 4) | parse_nibble(in[1]);
      v |= ((uint64_t)byte) << (8 * i);

      /* Skip the space after the byte */
      in += 3;
   }

   return v;
}

int
main(int argc, const char **argv)
{
   if (argc < 2) {
      fprintf(stderr, "Expected case list\n");
      return 1;
   }

   FILE *fp = fopen(argv[1], "r");

   if (fp == NULL) {
      fprintf(stderr, "Could not open the case list");
      return 1;
   }

   char line[128];
   unsigned nr_fail = 0, nr_pass = 0;

   while (fgets(line, sizeof(line), fp) != NULL) {
      char *output = NULL;
      size_t sz = 0;
      size_t len = strlen(line);

      /* Skip empty lines */
      if (len <= 1)
         continue;

      /* Parse the hex */
      uint8_t code[16];
      const char *cursor = line;
      const char *end = line + len;
      uint32_t i = 0;
      while ((cursor + 2) <= end && cursor[0] != ' ') {
         assert(i < ARRAY_SIZE(code));
         code[i] = (parse_nibble(cursor[0]) << 4) | parse_nibble(cursor[1]);
         cursor += 2;
         ++i;
      }

      /* Skip spacing */
      while (cursor < end && (*cursor) == ' ')
         ++cursor;

      FILE *outputp = open_memstream(&output, &sz);
      signed ret = agx2_disassemble_instr((void *)code, outputp, 0, false);
      unsigned instr_len = abs(ret);
      fclose(outputp);

      /* Rest of the line is the reference assembly */
      bool fail = strcmp(cursor, output);
      fail |= (instr_len != i);
      fail |= (ret < 0);

      if (fail) {
         /* Extra spaces after Got to align with Expected */
         printf("Got      %sExpected %s\n", output, cursor);

         if (instr_len != i) {
            printf("Got length %d, expected length %u\n", instr_len, i);
         }

         if (ret < 0) {
            printf("Got an error.\n");
         }

         nr_fail++;
      } else {
         nr_pass++;
      }

      free(output);
   }

   printf("Passed %u/%u tests.\n", nr_pass, nr_pass + nr_fail);
   fclose(fp);

   return nr_fail ? 1 : 0;
}
