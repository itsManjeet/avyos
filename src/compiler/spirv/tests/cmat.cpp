/*
 * Copyright 2025 Google LLC
 * SPDX-License-Identifier: MIT
 */

#include <gtest/gtest.h>

#include "util/memstream.h"

#include "helpers.h"

struct CooperativeMatrix : public spirv_test {
   void SetUp() {
     spirv_caps.CooperativeMatrixKHR = true;
     shader_text = NULL;
   }

   void TearDown() {
     free(shader_text);
   }

   // Prints the shader to a newly allocated null-terminated string, and
   // saves it into 'shader_text'. Returns NULL if resources were exhausted.
   char *to_string(nir_shader *shader) {
      free(shader_text);
      shader_text = NULL;
      size_t result_size = 0;
      struct u_memstream mem;
      if (u_memstream_open(&mem, &shader_text, &result_size)) {
         FILE * memf = u_memstream_get(&mem);
         nir_print_shader(shader, memf);
         u_memstream_close(&mem);
      }
      return shader_text;
   }

   char * shader_text;
};


TEST_F(CooperativeMatrix, VariableInitializer)
{
   // This module creates a cooperative matrix variable in Function storage
   // class with an initializer. spirv2nir should be able to print it,
   // including the initializer value.

   /*
   ; SPIR-V
   ; Version: 1.3
   ; Generator: Khronos SPIR-V Tools Assembler; 0
   ; Bound: 26
   ; Schema: 0
                  OpCapability Shader
                  OpCapability VulkanMemoryModel
                  OpCapability VulkanMemoryModelDeviceScope
                  OpCapability CooperativeMatrixKHR
                  OpExtension "SPV_KHR_vulkan_memory_model"
                  OpExtension "SPV_KHR_cooperative_matrix"
                  OpMemoryModel Logical Vulkan
                  OpEntryPoint GLCompute %main "main"
                  OpExecutionMode %main LocalSize 64 1 1
                  OpMemberName %buf_block_tint_explicit_layout 0 "inner"
                  OpName %buf_block_tint_explicit_layout "buf_block_tint_explicit_layout"
                  OpName %main "main"
                  OpName %m "m"
                  OpDecorate %_arr_float_uint_64 ArrayStride 4
                  OpMemberDecorate %buf_block_tint_explicit_layout 0 Offset 0
                  OpDecorate %buf_block_tint_explicit_layout Block
                  OpDecorate %5 DescriptorSet 0
                  OpDecorate %5 Binding 0
         %float = OpTypeFloat 32
          %uint = OpTypeInt 32 0
       %uint_64 = OpConstant %uint 64
   %_arr_float_uint_64 = OpTypeArray %float %uint_64
   %buf_block_tint_explicit_layout = OpTypeStruct %_arr_float_uint_64
   %_ptr_StorageBuffer_buf_block_tint_explicit_layout = OpTypePointer StorageBuffer %buf_block_tint_explicit_layout
             %5 = OpVariable %_ptr_StorageBuffer_buf_block_tint_explicit_layout StorageBuffer
          %void = OpTypeVoid
            %11 = OpTypeFunction %void
        %uint_3 = OpConstant %uint 3
        %uint_8 = OpConstant %uint 8
        %uint_2 = OpConstant %uint 2
            %15 = OpTypeCooperativeMatrixKHR %float %uint_3 %uint_8 %uint_8 %uint_2
   %_ptr_Function_15 = OpTypePointer Function %15
     %float_1_5 = OpConstant %float 1.5
            %18 = OpConstantComposite %15 %float_1_5
   %_ptr_StorageBuffer__arr_float_uint_64 = OpTypePointer StorageBuffer %_arr_float_uint_64
        %uint_0 = OpConstant %uint 0
   %_ptr_StorageBuffer_float = OpTypePointer StorageBuffer %float
          %main = OpFunction %void None %11
            %22 = OpLabel
             %m = OpVariable %_ptr_Function_15 Function %18
            %23 = OpLoad %15 %m None
            %24 = OpAccessChain %_ptr_StorageBuffer__arr_float_uint_64 %5 %uint_0
            %25 = OpAccessChain %_ptr_StorageBuffer_float %24 %uint_0
                  OpCooperativeMatrixStoreKHR %25 %23 %uint_0 %uint_8 NonPrivatePointer
                  OpReturn
                  OpFunctionEnd
    */
   static const uint32_t words[] = {
      0x07230203, 0x00010300, 0x00070000, 0x0000001a, 0x00000000, 0x00020011,
      0x00000001, 0x00020011, 0x000014e1, 0x00020011, 0x000014e2, 0x00020011,
      0x00001786, 0x0008000a, 0x5f565053, 0x5f52484b, 0x6b6c7576, 0x6d5f6e61,
      0x726f6d65, 0x6f6d5f79, 0x006c6564, 0x0008000a, 0x5f565053, 0x5f52484b,
      0x706f6f63, 0x74617265, 0x5f657669, 0x7274616d, 0x00007869, 0x0003000e,
      0x00000000, 0x00000003, 0x0005000f, 0x00000005, 0x00000001, 0x6e69616d,
      0x00000000, 0x00060010, 0x00000001, 0x00000011, 0x00000040, 0x00000001,
      0x00000001, 0x00050006, 0x00000002, 0x00000000, 0x656e6e69, 0x00000072,
      0x000a0005, 0x00000002, 0x5f667562, 0x636f6c62, 0x69745f6b, 0x655f746e,
      0x696c7078, 0x5f746963, 0x6f79616c, 0x00007475, 0x00040005, 0x00000001,
      0x6e69616d, 0x00000000, 0x00030005, 0x00000003, 0x0000006d, 0x00040047,
      0x00000004, 0x00000006, 0x00000004, 0x00050048, 0x00000002, 0x00000000,
      0x00000023, 0x00000000, 0x00030047, 0x00000002, 0x00000002, 0x00040047,
      0x00000005, 0x00000022, 0x00000000, 0x00040047, 0x00000005, 0x00000021,
      0x00000000, 0x00030016, 0x00000006, 0x00000020, 0x00040015, 0x00000007,
      0x00000020, 0x00000000, 0x0004002b, 0x00000007, 0x00000008, 0x00000040,
      0x0004001c, 0x00000004, 0x00000006, 0x00000008, 0x0003001e, 0x00000002,
      0x00000004, 0x00040020, 0x00000009, 0x0000000c, 0x00000002, 0x0004003b,
      0x00000009, 0x00000005, 0x0000000c, 0x00020013, 0x0000000a, 0x00030021,
      0x0000000b, 0x0000000a, 0x0004002b, 0x00000007, 0x0000000c, 0x00000003,
      0x0004002b, 0x00000007, 0x0000000d, 0x00000008, 0x0004002b, 0x00000007,
      0x0000000e, 0x00000002, 0x00071168, 0x0000000f, 0x00000006, 0x0000000c,
      0x0000000d, 0x0000000d, 0x0000000e, 0x00040020, 0x00000010, 0x00000007,
      0x0000000f, 0x0004002b, 0x00000006, 0x00000011, 0x3fc00000, 0x0004002c,
      0x0000000f, 0x00000012, 0x00000011, 0x00040020, 0x00000013, 0x0000000c,
      0x00000004, 0x0004002b, 0x00000007, 0x00000014, 0x00000000, 0x00040020,
      0x00000015, 0x0000000c, 0x00000006, 0x00050036, 0x0000000a, 0x00000001,
      0x00000000, 0x0000000b, 0x000200f8, 0x00000016, 0x0005003b, 0x00000010,
      0x00000003, 0x00000007, 0x00000012, 0x0005003d, 0x0000000f, 0x00000017,
      0x00000003, 0x00000000, 0x00050041, 0x00000013, 0x00000018, 0x00000005,
      0x00000014, 0x00050041, 0x00000015, 0x00000019, 0x00000018, 0x00000014,
      0x0006116a, 0x00000019, 0x00000017, 0x00000014, 0x0000000d, 0x00000020,
      0x000100fd, 0x00010038,
   };
   get_nir(sizeof(words) / sizeof(words[0]), words);
   ASSERT_TRUE(shader);

   char *text = to_string(shader);
   const char* expected = "m = { coopmat<float, SCOPE_SUBGROUP, 8, 8, ACCUMULATOR>(1.500000) }";
   EXPECT_NE(nullptr, strstr(text, expected)) << text;
}
