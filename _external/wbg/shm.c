#include "shm.h"

#include <assert.h>
#include <errno.h>
#include <fcntl.h>
#include <unistd.h>

#include <sys/types.h>
#include <sys/mman.h>
#include <linux/memfd.h>

#include <tllist.h>

#define LOG_MODULE "shm"
#include "log.h"
#include "stride.h"

#if !defined(MAP_UNINITIALIZED)
 #define MAP_UNINITIALIZED 0
#endif

#if !defined(MFD_NOEXEC_SEAL)
 #define MFD_NOEXEC_SEAL 0
#endif

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

    /*
     * Older kernels reject MFD_NOEXEC_SEAL with EINVAL. Try first
     * *with* it, and if that fails, try again *without* it.
     */
    errno = 0;
    pool_fd = memfd_create(
        "wbg-wayland-shm-buffer-pool",
        MFD_CLOEXEC | MFD_ALLOW_SEALING | MFD_NOEXEC_SEAL);

    if (pool_fd < 0 && errno == EINVAL) {
        pool_fd = memfd_create(
            "wbg-wayland-shm-buffer-pool", MFD_CLOEXEC | MFD_ALLOW_SEALING);
    }

    if (pool_fd == -1) {
        LOG_ERRNO("failed to create SHM backing memory file");
        goto err;
    }

    /* Total size */
    const uint32_t stride = stride_for_format_and_width(PIXMAN_x8r8g8b8, width);
    size = stride * height;
    if (ftruncate(pool_fd, size) == -1) {
        LOG_ERRNO("failed to truncate SHM pool");
        goto err;
    }

    mmapped = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, pool_fd, 0);
    if (mmapped == MAP_FAILED) {
        LOG_ERR("failed to mmap SHM backing memory file");
        goto err;
    }

    /* Seal file - we no longer allow any kind of resizing */
    /* TODO: wayland mmaps(PROT_WRITE), for some unknown reason, hence we cannot use F_SEAL_FUTURE_WRITE */
    if (fcntl(pool_fd, F_ADD_SEALS,
              F_SEAL_GROW | F_SEAL_SHRINK | /*F_SEAL_FUTURE_WRITE |*/ F_SEAL_SEAL) < 0)
    {
        LOG_ERRNO("failed to seal SHM backing memory file");
        /* This is not a fatal error */
    }

    pool = wl_shm_create_pool(shm, pool_fd, size);
    if (pool == NULL) {
        LOG_ERR("failed to create SHM pool");
        goto err;
    }

    buf = wl_shm_pool_create_buffer(
        pool, 0, width, height, stride, WL_SHM_FORMAT_XRGB8888);
    if (buf == NULL) {
        LOG_ERR("failed to create SHM buffer");
        goto err;
    }

    /* We use the entire pool for our single buffer */
    wl_shm_pool_destroy(pool); pool = NULL;
    close(pool_fd); pool_fd = -1;

    pix = pixman_image_create_bits_no_clear(
        PIXMAN_x8r8g8b8, width, height, mmapped, stride);
    if (pix == NULL) {
        LOG_ERR("failed to create pixman image");
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
