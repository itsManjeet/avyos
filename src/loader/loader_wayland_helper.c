/*
 * Copyright © 2022 Red Hat, Inc.
 *
 * Permission to use, copy, modify, distribute, and sell this software and its
 * documentation for any purpose is hereby granted without fee, provided that
 * the above copyright notice appear in all copies and that both that copyright
 * notice and this permission notice appear in supporting documentation, and
 * that the name of the copyright holders not be used in advertising or
 * publicity pertaining to distribution of the software without specific,
 * written prior permission.  The copyright holders make no representations
 * about the suitability of this software for any purpose.  It is provided "as
 * is" without express or implied warranty.
 *
 * THE COPYRIGHT HOLDERS DISCLAIM ALL WARRANTIES WITH REGARD TO THIS SOFTWARE,
 * INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS, IN NO
 * EVENT SHALL THE COPYRIGHT HOLDERS BE LIABLE FOR ANY SPECIAL, INDIRECT OR
 * CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE,
 * DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER
 * TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE
 * OF THIS SOFTWARE.
 */

#include <poll.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>

#include "presentation-time-client-protocol.h"

#include "util/list.h"
#include <util/os_time.h>
#include "util/perf/cpu_trace.h"

#include "loader_wayland_helper.h"

struct loader_wayland_presentation_feedback_data {
   struct loader_wayland_presentation *presentation;
   bool tracing;
   struct mesa_trace_flow flow;
   /* We store copies of name and id, since buffers can be
    * destroyed before feedback is serviced */
   char *buffer_name;
   uint32_t buffer_id;
   void *callback_data;
   struct wp_presentation_feedback *feedback;
   struct list_head link;
};

#ifndef HAVE_WL_DISPATCH_QUEUE_TIMEOUT
static int
wl_display_poll(struct wl_display *display,
                short int events,
                const struct timespec *timeout)
{
   int ret;
   struct pollfd pfd[1];
   struct timespec now;
   struct timespec deadline = {0};
   struct timespec result;
   struct timespec *remaining_timeout = NULL;

   if (timeout) {
      clock_gettime(CLOCK_MONOTONIC, &now);
      timespec_add(&deadline, &now, timeout);
   }

   pfd[0].fd = wl_display_get_fd(display);
   pfd[0].events = events;
   do {
      if (timeout) {
         clock_gettime(CLOCK_MONOTONIC, &now);
         timespec_sub_saturate(&result, &deadline, &now);
         remaining_timeout = &result;
      }
      ret = ppoll(pfd, 1, remaining_timeout, NULL);
   } while (ret == -1 && errno == EINTR);

   return ret;
}

int
wl_display_dispatch_queue_timeout(struct wl_display *display,
                                  struct wl_event_queue *queue,
                                  const struct timespec *timeout)
{
   int ret;
   struct timespec now;
   struct timespec deadline = {0};
   struct timespec result;
   struct timespec *remaining_timeout = NULL;

   if (timeout) {
      clock_gettime(CLOCK_MONOTONIC, &now);
      timespec_add(&deadline, &now, timeout);
   }

   if (wl_display_prepare_read_queue(display, queue) == -1)
      return wl_display_dispatch_queue_pending(display, queue);

   while (true) {
      ret = wl_display_flush(display);

      if (ret != -1 || errno != EAGAIN)
         break;

      if (timeout) {
         clock_gettime(CLOCK_MONOTONIC, &now);
         timespec_sub_saturate(&result, &deadline, &now);
         remaining_timeout = &result;
      }
      ret = wl_display_poll(display, POLLOUT, remaining_timeout);

      if (ret <= 0) {
         wl_display_cancel_read(display);
         return ret;
      }
   }

   /* Don't stop if flushing hits an EPIPE; continue so we can read any
    * protocol error that may have triggered it. */
   if (ret < 0 && errno != EPIPE) {
      wl_display_cancel_read(display);
      return -1;
   }

   while (true) {
      if (timeout) {
         clock_gettime(CLOCK_MONOTONIC, &now);
         timespec_sub_saturate(&result, &deadline, &now);
         remaining_timeout = &result;
      }

      ret = wl_display_poll(display, POLLIN, remaining_timeout);
      if (ret <= 0) {
         wl_display_cancel_read(display);
         break;
      }

      ret = wl_display_read_events(display);
      if (ret == -1)
         break;

      ret = wl_display_dispatch_queue_pending(display, queue);
      if (ret != 0)
         break;

      /* wl_display_dispatch_queue_pending can return 0 if we ended up reading
       * from WL fd, but there was no complete event to dispatch yet.
       * Try reading again. */
      if (wl_display_prepare_read_queue(display, queue) == -1)
         return wl_display_dispatch_queue_pending(display, queue);
   }

   return ret;
}
#endif

#ifndef HAVE_WL_CREATE_QUEUE_WITH_NAME
struct wl_event_queue *
wl_display_create_queue_with_name(struct wl_display *display, const char *name)
{
   return wl_display_create_queue(display);
}
#endif

int
loader_wayland_dispatch(struct wl_display *wl_display,
                        struct wl_event_queue *queue,
                        struct timespec *end_time)
{
   struct timespec current_time;
   struct timespec remaining_timeout;

   MESA_TRACE_FUNC();

   if (!end_time)
      return wl_display_dispatch_queue(wl_display, queue);

   clock_gettime(CLOCK_MONOTONIC, &current_time);
   timespec_sub_saturate(&remaining_timeout, end_time, &current_time);
   return wl_display_dispatch_queue_timeout(wl_display,
                                            queue,
                                            &remaining_timeout);
}

static char *
stringify_wayland_id(uint32_t id)
{
   char *out;

   if (asprintf(&out, "wl%d", id) < 0)
      return strdup("Wayland buffer");

   return out;
}

void
loader_wayland_wrap_buffer(struct loader_wayland_buffer *lwb,
                           struct wl_buffer *wl_buffer)
{
   lwb->buffer = wl_buffer;
   lwb->id = wl_proxy_get_id((struct wl_proxy *)wl_buffer);
   lwb->flow.id = 0;
   lwb->name = stringify_wayland_id(lwb->id);
}

void
loader_wayland_buffer_destroy(struct loader_wayland_buffer *lwb)
{
   wl_buffer_destroy(lwb->buffer);
   lwb->buffer = NULL;
   lwb->id = 0;
   lwb->flow.id = 0;
   free(lwb->name);
   lwb->name = NULL;
}

void
loader_wayland_buffer_set_flow(struct loader_wayland_buffer *lwb,
                               struct mesa_trace_flow *flow)
{
  lwb->flow = *flow;
}

bool
loader_wayland_wrap_surface(struct loader_wayland_surface *lws,
                            struct wl_surface *wl_surface,
                            struct wl_event_queue *queue)
{
   char *track_name;

   lws->surface = wl_surface;
   lws->wrapper = wl_proxy_create_wrapper(wl_surface);
   if (!lws->wrapper)
      return false;

   lws->id = wl_proxy_get_id((struct wl_proxy *)wl_surface);
   wl_proxy_set_queue((struct wl_proxy *)lws->wrapper, queue);

   asprintf(&track_name, "wl%d presentation", lws->id);
   lws->analytics.presentation_track_id = util_perfetto_new_track(track_name);
   free(track_name);

   asprintf(&lws->analytics.latency_str, "wl%d latency", lws->id);
   return true;
}

void
loader_wayland_surface_destroy(struct loader_wayland_surface *lws)
{
   if (!lws->wrapper)
      return;

   wl_proxy_wrapper_destroy(lws->wrapper);
   lws->surface = NULL;
   lws->id = 0;
   free(lws->analytics.latency_str);
   lws->analytics.latency_str = NULL;
}

static void
loader_wayland_trace_present(struct loader_wayland_presentation_feedback_data *fd,
                             uint64_t presentation_time)
{
   struct loader_wayland_surface *lws;
   UNUSED clockid_t clock;

   if (!fd->tracing || !util_perfetto_is_tracing_enabled())
      return;

   lws = fd->presentation->wayland_surface;
   clock = fd->presentation->clock_id;

   MESA_TRACE_SET_COUNTER(lws->analytics.latency_str,
                          (presentation_time - fd->flow.start_time) / 1000000.0);

   /* Close the previous image display interval first, if there is one. */
   if (lws->analytics.presenting) {
      MESA_TRACE_TIMESTAMP_END(fd->buffer_name,
                               lws->analytics.presentation_track_id,
                               clock, presentation_time);
   }

   lws->analytics.presenting = fd->buffer_id;

   MESA_TRACE_TIMESTAMP_BEGIN(fd->buffer_name,
                              lws->analytics.presentation_track_id,
                              fd->flow.id,
                              clock, presentation_time);
}

static void
presentation_handle_sync_output(void *data,
                                struct wp_presentation_feedback *feedback,
                                struct wl_output *output)
{
}

static void
feedback_fini(struct loader_wayland_presentation_feedback_data *fd)
{
   if (fd->tracing)
      free(fd->buffer_name);

   wp_presentation_feedback_destroy(fd->feedback);
   list_del(&fd->link);
   free(fd);
}

static void
presentation_handle_presented(void *data,
                              struct wp_presentation_feedback *feedback,
                              uint32_t tv_sec_hi, uint32_t tv_sec_lo,
                              uint32_t tv_nsec, uint32_t refresh,
                              uint32_t seq_hi, uint32_t seq_lo,
                              uint32_t flags)
{
   struct loader_wayland_presentation_feedback_data *fd = data;
   struct loader_wayland_presentation *pres = fd->presentation;
   struct timespec presentation_ts;
   uint64_t presentation_time;

   MESA_TRACE_FUNC_FLOW(&fd->flow);

   presentation_ts.tv_sec = ((uint64_t)tv_sec_hi << 32) + tv_sec_lo;
   presentation_ts.tv_nsec = tv_nsec;
   presentation_time = timespec_to_nsec(&presentation_ts);

   loader_wayland_trace_present(fd, presentation_time);
   if (pres->presented_callback) {
      pres->presented_callback(fd->callback_data, presentation_time, refresh);
   }

   feedback_fini(fd);
}

static void
presentation_handle_discarded(void *data,
                              struct wp_presentation_feedback *feedback)
{
   struct loader_wayland_presentation_feedback_data *fd = data;
   struct loader_wayland_presentation *pres = fd->presentation;

   MESA_TRACE_FUNC_FLOW(&fd->flow);

   if (pres->discarded_callback)
      pres->discarded_callback(fd->callback_data);

   feedback_fini(fd);
}

static const struct wp_presentation_feedback_listener
      pres_feedback_listener = {
   presentation_handle_sync_output,
   presentation_handle_presented,
   presentation_handle_discarded,
};

void
loader_wayland_wrap_presentation(struct loader_wayland_presentation *lpf,
                                 struct wp_presentation *wp_presentation,
                                 struct wl_event_queue *queue,
                                 clockid_t presentation_clock_id,
                                 struct loader_wayland_surface *lws,
                                 void (*presented_callback)(void *data, uint64_t pres_time, uint32_t refresh),
                                 void (*discarded_callback)(void *data),
                                 void (*teardown_callback)(void *data))

{
   lpf->presentation = wl_proxy_create_wrapper(wp_presentation);
   wl_proxy_set_queue((void *)lpf->presentation, queue);
   lpf->clock_id = presentation_clock_id;
   lpf->wayland_surface = lws;
   lpf->presented_callback = presented_callback;
   lpf->discarded_callback = discarded_callback;
   lpf->teardown_callback = teardown_callback;
   list_inithead(&lpf->outstanding_list);
}

void
loader_wayland_presentation_destroy(struct loader_wayland_presentation *pres)
{
   struct loader_wayland_presentation_feedback_data *fb, *tmp;

   if (pres->presentation == NULL)
      return;

   LIST_FOR_EACH_ENTRY_SAFE(fb, tmp, &pres->outstanding_list, link) {
      if (pres->teardown_callback)
         pres->teardown_callback(fb->callback_data);

      feedback_fini(fb);
   }
   wl_proxy_wrapper_destroy(pres->presentation);
   pres->presentation = NULL;
}

void
loader_wayland_presentation_feedback(struct loader_wayland_presentation *pres,
                                     struct loader_wayland_buffer *lwb,
                                     void *callback_data)
{
   struct loader_wayland_presentation_feedback_data *fd;
   bool tracing = false;

   if (!pres->presentation)
      return;

   tracing = util_perfetto_is_tracing_enabled();
   if (!pres->presented_callback &&
       !pres->discarded_callback &&
       !tracing)
      return;

   fd = malloc(sizeof *fd);
   fd->presentation = pres;
   fd->tracing = tracing;
   if (tracing) {
      fd->buffer_name = strdup(lwb->name);
      fd->buffer_id = lwb->id;
      fd->flow = lwb->flow;
   }
   fd->callback_data = callback_data;
   fd->feedback = wp_presentation_feedback(pres->presentation,
                                           pres->wayland_surface->wrapper);
   wp_presentation_feedback_add_listener(fd->feedback,
                                         &pres_feedback_listener,
                                         fd);
   list_add(&fd->link, &pres->outstanding_list);
}
