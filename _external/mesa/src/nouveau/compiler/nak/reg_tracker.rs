// Copyright © 2022 Collabora, Ltd.
// SPDX-License-Identifier: MIT

use crate::ir::*;

use std::ops::{Index, IndexMut, Range};

pub struct RegTracker<T> {
    reg: [T; 255],
    ureg: [T; 80],
    pred: [T; 7],
    upred: [T; 7],
    carry: [T; 1],
}

fn new_array_with<T, const N: usize>(f: &impl Fn() -> T) -> [T; N] {
    let mut v = Vec::new();
    for _ in 0..N {
        v.push(f());
    }
    v.try_into()
        .unwrap_or_else(|_| panic!("Array size mismatch"))
}

impl<T> RegTracker<T> {
    pub fn new_with(f: &impl Fn() -> T) -> Self {
        Self {
            reg: new_array_with(f),
            ureg: new_array_with(f),
            pred: new_array_with(f),
            upred: new_array_with(f),
            carry: new_array_with(f),
        }
    }

    pub fn for_each_instr_pred_mut(
        &mut self,
        instr: &Instr,
        mut f: impl FnMut(&mut T),
    ) {
        if let PredRef::Reg(reg) = &instr.pred.pred_ref {
            for i in &mut self[*reg] {
                f(i);
            }
        }
    }

    pub fn for_each_instr_src_mut(
        &mut self,
        instr: &Instr,
        mut f: impl FnMut(usize, &mut T),
    ) {
        for (i, src) in instr.srcs().iter().enumerate() {
            match &src.src_ref {
                SrcRef::Reg(reg) => {
                    for t in &mut self[*reg] {
                        f(i, t);
                    }
                }
                SrcRef::CBuf(CBufRef {
                    buf: CBuf::BindlessUGPR(reg),
                    ..
                }) => {
                    for t in &mut self[*reg] {
                        f(i, t);
                    }
                }
                _ => (),
            }
        }
    }

    pub fn for_each_instr_dst_mut(
        &mut self,
        instr: &Instr,
        mut f: impl FnMut(usize, &mut T),
    ) {
        for (i, dst) in instr.dsts().iter().enumerate() {
            if let Dst::Reg(reg) = dst {
                for t in &mut self[*reg] {
                    f(i, t);
                }
            }
        }
    }

    pub fn for_each_pred(&mut self, mut f: impl FnMut(&mut T)) {
        for p in &mut self.pred[..] {
            f(p);
        }
    }

    pub fn for_each_carry(&mut self, mut f: impl FnMut(&mut T)) {
        for c in &mut self.carry {
            f(c);
        }
    }
}

impl<T> Index<RegRef> for RegTracker<T> {
    type Output = [T];

    fn index(&self, reg: RegRef) -> &[T] {
        let range = reg.idx_range();
        let range = Range {
            start: usize::try_from(range.start).unwrap(),
            end: usize::try_from(range.end).unwrap(),
        };

        match reg.file() {
            RegFile::GPR => &self.reg[range],
            RegFile::UGPR => &self.ureg[range],
            RegFile::Pred => &self.pred[range],
            RegFile::UPred => &self.upred[range],
            RegFile::Carry => &self.carry[range],
            RegFile::Bar => &[], // Barriers have a HW scoreboard
            RegFile::Mem => panic!("Not a register"),
        }
    }
}

impl<T> IndexMut<RegRef> for RegTracker<T> {
    fn index_mut(&mut self, reg: RegRef) -> &mut [T] {
        let range = reg.idx_range();
        let range = Range {
            start: usize::try_from(range.start).unwrap(),
            end: usize::try_from(range.end).unwrap(),
        };

        match reg.file() {
            RegFile::GPR => &mut self.reg[range],
            RegFile::UGPR => &mut self.ureg[range],
            RegFile::Pred => &mut self.pred[range],
            RegFile::UPred => &mut self.upred[range],
            RegFile::Carry => &mut self.carry[range],
            RegFile::Bar => &mut [], // Barriers have a HW scoreboard
            RegFile::Mem => panic!("Not a register"),
        }
    }
}
