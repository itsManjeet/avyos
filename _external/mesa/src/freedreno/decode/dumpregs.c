/*
 * Copyright (c) Qualcomm Technologies, Inc. and/or its susidiaries.
 * SPDX-License-Identifier: MIT
 */

#include <stdbool.h>
#include <stdio.h>

#include "rnndec.h"
#include "rnnutil.h"

/**
 * A simple utility to dump a list off offset,name pairs for registers of
 * the specificied generation.
 */

int
main(int argc, char **argv)
{
   struct rnn *rnn;

   if (argc != 2) {
      printf("usage: dumpregs GEN\n");
      return -1;
   }

   rnn = rnn_new(true);

   rnn_load(rnn, argv[1]);

   for (unsigned i = 0; i < rnn->dom[0]->subelemsnum; i++) {
      struct rnndelem *e = rnn->dom[0]->subelems[i];

      if (!rnndec_varmatch(rnn->vc, &e->varinfo))
         continue;

      printf("0x%05X,%s\n", (uint32_t)e->offset, e->name);
   }

   return 0;
}
