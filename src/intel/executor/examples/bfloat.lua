local r = execute {
  src=[[
    @id      g3

    mov(8)   g4<1>F    g3<1>UD                       {A@1};
    mov(8)   g5<1>F    g4.1<0,1,0>F                  {A@1};

    // Converting F to unpacked BF works, but as will be
    // illustrated, is not very useful.

    mov(8)   g10<2>BF  g4<1>F                        {A@1};

    // With exception of DPAS, instructions need to have at
    // least one non-BF operand and the operands must be packed.

    mov(8)   g11<1>UW  g10<2>UW                      {A@1};  // Pack it!
    add(8)   g12<1>BF  g11<1>BF  g4<1>F              {A@1};

    // Converting F to packed BF doesn't work, so add the value
    // to -0.0f instead.  This will preserve the NaN.  Note +0.0f
    // would not work since it doesn't preserve -0.0f!

    mov(8)   g20<1>UD  0x80000000UD                  {A@1}; // -0.0f.
    add(8)   g21<1>BF  g4<1>F    g20<1>F             {A@1}; // F -> BF.

    // Converting BF to F doesn't work, so for a packed source,
    // shift-left the bits to expand it into an UD instead.

    shl(8)   g30<1>UD  g21<1>UW  16UW                {A@1}; // BF -> F.

    mad(8)   g40<1>BF  g12<1>BF  g21<1>BF  g5<1>F    {A@1};
    add(8)   g41<1>BF  g40<1>BF  g30<1>F             {A@1};

    shl(8)   g42<1>UD  g41<1>UW  16UW                {A@1}; // BF -> F.

    mov(8)   g43<1>UD  g42<1>F                       {A@1};
    @write   g3        g43

    @eot
  ]]
}

expected = {[0] = 0, 4, 8, 12, 16, 20, 24, 28}

print("result")
dump(r, 8)

print("expected")
dump(expected, 8)

for i=0,7 do
  if r[i] ~= expected[i] then
    print("FAIL")
    return
  end
end

print("OK")
