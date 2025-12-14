/*
 * Copyright © 2020 Google LLC
 * SPDX-License-Identifier: MIT
 */

#include "freedreno_layout.h"
#include "fd_layout_test.h"
#include "fd6_hw.h"

#include <stdio.h>

bool
fdl_test_layout(const struct testcase *testcase, const struct fd_dev_id *dev_id)
{
   struct fdl_layout layout;
   bool ok = true;

   int max_size = MAX2(testcase->layout.width0, testcase->layout.height0);
   int mip_levels = 1;
   while (max_size > 1 && testcase->layout.slices[mip_levels].pitch) {
      mip_levels++;
      max_size = u_minify(max_size, 1);
   }

   struct fdl_image_params params = {
      .format = testcase->format,
      .nr_samples = MAX2(testcase->layout.nr_samples, 1),
      .width0 = testcase->layout.width0,
      .height0 = MAX2(testcase->layout.height0, 1),
      .depth0 = MAX2(testcase->layout.depth0, 1),
      .mip_levels = mip_levels,
      .array_size = MAX2(testcase->array_size, 1),
      .is_3d = testcase->is_3d,
      .ubwc = testcase->layout.ubwc,
      .tile_mode = testcase->layout.tile_mode,
   };

   if (fd_dev_gen(dev_id) >= 6) {
      const struct fd_dev_info *dev_info = fd_dev_info_raw(dev_id);
      fdl6_layout_image(&layout, dev_info, &params, NULL);
   } else {
      assert(fd_dev_gen(dev_id) >= 5);
      fdl5_layout_image(&layout, &params);
   }

   /* fdl lays out UBWC data before the color data, while all we have
    * recorded in this testcase are the color offsets (other than the UBWC
    * buffer sharing test).  Shift the fdl layout down so we can compare
    * color offsets.
    */
   if (layout.ubwc && !testcase->layout.slices[0].offset) {
      for (int l = 1; l < mip_levels; l++)
         layout.slices[l].offset -= layout.slices[0].offset;
      layout.slices[0].offset = 0;
   }

   for (int l = 0; l < mip_levels; l++) {
      if (layout.slices[l].offset != testcase->layout.slices[l].offset) {
         fprintf(stderr, "%s %dx%dx%d@%dx lvl%d: offset 0x%x != 0x%x\n",
                 util_format_short_name(testcase->format), layout.width0,
                 layout.height0, layout.depth0, layout.nr_samples, l,
                 layout.slices[l].offset, testcase->layout.slices[l].offset);
         ok = false;
      }
      if (fdl_pitch(&layout, l) != testcase->layout.slices[l].pitch) {
         fprintf(stderr, "%s %dx%dx%d@%dx lvl%d: pitch %d != %d\n",
                 util_format_short_name(testcase->format), layout.width0,
                 layout.height0, layout.depth0, layout.nr_samples, l,
                 fdl_pitch(&layout, l), testcase->layout.slices[l].pitch);
         ok = false;
      }

      /* Test optional requirement of the slice size.  Important for testing 3D
       * layouts.
       */
      if (testcase->layout.slices[l].size0 && layout.slices[l].size0 !=
          testcase->layout.slices[l].size0) {
         fprintf(stderr, "%s %dx%dx%d@%dx lvl%d: slice size %d != %d\n",
                 util_format_short_name(testcase->format), layout.width0,
                 layout.height0, layout.depth0, layout.nr_samples, l,
                 layout.slices[l].size0,
                 testcase->layout.slices[l].size0);
         ok = false;
      }

      if (layout.ubwc_slices[l].offset !=
          testcase->layout.ubwc_slices[l].offset) {
         fprintf(stderr, "%s %dx%dx%d@%dx lvl%d: UBWC offset 0x%x != 0x%x\n",
                 util_format_short_name(testcase->format), layout.width0,
                 layout.height0, layout.depth0, layout.nr_samples, l,
                 layout.ubwc_slices[l].offset,
                 testcase->layout.ubwc_slices[l].offset);
         ok = false;
      }
      if (fdl_ubwc_pitch(&layout, l) != testcase->layout.ubwc_slices[l].pitch) {
         fprintf(stderr, "%s %dx%dx%d@%dx lvl%d: UBWC pitch %d != %d\n",
                 util_format_short_name(testcase->format), layout.width0,
                 layout.height0, layout.depth0, layout.nr_samples, l,
                 fdl_ubwc_pitch(&layout, l),
                 testcase->layout.ubwc_slices[l].pitch);
         ok = false;
      }
   }

   if (!ok) {
      fdl_dump_layout(&layout);
      fprintf(stderr, "\n");
   }

   return ok;
}
