#!/usr/bin/env python3

import argparse
import math
import sys


# Note: we use a pure gamma 2.2 function, rather than the piece-wise
# sRGB transfer function, since that is what all compositors do.

def srgb_to_linear(f: float) -> float:
    assert(f >= 0 and f <= 1.0)
    return math.pow(f, 2.2)


def linear_to_srgb(f: float) -> float:
    return math.pow(f, 1 / 2.2)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('c_output', type=argparse.FileType('w'))
    parser.add_argument('h_output', type=argparse.FileType('w'))
    opts = parser.parse_args()

    linear_table: list[int] = []

    for i in range(256):
        linear_table.append(int(srgb_to_linear(float(i) / 255) * 65535 + 0.5))


    opts.h_output.write("#pragma once\n")
    opts.h_output.write("#include <stdint.h>\n")
    opts.h_output.write("\n")
    opts.h_output.write('/* 8-bit input, 16-bit output */\n')
    opts.h_output.write("extern const uint16_t srgb_decode_8_to_16_table[256];")

    opts.h_output.write('\n')
    opts.h_output.write('static inline uint16_t\n')
    opts.h_output.write('srgb_decode_8_to_16(uint8_t v)\n')
    opts.h_output.write('{\n')
    opts.h_output.write('    return srgb_decode_8_to_16_table[v];\n')
    opts.h_output.write('}\n')

    opts.h_output.write('\n')
    opts.h_output.write('/* 8-bit input, 8-bit output */\n')
    opts.h_output.write("extern const uint8_t srgb_decode_8_to_8_table[256];\n")

    opts.h_output.write('\n')
    opts.h_output.write('static inline uint8_t\n')
    opts.h_output.write('srgb_decode_8_to_8(uint8_t v)\n')
    opts.h_output.write('{\n')
    opts.h_output.write('    return srgb_decode_8_to_8_table[v];\n')
    opts.h_output.write('}\n')

    opts.c_output.write('#include "srgb.h"\n')
    opts.c_output.write('\n')

    opts.c_output.write("const uint16_t srgb_decode_8_to_16_table[256] = {\n")
    for i in range(256):
        opts.c_output.write(f'    {linear_table[i]},\n')
    opts.c_output.write('};\n')

    opts.c_output.write("const uint8_t srgb_decode_8_to_8_table[256] = {\n")
    for i in range(256):
        opts.c_output.write(f'    {linear_table[i] >> 8},\n')
    opts.c_output.write('};\n')


if __name__ == '__main__':
    sys.exit(main())
