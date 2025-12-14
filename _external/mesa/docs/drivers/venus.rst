Virtio-GPU Venus
================

Venus is a Virtio-GPU protocol for Vulkan command serialization.  The protocol
definition and codegen are hosted at `venus-protocol
<https://gitlab.freedesktop.org/virgl/venus-protocol>`__.  The renderer is
hosted at `virglrenderer
<https://gitlab.freedesktop.org/virgl/virglrenderer>`__.

Requirements
------------

The Venus renderer requires

- Linux platform
   - Vulkan 1.1
   - :ext:`VK_KHR_external_memory_fd`
- Android platform
   - Vulkan 1.1
   - :ext:`VK_EXT_external_memory_dma_buf`
   - :ext:`VK_EXT_image_drm_format_modifier`
   - :ext:`VK_EXT_queue_family_foreign`

from the host driver.  However, it violates the spec and relies on
implementation-defined behaviors to support ``vkMapMemory`` (see `below
<#vk-memory-property-host-visible-bit>`__).  It is not expected to work on all
drivers meeting the requirements.  It has only been tested with:

- ANV 21.1 or later
- RADV 21.1 or later
   - Note: you need 6.13+ kernel that already has
     `KVM: Stop grabbing references to PFNMAP'd pages
     <https://lore.kernel.org/all/20241010182427.1434605-1-seanjc@google.com/>`__.
   - Note: for dGPU paired with Intel CPU, you need 6.11+ kernel patched with
     `KVM: VMX: Always honor guest PAT on CPUs that support self-snoop
     <https://lore.kernel.org/all/20240309010929.1403984-6-seanjc@google.com/>`__,
     or 6.16+ kernel with VMM to opt-out ``KVM_X86_QUIRK_IGNORE_GUEST_PAT`` (QEMU
     request is `here <https://gitlab.com/qemu-project/qemu/-/issues/2943>`__).
- Turnip 22.0 or later
- PanVK 25.1 or later
- Lavapipe 22.1 or later
- Mali (Proprietary) r32p0 or later
- NVIDIA (Proprietary) 570.86 or later
   - Note: if paired with Intel CPU, you need 6.11+ kernel patched with
     `KVM: VMX: Always honor guest PAT on CPUs that support self-snoop
     <https://lore.kernel.org/all/20240309010929.1403984-6-seanjc@google.com/>`__,
     or 6.16+ kernel with VMM to opt-out ``KVM_X86_QUIRK_IGNORE_GUEST_PAT`` (QEMU
     request is `here <https://gitlab.com/qemu-project/qemu/-/issues/2943>`__).

The Venus driver requires supports for

- ``VIRTGPU_PARAM_3D_FEATURES``
- ``VIRTGPU_PARAM_CAPSET_QUERY_FIX``
- ``VIRTGPU_PARAM_RESOURCE_BLOB``
- ``VIRTGPU_PARAM_HOST_VISIBLE``
- ``VIRTGPU_PARAM_CONTEXT_INIT``

from the virtio-gpu kernel driver, unless vtest is used.  That usually means
the guest kernel should be at least 5.16 or have the parameters back ported,
paired with hypervisors such as `crosvm <https://crosvm.dev>`__, or `QEMU
<https://www.qemu.org>`__.

vtest
-----

The simplest way to test Venus is to use virglrenderer's vtest server.  To
build virglrenderer with Venus support and to start the vtest server,

.. code-block:: sh

    $ git clone https://gitlab.freedesktop.org/virgl/virglrenderer.git
    $ cd virglrenderer
    $ meson out -Dvenus=true
    $ meson compile -C out
    $ meson devenv -C out
    $ ./vtest/virgl_test_server --venus
    $ exit

In another shell,

.. code-block:: sh

    $ export VK_DRIVER_FILES=<path-to-virtio_icd.x86_64.json>
    $ export VN_DEBUG=vtest
    $ vulkaninfo
    $ vkcube

If the host driver of the system is not new enough, it is a good idea to build
the host driver as well when building the Venus driver.  Just remember to set
:envvar:`VK_DRIVER_FILES` when starting the vtest server so that the vtest
server finds the locally built host driver.

Virtio-GPU
----------

The driver requires ``VIRTGPU_PARAM_CONTEXT_INIT`` from the virtio-gpu kernel
driver, which was upstreamed in kernel 5.16.

crosvm is written in Rust.  To build crosvm, make sure Rust has been installed
and

.. code-block:: sh

 $ git clone --recurse-submodules \
       https://chromium.googlesource.com/chromiumos/platform/crosvm
 $ cd crosvm
 $ RUSTFLAGS="-L<path-to-virglrenderer>/out/src" cargo build \
       --features "x wl-dmabuf virgl_renderer virgl_renderer_next default-no-sandbox"

Note that crosvm must be built with ``default-no-sandbox`` or started with
``--disable-sandbox`` in this setup.

This is how one might want to start crosvm

.. code-block:: sh

 $ sudo LD_LIBRARY_PATH=<...> VK_DRIVER_FILES=<...> ./target/debug/crosvm run \
       --gpu vulkan=true \
       --gpu-render-server path=<path-to-virglrenderer>/out/server/virgl_render_server \
       --display-window-keyboard \
       --display-window-mouse \
       --net "host-ip 192.168.0.1,netmask=255.255.255.0,mac=12:34:56:78:9a:bc" \
       --rwdisk disk.img \
       -p root=/dev/vda1 \
       <path-to-bzImage>

assuming a working system is installed to partition 1 of ``disk.img``.
``sudo`` or ``CAP_NET_ADMIN`` is needed to set up the TAP network device.

Optional Requirements
---------------------

When virglrenderer is built with ``-Dminigbm_allocation=true``, the Venus
renderer might need to import GBM BOs.  The imports will fail unless the host
driver supports the formats, especially multi-planar ones, and the DRM format
modifiers of the GBM BOs.

In the future, if virglrenderer's ``virgl_renderer_export_fence`` is
supported, the Venus renderer will require :ext:`VK_KHR_external_fence_fd`
with ``VK_EXTERNAL_FENCE_HANDLE_TYPE_SYNC_FD_BIT`` from the host driver.

VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT
-----------------------------------

The Venus renderer makes assumptions about ``VkDeviceMemory`` that has
``VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT``.  The assumptions are illegal and rely
on the current behaviors of the host drivers.  It should be possible to remove
some of the assumptions and incrementally improve compatibilities with more
host drivers by imposing platform-specific requirements.  But the long-term
plan is to create a new Vulkan extension for the host drivers to address this
specific use case.

The Venus renderer assumes a device memory that has
``VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT`` can be exported as a mmapable dma-buf
(in the future, the plan is to export the device memory as an opaque fd).  It
chains ``VkExportMemoryAllocateInfo`` to ``VkMemoryAllocateInfo`` without
checking if the host driver can export the device memory.

The dma-buf is mapped (in the future, the plan is to import the opaque fd and
call ``vkMapMemory``) but the mapping is not accessed.  Instead, the mapping
is passed to ``KVM_SET_USER_MEMORY_REGION``.  The hypervisor, host KVM, and
the guest kernel work together to set up a write-back or write-combined guest
mapping (see ``virtio_gpu_vram_mmap`` of the virtio-gpu kernel driver).  CPU
accesses to the device memory are via the guest mapping, and are assumed to be
coherent when the device memory also has
``VK_MEMORY_PROPERTY_HOST_COHERENT_BIT``.

While the Venus renderer can force a ``VkDeviceMemory`` external, it does not
force a ``VkImage`` or a ``VkBuffer`` external.  As a result, it can bind an
external device memory to a non-external resource.
