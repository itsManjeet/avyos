use crate::pipe::{context::PipeContext, screen::*};

use libc_rust_gen::close;
use mesa_rust_gen::*;

use std::sync::Arc;

pub struct FenceFd {
    pub fd: i32,
}

impl Drop for FenceFd {
    fn drop(&mut self) {
        unsafe {
            close(self.fd);
        }
    }
}

pub struct PipeFence {
    fence: *mut pipe_fence_handle,
    screen: Arc<PipeScreen>,
}

impl PipeFence {
    pub fn new(fence: *mut pipe_fence_handle, screen: &Arc<PipeScreen>) -> Self {
        Self {
            fence: fence,
            screen: Arc::clone(screen),
        }
    }

    pub fn gpu_signal(&self, ctx: &PipeContext) {
        debug_assert!(ctx.has_fence_server());
        unsafe {
            ctx.pipe().as_ref().fence_server_signal.unwrap()(ctx.pipe().as_ptr(), self.fence, 0);
        }
    }

    pub fn gpu_wait(&self, ctx: &PipeContext) {
        debug_assert!(ctx.has_fence_server());
        unsafe {
            ctx.pipe().as_ref().fence_server_sync.unwrap()(ctx.pipe().as_ptr(), self.fence, 0);
        }
    }

    pub fn wait(&self) {
        self.screen.fence_finish(self.fence);
    }
}

impl Drop for PipeFence {
    fn drop(&mut self) {
        self.screen.unref_fence(self.fence);
    }
}
