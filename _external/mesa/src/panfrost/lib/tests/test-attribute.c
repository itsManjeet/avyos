/*
 * Copyright 2025 Collabora Ltd.
 * SPDX-License-Identifier: MIT
 */
#include "pan_encoder.h"

struct test {
   const char *label;
   uint32_t offset_id;
   uint32_t divisor;
};

#define TEST_DEF(label, offset_id, divisor) {label, offset_id, divisor}

static const struct test attribute_tests[] = {
   TEST_DEF("Sanity check even", 2, 2),
   TEST_DEF("Sanity check odd", 9, 3),
   TEST_DEF("Sanity check random #1", 42, 3),
   TEST_DEF("Sanity check random #2", 999, 3),
   TEST_DEF("Sanity check max #1", UINT32_MAX, UINT32_MAX),
   TEST_DEF("Sanity check max #2", UINT32_MAX, INT32_MAX),
   TEST_DEF("Regression test case #1", 404896682, 444453),
   TEST_DEF("Regression test case #2", 449209605, 980807),
   TEST_DEF("Regression test case #3", 412962763, 103240691),
   TEST_DEF("Regression test case #4", 1269169964, 507465),
   TEST_DEF("Regression test case #5", 1771745081, 6867229),
};

static uint32_t
compute_effective_id(uint64_t offset_id, uint64_t r, uint64_t e, uint64_t d)
{
   return (offset_id + e) * (d + (1u << 31)) / ((uint64_t)1 << (32 + r));
}

#define ASSERT_EQ(x, y)                                                        \
   do {                                                                        \
      if (x == y) {                                                            \
         nr_pass++;                                                            \
      } else {                                                                 \
         nr_fail++;                                                            \
         fprintf(stderr, "%s: Assertion failed %s (%x) != %s (%x)\n", T.label, \
                 #x, x, #y, y);                                                \
      }                                                                        \
   } while (0)

int
main(int argc, const char **argv)
{
   unsigned nr_pass = 0, nr_fail = 0;

   for (unsigned i = 0; i < ARRAY_SIZE(attribute_tests); ++i) {
      struct test T = attribute_tests[i];

      uint32_t e;
      uint32_t r;
      uint32_t d = pan_compute_npot_divisor(T.divisor, &r, &e);

      uint32_t expected_effective_id = T.offset_id / T.divisor;
      uint32_t computed_effective_id =
         compute_effective_id(T.offset_id, r, e, d);

      ASSERT_EQ(expected_effective_id, computed_effective_id);
   }

   printf("Passed %u/%u\n", nr_pass, nr_pass + nr_fail);
   return nr_fail ? 1 : 0;
}
