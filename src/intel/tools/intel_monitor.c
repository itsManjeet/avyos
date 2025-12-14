/*
 * Copyright 2024 Intel Corporation
 * SPDX-License-Identifier: MIT
 *
 * Purpose:
 *   Enables EU stall sampling available in Xe2+ Intel GPUs. Enables
 *   GPU sampling of EU stalls. Every N mircoseconds, program collects
 *   sampled data from GPU. Accumulated data dumped to stdout once
 *   intel_monitor is closed.
 */

#include <fcntl.h>
#include <getopt.h>
#include <termios.h>

#include <xf86drm.h>

#include "intel_monitor_eustall.h"
#include "dev/intel_device_info.h"

static int
get_drm_device(struct intel_device_info *devinfo)
{
   drmDevicePtr devices[8];
   int max_devices = drmGetDevices2(0, devices, 8);

   int i, fd = -1;
   for (i = 0; i < max_devices; i++) {
      if (devices[i]->available_nodes & 1 << DRM_NODE_RENDER &&
          devices[i]->bustype == DRM_BUS_PCI &&
          devices[i]->deviceinfo.pci->vendor_id == 0x8086) {
         fd = open(devices[i]->nodes[DRM_NODE_RENDER], O_RDWR | O_CLOEXEC);
         if (fd < 0)
            continue;

         if (!intel_get_device_info_from_fd(fd, devinfo, -1, -1)) {
            close(fd);
            fd = -1;
            continue;
         }

         /* Found a device! */
         break;
      }
   }

   return fd;
}

/* Keyboard hit function
 *
 * Return true if keypress detected.
 */
static bool
kbhit()
{
    int ch, oldf;
    struct termios oldt, newt;

    tcgetattr(STDIN_FILENO, &oldt);
    newt = oldt;
    newt.c_lflag &= ~(ICANON | ECHO);
    tcsetattr(STDIN_FILENO, TCSANOW, &newt);
    oldf = fcntl(STDIN_FILENO, F_GETFL, 0);
    fcntl(STDIN_FILENO, F_SETFL, oldf | O_NONBLOCK);

    ch = getchar();

    tcsetattr(STDIN_FILENO, TCSANOW, &oldt);
    fcntl(STDIN_FILENO, F_SETFL, oldf);

    if (ch != EOF) {
        ungetc(ch, stdin);
        return true;
    }

    return false;
}

static void
print_help(const char *progname, FILE *file)
{
   fprintf(file,
      "\n"
      "Usage: %s [OPTION]...\n"
      "Sample and collate GPU metrics across system. Studies workloads run in parallel.\n"
      "      -h, --help            display this help\n"
      "      -e, --eustall         sample eu stalls. Supported Xe2+.\n"
      "      -f, --file            name of output file. DEFAULT=stdout\n"
      "      -t, --sample_time     period of sampling, in microseconds. DEFAULT=1000\n"
      "\n"
      "Eu stall sample usage:\n"
      "   0. Run `sudo sysctl dev.xe.observation_paranoid=0`\n"
      "   1. Launch gfx app with INTEL_DEBUG=shaders-lineno. Redirect stderr to asm.txt.\n"
      "   2. When gfx app ready to monitor, begin capturing eustall data by launching\n"
      "      `intel_monitor -e -f eustall.csv` in separate console.\n"
      "   3. When enough data has been collected, close intel_monitor by pressing any key.\n"
      "   4. Correlate eustall data in eustall.csv with shader instructions in asm.txt by\n"
      "      matching instruction offsets. Use data to determine which instructions are\n"
      "      stalling and why.\n"
      "\n"
      "Eu stall defintions:\n"
      "tdr_count        - Number of cycles EU stalled, with at least one thread waiting\n"
      "                   on Pixel Shader dependency. Multiple stall reasons can\n"
      "                   qualify during the same cycle.\n"
      "other_count      - Number of cycles EU stalled, with at least one thread waiting\n"
      "                   on any other dependency (Flag/EoT etc). Multiple stall reasons\n"
      "                   can qualify during the same cycle.\n"
      "control_count    - Number of cycles EU stalled, with at least one thread waiting\n"
      "                   for JEU to complete branch instruction. Multiple stall reasons\n"
      "                   can qualify during the same cycle.\n"
      "pipestall_count  - Number of cycles EU stalled, with at least one thread ready to\n"
      "                   be scheduled (Grf conf/send holds etc). Multiple stall reasons\n"
      "                   can qualify during the same cycle.\n"
      "send_count       - Number of cycles EU stalled, with at least one thread waiting\n"
      "                   for SEND message to be dispatched from EU. Multiple stall\n"
      "                   reasons can qualify during the same cycle.\n"
      "dist_acc_count   - Number of cycles EU stalled, with at least one thread waiting\n"
      "                   for ALU to write GRF/ACC register. Multiple stall reasons can\n"
      "                   qualify during the same cycle.\n"
      "sbid_count       - Number of cycles EU stalled, with at least one thread waiting\n"
      "                   for Scoreboard token to be available. Multiple stall reasons\n"
      "                   can qualify during the same cycle.\n"
      "sync_count       - Number of cycles EU stalled, with at least one thread waiting\n"
      "                   for Gateway to write Notify register. Multiple stall reasons\n"
      "                   can qualify during the same cycle.\n"
      "inst_fetch_count - Number of cycles EU stalled, with at least one thread waiting\n"
      "                   for Instruction Fetch. Multiple stall reasons can qualify\n"
      "                   during the same cycle.\n"
      "active_count     - Number of cycles no EU stalled.\n\n",
      progname);
}

enum intel_monitor_sampling_mode
{
   /* No sampling requested */
   INTEL_MONITOR_MODE_NONE = 0,

   /* Sample eu stalls */
   INTEL_MONITOR_MODE_EUSTALL = 1
};

int
main(int argc, char *argv[])
{
   int c, i;
   FILE *fd_out = stdout;
   uint64_t sample_period_us = 1000;
   bool success = true;

   void *cfg = NULL;
   bool (*do_sample)(void*) = NULL;
   void (*do_dump_results)(void*, FILE*) = NULL;
   void (*do_close)(void*) = NULL;

   enum intel_monitor_sampling_mode sample_mode = INTEL_MONITOR_MODE_NONE;
   const struct option intel_monitor_opts[] = {
      { "help",         no_argument,       NULL, 'h' },
      { "eustall",      no_argument,       NULL, 'e' },
      { "file",         required_argument, NULL, 'f' },
      { "sample_time",  required_argument, NULL, 't' },
      { NULL,           0,                 NULL, 0 }
   };

   struct intel_device_info devinfo;
   int drm_fd = get_drm_device(&devinfo);
   if (drm_fd < 0) {
      fprintf(stderr, "IMON: Error encountered while getting drm_device. err=%i\n",
              drm_fd);
      exit(EXIT_FAILURE);
   }

   /* Parse arguments */
   while ((c = getopt_long(argc, argv, "hef:t:", intel_monitor_opts, &i)) != -1) {
      switch (c) {
      case 'h':
         print_help(argv[0], stderr);
         return EXIT_SUCCESS;
      case 'e':
         sample_mode = INTEL_MONITOR_MODE_EUSTALL;
         break;
      case 'f':
         fd_out = fopen(optarg, "w");
         if (!fd_out) {
            fprintf(stderr, "IMON: Error opening output file '%s'\n", optarg);
            exit(EXIT_FAILURE);
         }
         break;
      case 't':
         sample_period_us = atoi(optarg);
         break;
      default:
         fprintf(stderr,
                 "IMON: Error. Unexpected parameter encountered '%c'.\n", c);
         exit(EXIT_FAILURE);
      }
   }

   /* Setup cfg based on selected sampling mode */
   if (sample_mode == INTEL_MONITOR_MODE_EUSTALL) {
      fprintf(stderr, "IMON: Setting up EU Stall Sampling\n");
      if (devinfo.ver < 20) {
         fprintf(stderr, "IMON: EU Stall Sampler supported on Xe2+ platforms.\n");
         exit(EXIT_FAILURE);
      }

      cfg = eustall_setup(drm_fd, &devinfo);

      do_sample = eustall_sample;
      do_dump_results = eustall_dump_results;
      do_close = eustall_close;
   } else {
      fprintf(stderr,
              "IMON: Error. No sampling mode set. Modes supported: ['-e']\n");
      exit(EXIT_FAILURE);
   }

   /* Main loop */
   fprintf(stderr, "IMON: Collecting samples. <Press any key to stop>\n");
   while (!kbhit()) {
      if (!success)
         exit(EXIT_FAILURE);

      success = do_sample(cfg);
      usleep(sample_period_us);
   }

   fprintf(stderr, "IMON: Dumping results\n");
   do_dump_results(cfg, fd_out);
   do_close(cfg);
   fprintf(stderr, "IMON: done\n");

   return EXIT_SUCCESS;
}
