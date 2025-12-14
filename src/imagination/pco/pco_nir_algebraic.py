# Copyright © 2025 Imagination Technologies Ltd.
# SPDX-License-Identifier: MIT

import argparse
import sys
import math

a = 'a'
b = 'b'

lower_algebraic = []
lower_algebraic_late = []

lower_scmp = [
   # Float comparisons + bool conversions.
   (('b2f32', ('flt', a, b)), ('slt', a, 'b@32'), '!options->lower_scmp'),
   (('b2f32', ('fge', a, b)), ('sge', a, 'b@32'), '!options->lower_scmp'),
   (('b2f32', ('feq', a, b)), ('seq', a, 'b@32'), '!options->lower_scmp'),
   (('b2f32', ('fneu', a, b)), ('sne', a, 'b@32'), '!options->lower_scmp'),

   # Float comparisons + bool conversions via bcsel.
   (('bcsel@32', ('flt', a, b), 1.0, 0.0), ('slt', a, 'b@32'), '!options->lower_scmp'),
   (('bcsel@32', ('fge', a, b), 1.0, 0.0), ('sge', a, 'b@32'), '!options->lower_scmp'),
   (('bcsel@32', ('feq', a, b), 1.0, 0.0), ('seq', a, 'b@32'), '!options->lower_scmp'),
   (('bcsel@32', ('fneu', a, b), 1.0, 0.0), ('sne', a, 'b@32'), '!options->lower_scmp'),
]

lower_algebraic_late.extend(lower_scmp)

# TODO: core-specific info.
params=[]

def run():
    import nir_algebraic  # pylint: disable=import-error
    print('#include "pco_internal.h"')
    print(nir_algebraic.AlgebraicPass('pco_nir_lower_algebraic', lower_algebraic, params=params).render())
    print(nir_algebraic.AlgebraicPass('pco_nir_lower_algebraic_late', lower_algebraic_late, params=params).render())

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-p', '--import-path', required=True)
    args = parser.parse_args()
    sys.path.insert(0, args.import_path)
    run()

if __name__ == '__main__':
    main()
