use crate::api::icd::*;
use crate::core::context::*;
use crate::core::device::*;
use crate::core::event::*;
use crate::core::kernel::*;
use crate::core::platform::*;
use crate::impl_cl_type_trait;

use mesa_rust::compiler::nir::NirShader;
use mesa_rust::pipe::context::PipeContext;
use mesa_rust::pipe::context::PipeContextPrio;
use mesa_rust_gen::*;
use mesa_rust_util::properties::*;
use rusticl_opencl_gen::*;

use std::cell::RefCell;
use std::cmp;
use std::ffi::c_void;
use std::mem;
use std::mem::ManuallyDrop;
use std::ops::Deref;
use std::ptr;
use std::ptr::NonNull;
use std::sync::mpsc;
use std::sync::Arc;
use std::sync::Mutex;
use std::sync::Weak;
use std::thread;
use std::thread::JoinHandle;

struct CSOWrapper<'a> {
    ctx: &'a PipeContext,
    cso: NonNull<c_void>,
}

impl<'a> CSOWrapper<'a> {
    fn new(ctx: &QueueContext<'a>, nir: &NirShader) -> Option<CSOWrapper<'a>> {
        Some(Self {
            ctx: ctx.ctx,
            cso: NonNull::new(ctx.create_compute_state(nir, nir.shared_size()))?,
        })
    }
}

impl Drop for CSOWrapper<'_> {
    fn drop(&mut self) {
        self.ctx.delete_compute_state(self.cso.as_ptr());
    }
}

struct QueueKernelState<'a> {
    builds: Option<Arc<NirKernelBuilds>>,
    variant: NirKernelVariant,
    cso: Option<CSOWrapper<'a>>,
}

/// State tracking wrapper for [PipeContext]
///
/// Used for tracking bound GPU state to lower CPU overhead and centralize state tracking
pub struct QueueContext<'a> {
    ctx: &'a PipeContext,
    pub dev: &'static Device,
    use_stream: bool,
    kernel_state: RefCell<QueueKernelState<'a>>,
}

impl QueueContext<'_> {
    // TODO: figure out how to make it &mut self without causing tons of borrowing issues.
    pub fn bind_kernel(
        &self,
        builds: &Arc<NirKernelBuilds>,
        variant: NirKernelVariant,
    ) -> CLResult<()> {
        // this should never panic, but you never know.
        let mut state = self.kernel_state.borrow_mut();

        // If we already set the CSO then we don't have to bind again.
        if let Some(stored_builds) = &state.builds {
            if Arc::ptr_eq(stored_builds, builds) && state.variant == variant {
                return Ok(());
            }
        }

        let nir_kernel_build = &builds[variant];
        match nir_kernel_build.nir_or_cso() {
            // SAFETY: We keep the cso alive until a new one is set.
            KernelDevStateVariant::Cso(cso) => unsafe {
                cso.bind_to_ctx(self);
            },
            // TODO: We could cache the cso here.
            KernelDevStateVariant::Nir(nir) => {
                let cso = CSOWrapper::new(self, nir).ok_or(CL_OUT_OF_HOST_MEMORY)?;
                unsafe {
                    self.bind_compute_state(cso.cso.as_ptr());
                }
                state.cso.replace(cso);
            }
        };

        // We can only store the new builds after we bound the new cso otherwise we might drop it
        // too early.
        state.builds = Some(Arc::clone(builds));
        state.variant = variant;

        Ok(())
    }

    pub fn update_cb0(&self, data: &[u8]) -> CLResult<()> {
        // only update if we actually bind data
        if !data.is_empty() {
            if self.use_stream {
                if !self.ctx.set_constant_buffer_stream(0, data) {
                    return Err(CL_OUT_OF_RESOURCES);
                }
            } else {
                self.ctx.set_constant_buffer(0, data);
            }
        }
        Ok(())
    }
}

// This should go once we moved all state tracking into QueueContext
impl Deref for QueueContext<'_> {
    type Target = PipeContext;

    fn deref(&self) -> &Self::Target {
        self.ctx
    }
}

impl Drop for QueueContext<'_> {
    fn drop(&mut self) {
        self.set_constant_buffer(0, &[]);
        self.ctx.clear_shader_images(self.dev.caps.max_write_images);
        if self.kernel_state.get_mut().builds.is_some() {
            // SAFETY: We simply unbind here. The bound cso will only be dropped at the end of this
            //         drop handler.
            unsafe {
                self.ctx.bind_compute_state(ptr::null_mut());
            }
        }
    }
}

/// The main purpose of this type is to be able to create the context outside of the worker thread
/// to report back any sort of allocation failures early.
struct SendableQueueContext {
    // need to use ManuallyDrop so we can recycle the context without cloning
    ctx: ManuallyDrop<PipeContext>,
    dev: &'static Device,
}

impl SendableQueueContext {
    fn new(device: &'static Device, prio: cl_queue_priority_khr) -> CLResult<Self> {
        let prio = if prio & CL_QUEUE_PRIORITY_HIGH_KHR != 0 {
            PipeContextPrio::High
        } else if prio & CL_QUEUE_PRIORITY_MED_KHR != 0 {
            PipeContextPrio::Med
        } else {
            PipeContextPrio::Low
        };

        Ok(Self {
            ctx: ManuallyDrop::new(device.create_context(prio).ok_or(CL_OUT_OF_HOST_MEMORY)?),
            dev: device,
        })
    }

    /// The returned value can be used to execute operation on the wrapped context in a safe manner.
    fn ctx(&self) -> QueueContext<'_> {
        QueueContext {
            ctx: &self.ctx,
            kernel_state: RefCell::new(QueueKernelState {
                builds: None,
                variant: NirKernelVariant::Default,
                cso: None,
            }),
            dev: self.dev,
            use_stream: self.dev.prefers_real_buffer_in_cb0(),
        }
    }
}

impl Drop for SendableQueueContext {
    fn drop(&mut self) {
        let ctx = unsafe { ManuallyDrop::take(&mut self.ctx) };
        self.dev.recycle_context(ctx);
    }
}

struct QueueState {
    pending: Vec<Arc<Event>>,
    last: Weak<Event>,
    // `Sync` on `Sender` was stabilized in 1.72, until then, put it into our Mutex.
    // see https://github.com/rust-lang/rust/commit/5f56956b3c7edb9801585850d1f41b0aeb1888ff
    chan_in: mpsc::Sender<Vec<Arc<Event>>>,
}

pub struct Queue {
    pub base: CLObjectBase<CL_INVALID_COMMAND_QUEUE>,
    pub context: Arc<Context>,
    pub device: &'static Device,
    pub props: cl_command_queue_properties,
    pub props_v2: Properties<cl_queue_properties>,
    state: Mutex<QueueState>,
    _thrd: JoinHandle<()>,
}

/// Wrapper around Event to set it to an error state on drop. This is useful for dealing with panics
/// inside the worker thread, so all not yet processed events will bet put into an error state, so
/// Event::wait won't infinitely spin on the status to change.
#[repr(transparent)]
struct QueueEvent(Arc<Event>);

impl QueueEvent {
    fn call(self, ctx: &QueueContext) -> (cl_int, Arc<Event>) {
        let res = self.0.call(ctx);
        (res, self.into_inner())
    }

    fn deps(&self) -> &[Arc<Event>] {
        &self.0.deps
    }

    fn into_inner(self) -> Arc<Event> {
        // SAFETY: QueueEvent is transparent wrapper so it's safe to transmute the value. We want to
        //         prevent drop from running on it. Alternatively we could use ManuallyDrop, but
        //         that also requires an unsafe call (ManuallyDrop::take).
        unsafe { mem::transmute(self) }
    }

    fn has_same_queue_as(&self, ev: &Event) -> bool {
        match (&self.0.queue, &ev.queue) {
            (Some(a), Some(b)) => Weak::ptr_eq(a, b),
            _ => false,
        }
    }

    fn set_user_status(self, status: cl_int) {
        self.into_inner().set_user_status(status);
    }
}

impl Drop for QueueEvent {
    fn drop(&mut self) {
        // Make sure the status isn't an error or success.
        debug_assert!(self.0.status() > CL_RUNNING as cl_int);
        self.0.set_user_status(CL_OUT_OF_HOST_MEMORY)
    }
}

/// Wrapper around received events, so they automatically go into an error state whenever the queue
/// thread panics. We should use panic::catch_unwind, but that requires things to be UnwindSafe and
/// that's not easily doable.
#[repr(transparent)]
struct QueueEvents(Vec<QueueEvent>);

impl QueueEvents {
    fn new(events: Vec<Arc<Event>>) -> Self {
        Self(events.into_iter().map(QueueEvent).collect())
    }
}

impl IntoIterator for QueueEvents {
    type Item = QueueEvent;
    type IntoIter = <Vec<QueueEvent> as IntoIterator>::IntoIter;

    fn into_iter(self) -> Self::IntoIter {
        self.0.into_iter()
    }
}

impl_cl_type_trait!(cl_command_queue, Queue, CL_INVALID_COMMAND_QUEUE);

fn flush_events(evs: &mut Vec<Arc<Event>>, pipe: &PipeContext) -> cl_int {
    if !evs.is_empty() {
        pipe.flush().wait();
        if pipe.device_reset_status() != pipe_reset_status::PIPE_NO_RESET {
            // if the context reset while executing, simply put all events into error state.
            evs.drain(..)
                .for_each(|e| e.set_user_status(CL_OUT_OF_RESOURCES));
            return CL_OUT_OF_RESOURCES;
        } else {
            evs.drain(..).for_each(|e| e.signal());
        }
    }

    CL_SUCCESS as cl_int
}

impl Queue {
    pub fn new(
        context: Arc<Context>,
        device: &'static Device,
        props: cl_command_queue_properties,
        props_v2: Properties<cl_queue_properties>,
    ) -> CLResult<Arc<Queue>> {
        // If CL_QUEUE_PRIORITY_KHR is not specified, the default priority CL_QUEUE_PRIORITY_MED_KHR
        // is used.
        let mut prio = *props_v2
            .get(&CL_QUEUE_PRIORITY_KHR.into())
            .unwrap_or(&CL_QUEUE_PRIORITY_MED_KHR.into())
            as cl_queue_priority_khr;

        // Fallback to the default priority if it's not supported by the device.
        if device.context_priority_supported() & prio == 0 {
            prio = CL_QUEUE_PRIORITY_MED_KHR;
        }

        // we assume that memory allocation is the only possible failure. Any other failure reason
        // should be detected earlier (e.g.: checking for CAPs).
        let ctx = SendableQueueContext::new(device, prio)?;
        let (tx_q, rx_t) = mpsc::channel::<Vec<Arc<Event>>>();
        Ok(Arc::new(Self {
            base: CLObjectBase::new(RusticlTypes::Queue),
            context: context,
            device: device,
            props: props,
            props_v2: props_v2,
            state: Mutex::new(QueueState {
                pending: Vec::new(),
                last: Weak::new(),
                chan_in: tx_q,
            }),
            _thrd: thread::Builder::new()
                .name("rusticl queue thread".into())
                .spawn(move || {
                    // Track the error of all executed events. This is only needed for in-order
                    // queues, so for out of order we'll need to update this.
                    // Also, the OpenCL specification gives us enough freedom to do whatever we want
                    // in case of any event running into an error while executing:
                    //
                    //   Unsuccessful completion results in abnormal termination of the command
                    //   which is indicated by setting the event status to a negative value. In this
                    //   case, the command-queue associated with the abnormally terminated command
                    //   and all other command-queues in the same context may no longer be available
                    //   and their behavior is implementation-defined.
                    //
                    // TODO: use pipe_context::set_device_reset_callback to get notified about gone
                    //       GPU contexts
                    let mut last_err = CL_SUCCESS as cl_int;
                    let ctx = ctx.ctx();
                    let mut flushed = Vec::new();
                    loop {
                        debug_assert!(flushed.is_empty());

                        let Ok(new_events) = rx_t.recv() else {
                            break;
                        };

                        let new_events = QueueEvents::new(new_events);
                        for e in new_events {
                            // If we hit any deps from another queue, flush so we don't risk a dead
                            // lock.
                            if e.deps().iter().any(|ev| !e.has_same_queue_as(ev)) {
                                let dep_err = flush_events(&mut flushed, &ctx);
                                last_err = cmp::min(last_err, dep_err);
                            }

                            // check if any dependency has an error
                            for dep in e.deps() {
                                // We have to wait on user events or events from other queues.
                                let dep_err = if dep.is_user() || !e.has_same_queue_as(dep) {
                                    dep.wait()
                                } else {
                                    dep.status()
                                };

                                last_err = cmp::min(last_err, dep_err);
                            }

                            if last_err < 0 {
                                // If a dependency failed, fail this event as well.
                                e.set_user_status(last_err);
                                continue;
                            }

                            // if there is an execution error don't bother signaling it as the  context
                            // might be in a broken state. How queues behave after any event hit an
                            // error is entirely implementation defined.
                            let (err, e) = e.call(&ctx);
                            last_err = err;
                            if last_err < 0 {
                                continue;
                            }

                            if e.is_user() {
                                // On each user event we flush our events as application might
                                // wait on them before signaling user events.
                                last_err = flush_events(&mut flushed, &ctx);

                                if last_err >= 0 {
                                    // Wait on user events as they are synchronization points in the
                                    // application's control.
                                    e.wait();
                                }
                            } else if Platform::dbg().sync_every_event {
                                flushed.push(e);
                                last_err = flush_events(&mut flushed, &ctx);
                            } else {
                                flushed.push(e);
                            }
                        }

                        let flush_err = flush_events(&mut flushed, &ctx);
                        last_err = cmp::min(last_err, flush_err);
                    }
                })
                .unwrap(),
        }))
    }

    pub fn queue(&self, e: Arc<Event>) {
        if self.is_profiling_enabled() {
            e.set_time(EventTimes::Queued, self.device.screen().get_timestamp());
        }
        self.state.lock().unwrap().pending.push(e);
    }

    pub fn flush(&self, wait: bool) -> CLResult<()> {
        let mut state = self.state.lock().unwrap();
        let events = mem::take(&mut state.pending);
        let mut queues = Event::deep_unflushed_queues(&events);

        // Update last if and only if we get new events, this prevents breaking application code
        // doing things like `clFlush(q); clFinish(q);`
        if let Some(last) = events.last() {
            state.last = Arc::downgrade(last);

            // This should never ever error, but if it does return an error
            state
                .chan_in
                .send(events)
                .map_err(|_| CL_OUT_OF_HOST_MEMORY)?;
        }

        let last = wait.then(|| state.last.clone());

        // We have to unlock before actually flushing otherwise we'll run into dead locks when a
        // queue gets flushed concurrently.
        drop(state);

        // We need to flush out other queues implicitly and this _has_ to happen after taking the
        // pending events, otherwise we'll risk dead locks when waiting on events.
        queues.remove(self);
        for q in queues {
            q.flush(false)?;
        }

        if let Some(last) = last {
            // Waiting on the last event is good enough here as the queue will process it in order
            // It's not a problem if the weak ref is invalid as that means the work is already done
            // and waiting isn't necessary anymore.
            //
            // We also ignore any error state of events as it's the callers responsibility to check
            // for it if it cares.
            last.upgrade().map(|e| e.wait());
        }
        Ok(())
    }

    pub fn is_profiling_enabled(&self) -> bool {
        (self.props & (CL_QUEUE_PROFILING_ENABLE as u64)) != 0
    }
}

impl Drop for Queue {
    fn drop(&mut self) {
        // When reaching this point the queue should have been flushed already, but do it here once
        // again just to be sure.
        let _ = self.flush(false);
    }
}
