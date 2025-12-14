/*
 * Copyright © 2021 Collabora, Ltd.
 * Author: Antonio Caggiano <antonio.caggiano@collabora.com>
 *
 * SPDX-License-Identifier: MIT
 */

#pragma once

struct pan_perf;

namespace pps {
class PanfrostDevice {
 public:
   PanfrostDevice(int fd);
   ~PanfrostDevice();

   PanfrostDevice(const PanfrostDevice &) = delete;
   PanfrostDevice &operator=(const PanfrostDevice &) = delete;

   PanfrostDevice(PanfrostDevice &&);
   PanfrostDevice &operator=(PanfrostDevice &&);

   int fd = -1;
};

class PanfrostPerf {
 public:
   PanfrostPerf(const PanfrostDevice &dev);
   ~PanfrostPerf();

   PanfrostPerf(const PanfrostPerf &) = delete;
   PanfrostPerf &operator=(const PanfrostPerf &) = delete;

   PanfrostPerf(PanfrostPerf &&);
   PanfrostPerf &operator=(PanfrostPerf &&);

   int enable() const;
   void disable() const;
   int dump() const;

   struct pan_perf *perf = nullptr;
};

} // namespace pps
