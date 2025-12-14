
#include "util/format/u_format.h"
#include "compiler/nir/nir_builder.h"
#include "v3d_compiler.h"

nir_def *
v3d_nir_get_tlb_color(nir_builder *b, struct v3d_compile *c, int rt, int sample)
{
        nir_def *color[4];
        for (int i = 0; i < 4; i++) {
                color[i] =
                        nir_load_tlb_color_brcm(b, 1, 32,
                                                nir_imm_int(b, rt),
                                                .base = sample,
                                                .component = i);
        }
        return nir_vec(b, color, 4);
}

static bool
lower_load_output(nir_builder *b, nir_intrinsic_instr *intr, void *data)
{
        if (intr->intrinsic != nir_intrinsic_load_output)
                return false;

        b->cursor = nir_before_instr(&intr->instr);

        struct v3d_compile *c = data;

        nir_io_semantics sem = nir_intrinsic_io_semantics(intr);
        unsigned rt = sem.location - FRAG_RESULT_DATA0;

        unsigned num_samples = c->fs_key->msaa ? V3D_MAX_SAMPLES : 1;
        assert(num_samples > 0);

        nir_def *sample_id = num_samples > 1 ? nir_load_sample_id(b) : NULL;
        nir_def *out = v3d_nir_get_tlb_color(b, c, rt, 0);

        /* If msaa, run through every sample and, if it matches the current sample
         * id, use it.
         */

        for (unsigned i = 1; i < num_samples; i++) {
                nir_def *is_cur_sample = nir_ieq_imm(b, sample_id, i);
                nir_def *val = v3d_nir_get_tlb_color(b, c, rt, i);
                out = nir_bcsel(b, is_cur_sample, val, out);
        }

        nir_def_replace(&intr->def, out);

        return true;
}

bool
v3d_nir_lower_load_output(nir_shader *s, struct v3d_compile *c)
{
        return  nir_shader_intrinsics_pass(s, lower_load_output,
                                           nir_metadata_control_flow,
                                           c);
}
