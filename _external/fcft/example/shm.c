#include "shm.h"

#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <assert.h>
#include <errno.h>

#include <sys/types.h>
#include <sys/mman.h>

#include <tllist.h>

#include "stride.h"

static void
buffer_destroy(struct buffer *buf)
{
    pixman_image_unref(buf->pix);
    wl_buffer_destroy(buf->wl_buf);
    munmap(buf->mmapped, buf->size);
    free(buf);
}

static void
buffer_release(void *data, struct wl_buffer *wl_buffer)
{
    struct buffer *buffer = data;
    buffer_destroy(buffer);
}

static const struct wl_buffer_listener buffer_listener = {
    .release = &buffer_release,
};

struct buffer *
shm_get_buffer(struct wl_shm *shm, int width, int height, unsigned long cookie)
{
    /*
     * 1. open a memory backed "file" with memfd_create()
     * 2. mmap() the memory file, to be used by the pixman image
     * 3. create a wayland shm buffer for the same memory file
     *
     * The pixman image and the wayland buffer are now sharing memory.
     */

    int pool_fd = -1;
    void *mmapped = NULL;
    size_t size = 0;

    struct wl_shm_pool *pool = NULL;
    struct wl_buffer *buf = NULL;
    pixman_image_t *pix = NULL;

    /* Backing memory for SHM */
#if defined(MEMFD_CREATE)
    pool_fd = memfd_create("fcft-example-wayland-shm-buffer-pool",
                           MFD_CLOEXEC | MFD_ALLOW_SEALING);
#elif defined(__FreeBSD__)
    // memfd_create on FreeBSD 13 is SHM_ANON without sealing support
    pool_fd = shm_open(SHM_ANON, O_RDWR | O_CLOEXEC, 0600);
#else
    char name[] = "/tmp/fcft-example-wayland-shm-buffer-pool-XXXXXX";
    pool_fd = mkostemp(name, O_CLOEXEC);
    unlink(name);
#endif

    /* Total size */
    const uint32_t stride = stride_for_format_and_width(PIXMAN_x8r8g8b8, width);
    size = stride * height;
    if (ftruncate(pool_fd, size) == -1) {
        fprintf(
            stderr,
            "error: failed to truncate SHM pool: %s\n", strerror(errno));
        goto err;
    }

    mmapped = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, pool_fd, 0);
    if (mmapped == MAP_FAILED) {
        fprintf(
            stderr,
            "error: failed to mmap SHM backing memory file: %s\n",
            strerror(errno));
        goto err;
    }

    pool = wl_shm_create_pool(shm, pool_fd, size);
    if (pool == NULL) {
        fprintf(
            stderr, "error: failed to create SHM pool: %s\n", strerror(errno));
        goto err;
    }

    buf = wl_shm_pool_create_buffer(
        pool, 0, width, height, stride, WL_SHM_FORMAT_ARGB8888);
    if (buf == NULL) {
        fprintf(
            stderr,
            "error: failed to create SHM buffer: %s\n", strerror(errno));
        goto err;
    }

    /* We use the entire pool for our single buffer */
    wl_shm_pool_destroy(pool); pool = NULL;
    close(pool_fd); pool_fd = -1;

    pix = pixman_image_create_bits_no_clear(
        PIXMAN_a8r8g8b8, width, height, mmapped, stride);
    if (pix == NULL) {
        fprintf(stderr, "error: failed to create pixman image\n");
        goto err;
    }

    struct buffer *buffer = malloc(sizeof(*buffer));
    *buffer = (struct buffer){
        .width = width,
        .height = height,
        .stride = stride,
        .cookie = cookie,
        .busy = true,
        .size = size,
        .mmapped = mmapped,
        .wl_buf = buf,
        .pix = pix,
    };

    wl_buffer_add_listener(buffer->wl_buf, &buffer_listener, buffer);
    return buffer;

err:
    if (pix != NULL)
        pixman_image_unref(pix);
    if (buf != NULL)
        wl_buffer_destroy(buf);
    if (pool != NULL)
        wl_shm_pool_destroy(pool);
    if (pool_fd != -1)
        close(pool_fd);
    if (mmapped != NULL)
        munmap(mmapped, size);

    return NULL;
}

void
shm_buffer_destroy(struct buffer *buf)
{
    buffer_destroy(buf);
}
