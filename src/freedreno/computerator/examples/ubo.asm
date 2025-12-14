@localsize 1, 1, 1
@ubo 4 1, 2, 3, 4 ; UBO 0
@ubo 4 5, 6, 7, 8 ; UBO 1
@buf 4
@invocationid(r0.x)
@branchstack 1
@earlypreamble

shps #main
getone #main

mova1 a1.x, (r)0
(ss)nop

; copy UBO 0 to c0
ldc.4.k.imm c[a1.x], 0, 0

(sy)(ss)shpe

main:

; load UBO 1 to r1
(jp)ldc.offset0.4.imm r1.x, 0, 1

(sy)(rpt3)add.u r1.x, (r)r1.x, (r)c0.x
(rpt2)nop

stib.b.untyped.1d.u32.4.imm r1.x, r0.x, 0
end
