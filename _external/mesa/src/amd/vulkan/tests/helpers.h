/*
 * Copyright © 2025 Valve Corporation
 *
 * SPDX-License-Identifier: MIT
 */

#ifndef RADV_TEST_HELPERS_H
#define RADV_TEST_HELPERS_H

#include <gtest/gtest.h>
#include <vulkan/vulkan.h>

#include <string>
#include <unordered_map>

#define FUNCTION_LIST                                                                                                  \
   ITEM(CreateInstance)                                                                                                \
   ITEM(DestroyInstance)                                                                                               \
   ITEM(EnumeratePhysicalDevices)                                                                                      \
   ITEM(GetPhysicalDeviceProperties2)                                                                                  \
   ITEM(GetPhysicalDeviceQueueFamilyProperties2)                                                                       \
   ITEM(GetPhysicalDeviceFormatProperties2)                                                                            \
   ITEM(CreateDevice)                                                                                                  \
   ITEM(DestroyDevice)                                                                                                 \
   ITEM(CreateShaderModule)                                                                                            \
   ITEM(DestroyShaderModule)                                                                                           \
   ITEM(CreateComputePipelines)                                                                                        \
   ITEM(DestroyPipeline)                                                                                               \
   ITEM(CreatePipelineLayout)                                                                                          \
   ITEM(DestroyPipelineLayout)                                                                                         \
   ITEM(GetPipelineExecutableStatisticsKHR)                                                                            \
   ITEM(GetPipelineExecutablePropertiesKHR)

class radv_test : public testing::Test {
public:
   radv_test();
   ~radv_test();

   void create_device();
   void destroy_device();

   void get_physical_device_properties2(VkPhysicalDeviceProperties2 *pdev_props);
   void get_physical_device_format_properties2(VkFormat format, VkFormatProperties2 *format_props);

   bool is_dedicated_sparse_queue_enabled();

   void create_compute_pipeline(uint32_t code_size, const uint32_t *code, VkPipelineCreateFlags flags = 0);
   void destroy_pipeline();

   uint64_t get_pipeline_hash(VkShaderStageFlags stage);

   void add_envvar(std::string name, std::string value)
   {
      setenv(name.c_str(), value.c_str(), 1);

      envvars.insert(std::make_pair<std::string, std::string>(std::move(name), std::move(value)));
   }

   void unset_envvars()
   {
      for (auto &envvar : envvars)
         unsetenv(envvar.first.c_str());
      envvars.clear();
   }

#define ITEM(n) PFN_vk##n n;
   FUNCTION_LIST
#undef ITEM

   VkInstance instance;
   VkPhysicalDevice physical_device;
   VkDevice device;

   VkPipelineLayout pipeline_layout;
   VkPipeline pipeline;

   std::unordered_map<std::string, std::string> envvars;
};

#endif /* RADV_TEST_HELPERS_H */
