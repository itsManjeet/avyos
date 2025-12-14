#include <assert.h>
#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

#include "client.h"
#include "drm.h"
#include "evdev.h"
#include "hidraw.h"
#include "linked_list.h"
#include "log.h"
#include "protocol.h"
#include "seat.h"
#include "terminal.h"
#include "wscons.h"

/*
 * seat_create creates a new seat with the given name, which may be VT-bound.
 *
 * A VT-bound seat is one where exactly one client session can exist per VT,
 * and switching VTs switches the active session accordingly. A non-VT-bound
 * seat is one where VTs are not used, and any number of sessions can be opened
 * which are switched "virtually", without any effects on present VTs.
 *
 * VT-bound seats must be used when VTs are enabled to properly disable kernel
 * console input processing and rendition.
 */
struct seat *seat_create(const char *seat_name, bool vt_bound) {
	struct seat *seat = calloc(1, sizeof(struct seat));
	if (seat == NULL) {
		return NULL;
	}
	linked_list_init(&seat->clients);
	seat->vt_bound = vt_bound;
	seat->seat_name = strdup(seat_name);
	seat->cur_vt = 0;
	if (seat->seat_name == NULL) {
		free(seat);
		return NULL;
	}
	if (vt_bound) {
		log_infof("Created VT-bound seat %s", seat_name);
	} else {
		log_infof("Created seat %s", seat_name);
	}
	return seat;
}

void seat_destroy(struct seat *seat) {
	assert(seat);
	while (!linked_list_empty(&seat->clients)) {
		struct client *client = (struct client *)seat->clients.next;
		assert(client->seat == seat);
		client_destroy(client);
	}
	linked_list_remove(&seat->link);
	free(seat->seat_name);
	free(seat);
}

static void seat_update_vt(struct seat *seat) {
	int tty0fd = terminal_open(0);
	if (tty0fd == -1) {
		log_errorf("Could not open tty0 to update VT: %s", strerror(errno));
		return;
	}
	seat->cur_vt = terminal_current_vt(tty0fd);
	close(tty0fd);
}

static int vt_open(int vt) {
	int ttyfd = terminal_open(vt);
	if (ttyfd == -1) {
		log_errorf("Could not open terminal for VT %d: %s", vt, strerror(errno));
		return -1;
	}

	terminal_set_process_switching(ttyfd, true);
	terminal_set_keyboard(ttyfd, false);
	terminal_set_graphics(ttyfd, true);
	close(ttyfd);
	return 0;
}

static int vt_close(int vt) {
	int ttyfd = terminal_open(vt);
	if (ttyfd == -1) {
		log_errorf("Could not open terminal to clean up VT %d: %s", vt, strerror(errno));
		return -1;
	}
	terminal_set_process_switching(ttyfd, true);
	terminal_set_keyboard(ttyfd, true);
	terminal_set_graphics(ttyfd, false);
	close(ttyfd);
	return 0;
}

static int vt_switch(struct seat *seat, int vt) {
	int ttyfd = terminal_open(seat->cur_vt);
	if (ttyfd == -1) {
		log_errorf("Could not open terminal to switch to VT %d: %s", vt, strerror(errno));
		return -1;
	}
	terminal_set_process_switching(ttyfd, true);
	terminal_switch_vt(ttyfd, vt);
	close(ttyfd);
	return 0;
}

static int vt_ack(struct seat *seat, bool release) {
	int tty0fd = terminal_open(seat->cur_vt);
	if (tty0fd == -1) {
		log_errorf("Could not open tty0 to ack VT signal: %s", strerror(errno));
		return -1;
	}
	if (release) {
		terminal_ack_release(tty0fd);
	} else {
		terminal_ack_acquire(tty0fd);
	}
	close(tty0fd);
	return 0;
}

/*
 * seat_activate opens the next client on the seat, assuming no client is
 * currently active.
 *
 * 1. If a client is queued on the seat by seat_set_next_session, it is chosen.
 *
 * 2. If VT bound, it choses the next client whose session matches the current
 *    VT. This should only apply if the previous client was deactivated because
 *    of a VT switch.
 *
 * 3. Otherwise, the first client on the seat's list of clients, if any.
 *
 * Be careful not to call seat_activate immediately after closing a client, as
 * this can lead to it immediatley re-opening. The client should be removed as
 * a candidate before seat_activate is called.
 */
static int seat_activate(struct seat *seat) {
	if (seat->active_client != NULL) {
		return 0;
	}

	struct client *next_client = NULL;
	if (seat->next_client != NULL) {
		log_debugf("Activating next queued client on %s", seat->seat_name);
		next_client = seat->next_client;
		seat->next_client = NULL;
	} else if (linked_list_empty(&seat->clients)) {
		log_infof("No clients on %s to activate", seat->seat_name);
		return -1;
	} else if (seat->vt_bound && seat->cur_vt == -1) {
		return -1;
	} else if (seat->vt_bound) {
		for (struct linked_list *elem = seat->clients.next; elem != &seat->clients;
		     elem = elem->next) {
			struct client *client = (struct client *)elem;
			if (client->session == seat->cur_vt) {
				log_debugf("Activating client belonging to VT %d", seat->cur_vt);
				next_client = client;
				goto done;
			}
		}

		log_infof("No clients belonging to VT %d to activate", seat->cur_vt);
		return -1;
	} else {
		log_debugf("Activating first client on %s", seat->seat_name);
		next_client = (struct client *)seat->clients.next;
	}

done:
	return seat_open_client(seat, next_client);
}

/*
 * seat_add_client assigns a sesssion ID to the client and adds it to the seat,
 * if allowed. The client does not open the seat, remaining closed until
 * `seat_open_client` is called.
 *
 * Fails if the client is not eligible to be added to a new seat, or if the
 * seat does not accept new clients.
 */
int seat_add_client(struct seat *seat, struct client *client) {
	if (client->seat != NULL) {
		log_error("Could not add client: client is already a member of a seat");
		errno = EBUSY;
		return -1;
	}

	if (seat->vt_bound && seat->active_client != NULL &&
	    seat->active_client->state != CLIENT_PENDING_DISABLE) {
		log_error("Could not add client: seat is VT-bound and has an active client");
		errno = EBUSY;
		return -1;
	}

	if (client->session != -1) {
		log_error("Could not add client: client cannot be reused");
		errno = EINVAL;
		return -1;
	}

	if (seat->vt_bound) {
		seat_update_vt(seat);
		if (seat->cur_vt == -1) {
			log_error("Could not determine VT for client");
			errno = EINVAL;
			return -1;
		}
		if (seat->active_client != NULL) {
			for (struct linked_list *elem = seat->clients.next; elem != &seat->clients;
			     elem = elem->next) {
				struct client *client = (struct client *)elem;
				if (client->session == seat->cur_vt) {
					log_error("Could not add client: seat is VT-bound and already has pending client");
					errno = EBUSY;
					return -1;
				}
			}
		}
		client->session = seat->cur_vt;
	} else {
		client->session = seat->session_cnt++;
	}

	client->seat = seat;
	linked_list_remove(&client->link);
	linked_list_insert(&seat->clients, &client->link);

	log_infof("Added client %d to %s", client->session, seat->seat_name);

	return 0;
}

/*
 * seat_remove_client tears down the client and removes it from the seat,
 * revoking any open devices as necessary. If the client was active on the seat
 * at the time of this call, seat_activate is called to activate a new client
 * if any is eligible. If the seat is VT-bound, this also re-configures the VT
 * for non-graphical use.
 */
void seat_remove_client(struct client *client) {
	struct seat *seat = client->seat;
	if (seat->next_client == client) {
		seat->next_client = NULL;
	}

	linked_list_remove(&client->link);
	linked_list_init(&client->link);

	while (!linked_list_empty(&client->devices)) {
		struct seat_device *device = (struct seat_device *)client->devices.next;
		if (seat_close_device(client, device) == -1) {
			log_errorf("Could not close %s: %s", device->path, strerror(errno));
		}
	}

	bool was_current = seat->active_client == client;
	if (was_current) {
		seat->active_client = NULL;
		seat_activate(seat);
	}

	if (seat->vt_bound) {
		if (was_current && seat->active_client == NULL) {
			// This client was current, but there were no clients
			// waiting to take this VT, so clean it up.
			log_debug("Closing active VT");
			vt_close(seat->cur_vt);
		} else if (!was_current && client->state != CLIENT_CLOSED) {
			// This client was not current, but as the client was
			// running, we need to clean up the VT.
			log_debug("Closing inactive VT");
			vt_close(client->session);
		}
	}

	client->state = CLIENT_CLOSED;
	client->seat = NULL;

	log_infof("Removed client %d from %s", client->session, seat->seat_name);
}

/*
 * seat_find_device finds an open device on the seat based on its device ID.
 */
struct seat_device *seat_find_device(struct client *client, int device_id) {
	if (device_id == 0) {
		errno = EINVAL;
		return NULL;
	}

	for (struct linked_list *elem = client->devices.next; elem != &client->devices;
	     elem = elem->next) {
		struct seat_device *seat_device = (struct seat_device *)elem;
		if (seat_device->device_id == device_id) {
			return seat_device;
		}
	}
	errno = ENOENT;
	return NULL;
}

/*
 * seat_open_device opens a device by the specified device path for the client,
 * sanitizing the path and configuring the device as necessary for usage. If
 * such a device has already been opened, the reference count is increased and
 * the device entry is reused.
 *
 * Fails if the client is not active or has exceeded its device limit, or if
 * the device type is not supported or could not be opened.
 */
struct seat_device *seat_open_device(struct client *client, const char *path) {
	struct seat *seat = client->seat;
	log_debugf("Opening device %s for client %d on %s", path, client->session, seat->seat_name);

	if (client->state != CLIENT_ACTIVE) {
		log_error("Could open device: client is not active");
		errno = EPERM;
		return NULL;
	}
	assert(seat->active_client == client);

	char sanitized_path[PATH_MAX];
	if (realpath(path, sanitized_path) == NULL) {
		log_errorf("Could not canonicalize path %s: %s", path, strerror(errno));
		return NULL;
	}

	enum seat_device_type type;
	if (path_is_evdev(sanitized_path)) {
		type = SEAT_DEVICE_TYPE_EVDEV;
	} else if (path_is_drm(sanitized_path)) {
		type = SEAT_DEVICE_TYPE_DRM;
	} else if (path_is_wscons(sanitized_path)) {
		type = SEAT_DEVICE_TYPE_WSCONS;
	} else if (path_is_hidraw(sanitized_path)) {
		type = SEAT_DEVICE_TYPE_HIDRAW;
	} else {
		log_errorf("%s is not a supported device type ", sanitized_path);
		errno = ENOENT;
		return NULL;
	}

	int device_id = 1;
	size_t device_count = 0;
	struct seat_device *device = NULL;
	for (struct linked_list *elem = client->devices.next; elem != &client->devices;
	     elem = elem->next) {
		struct seat_device *old_device = (struct seat_device *)elem;

		if (strcmp(old_device->path, sanitized_path) == 0) {
			old_device->ref_cnt++;
			return device;
		}

		if (old_device->device_id >= device_id) {
			device_id = old_device->device_id + 1;
		}
		device_count++;
	}

	if (device_count >= MAX_SEAT_DEVICES) {
		log_error("Client exceeded max seat devices");
		errno = EMFILE;
		return NULL;
	}

	int fd = open(sanitized_path, O_RDWR | O_NOCTTY | O_NOFOLLOW | O_CLOEXEC | O_NONBLOCK);
	if (fd == -1) {
		log_errorf("Could not open file: %s", strerror(errno));
		return NULL;
	}

	switch (type) {
	case SEAT_DEVICE_TYPE_DRM:
		if (drm_set_master(fd) == -1) {
			log_errorf("Could not make device fd drm master: %s", strerror(errno));
		}
		break;
	case SEAT_DEVICE_TYPE_EVDEV:
		// Nothing to do here
		break;
	case SEAT_DEVICE_TYPE_WSCONS:
		// Nothing to do here
		break;
	case SEAT_DEVICE_TYPE_HIDRAW:
		// Nothing to do here
		break;
	default:
		log_errorf("Invalid seat device type: %d", type);
		abort();
	}

	device = calloc(1, sizeof(struct seat_device));
	if (device == NULL) {
		log_errorf("Allocation failed: %s", strerror(errno));
		close(fd);
		errno = ENOMEM;
		return NULL;
	}

	device->path = strdup(sanitized_path);
	if (device->path == NULL) {
		log_errorf("Allocation failed: %s", strerror(errno));
		close(fd);
		free(device);
		return NULL;
	}

	device->ref_cnt = 1;
	device->type = type;
	device->fd = fd;
	device->device_id = device_id;
	device->active = true;
	linked_list_insert(&client->devices, &device->link);

	return device;
}

/*
 * seat_deactivate_device revokes access to the device so that the client can
 * no longer use it for privileged actions. Depending on the device type, the
 * client may be required to reopen the device to use it again.
 */
static int seat_deactivate_device(struct seat_device *seat_device) {
	if (!seat_device->active) {
		return 0;
	}
	switch (seat_device->type) {
	case SEAT_DEVICE_TYPE_DRM:
		if (drm_drop_master(seat_device->fd) == -1) {
			log_errorf("Could not revoke drm master on device fd: %s", strerror(errno));
			return -1;
		}
		break;
	case SEAT_DEVICE_TYPE_EVDEV:
		if (evdev_revoke(seat_device->fd) == -1) {
			log_errorf("Could not revoke evdev on device fd: %s", strerror(errno));
			return -1;
		}
		break;
	case SEAT_DEVICE_TYPE_HIDRAW:
		if (hidraw_revoke(seat_device->fd) == -1) {
			log_errorf("Could not revoke hidraw on device fd: %s", strerror(errno));
			return -1;
		}
		break;
	case SEAT_DEVICE_TYPE_WSCONS:
		// Nothing to do here
		break;
	default:
		log_errorf("Invalid seat device type: %d", seat_device->type);
		abort();
	}
	seat_device->active = false;
	return 0;
}

/*
 * seat_close_device reduces the reference count for the device. If it reaches
 * zero, the device is deactivated, closed and removed.
 */
int seat_close_device(struct client *client, struct seat_device *seat_device) {
	log_debugf("Closing device %s for client %d on %s", seat_device->path, client->session,
		   client->seat->seat_name);

	seat_device->ref_cnt--;
	if (seat_device->ref_cnt > 0) {
		return 0;
	}

	linked_list_remove(&seat_device->link);
	if (seat_device->fd != -1) {
		seat_deactivate_device(seat_device);
		close(seat_device->fd);
	}
	free(seat_device->path);
	free(seat_device);
	return 0;
}

/*
 * seat_activate_device re-activates the device for reuse after deactivation.
 * It fails if the device cannot be reused.
 */
static int seat_activate_device(struct seat_device *seat_device) {
	if (seat_device->active) {
		return 0;
	}
	switch (seat_device->type) {
	case SEAT_DEVICE_TYPE_DRM:
		if (drm_set_master(seat_device->fd) == -1) {
			log_errorf("Could not make device fd drm master: %s", strerror(errno));
		}
		seat_device->active = true;
		break;
	case SEAT_DEVICE_TYPE_EVDEV:
		errno = EINVAL;
		return -1;
	case SEAT_DEVICE_TYPE_HIDRAW:
		errno = EINVAL;
		return -1;
	case SEAT_DEVICE_TYPE_WSCONS:
		// Nothing to do here
		break;
	default:
		log_errorf("Invalid seat device type: %d", seat_device->type);
		abort();
	}

	return 0;
}

/*
 * seat_open_client makes the client active. The client must be a disabled or
 * new member of the seat, and the seat must not have an active seat. If
 * VT-bound, this opens the VT and configures it for a graphical session.
 */
int seat_open_client(struct seat *seat, struct client *client) {
	assert(client->seat == seat);
	if (client->state != CLIENT_NEW && client->state != CLIENT_DISABLED) {
		log_error("Could not enable client: client is not new or disabled");
		errno = EALREADY;
		return -1;
	}

	if (seat->active_client != NULL) {
		log_error("Could not enable client: seat already has an active client");
		errno = EBUSY;
		return -1;
	}

	if (seat->vt_bound && vt_open(client->session) == -1) {
		log_error("Could not open VT for client");
		goto error;
	}

	for (struct linked_list *elem = client->devices.next; elem != &client->devices;
	     elem = elem->next) {
		struct seat_device *device = (struct seat_device *)elem;
		if (seat_activate_device(device) == -1) {
			log_errorf("Could not activate %s: %s", device->path, strerror(errno));
		}
	}

	client->state = CLIENT_ACTIVE;
	seat->active_client = client;
	if (client_send_enable_seat(client) == -1) {
		log_error("Could not send enable signal to client");
		goto error;
	}

	log_infof("Opened client %d on %s", client->session, seat->seat_name);
	return 0;

error:
	if (seat->vt_bound) {
		vt_close(seat->cur_vt);
	}
	return -1;
}

/*
 * seat_disable_client deactivates all devices of an active client and sends a
 * request for it to disable, which it must ack. It is meant for when a client
 * is suspended due to session switching.
 */
static int seat_disable_client(struct client *client) {
	if (client->state != CLIENT_ACTIVE) {
		log_error("Could not disable client: client is not active");
		errno = EBUSY;
		return -1;
	}
	struct seat *seat = client->seat;
	assert(seat != NULL);
	assert(seat->active_client == client);

	// We *deactivate* all remaining fds. These may later be reactivated.
	// The reason we cannot just close them is that certain device fds, such
	// as for DRM, must maintain the exact same file description for their
	// contexts to remain valid.
	for (struct linked_list *elem = client->devices.next; elem != &client->devices;
	     elem = elem->next) {
		struct seat_device *device = (struct seat_device *)elem;
		if (seat_deactivate_device(device) == -1) {
			log_errorf("Could not deactivate %s: %s", device->path, strerror(errno));
		}
	}

	client->state = CLIENT_PENDING_DISABLE;
	if (client_send_disable_seat(seat->active_client) == -1) {
		log_error("Could not send disable event");
		return -1;
	}

	log_infof("Disabling client %d on %s", client->session, seat->seat_name);
	return 0;
}

/*
 * seat_ack_disable_client finalizes disable of a client, and activates the
 * next applicable client if any. As disable is intended for session switching,
 * there should either be a queued session or we are on a different VT. In
 * either case, we should not risk the client being re-opened.
 */
int seat_ack_disable_client(struct client *client) {
	struct seat *seat = client->seat;
	if (client->state != CLIENT_PENDING_DISABLE) {
		log_error("Could not ack disable: client is not pending disable");
		errno = EBUSY;
		return -1;
	}

	client->state = CLIENT_DISABLED;
	log_infof("Disabled client %d on %s", client->session, seat->seat_name);

	if (seat->active_client != client) {
		return 0;
	}

	seat->active_client = NULL;
	seat_activate(seat);

	// If we're VT-bound, we've either de-activated a client on a foreign
	// VT, in which case we need to do nothing, or disabled the current VT,
	// in which case seat_activate would just immediately re-enable it.
	return 0;
}

/*
 * seat_set_next_session queues a new client to be opened based on its session
 * ID. It can only b eperformed by an active client, and only if a switch has
 * not already been requested. If the seat is VT-bound, a VT switch is
 * performed and the VT ack/release mechanism takes care of the rest to avoid
 * conflicts between the two mechanisms.
 */
int seat_set_next_session(struct client *client, int session) {
	if (client->state != CLIENT_ACTIVE) {
		log_error("Could not set next session: client is not active");
		errno = EPERM;
		return -1;
	}
	struct seat *seat = client->seat;
	assert(seat->active_client == client);

	if (session <= 0) {
		log_errorf("Could not set next session: invalid session value %d", session);
		errno = EINVAL;
		return -1;
	}

	if (session == client->session) {
		log_info("Could not set next session: requested session is already active");
		return 0;
	}

	if (seat->next_client != NULL) {
		log_info("Could not set next session: switch is already queued");
		return 0;
	}

	if (seat->vt_bound) {
		log_infof("Switching from VT %d to VT %d", seat->cur_vt, session);
		if (vt_switch(seat, session) == -1) {
			log_error("Could not switch VT");
			return -1;
		}
		return 0;
	}

	struct client *target = NULL;
	for (struct linked_list *elem = seat->clients.next; elem != &seat->clients;
	     elem = elem->next) {
		struct client *c = (struct client *)elem;
		if (c->session == session) {
			target = c;
			break;
		}
	}

	if (target == NULL) {
		log_error("Could not set next session: no such client");
		errno = EINVAL;
		return -1;
	}

	log_infof("Queuing switch to client %d on %s", session, seat->seat_name);
	seat->next_client = target;
	seat_disable_client(seat->active_client);
	return 0;
}

/*
 * seat_vt_activate is called when a VT activation signal is received. We
 * respond by acking the signal and finding an applicable client for the newly
 * opened VT.
 */
int seat_vt_activate(struct seat *seat) {
	if (!seat->vt_bound) {
		log_debug("VT activation on non VT-bound seat, ignoring");
		return -1;
	}
	seat_update_vt(seat);
	log_debug("Activating VT");
	vt_ack(seat, false);
	if (seat->active_client == NULL) {
		seat_activate(seat);
	}
	return 0;
}

/*
 * seat_vt_release is called when a VT release signal is received. We respond
 * by disabling our current client and acking the signal to let the kernel
 * proceed with the switch.
 */
int seat_vt_release(struct seat *seat) {
	if (!seat->vt_bound) {
		log_debug("VT release request on non VT-bound seat, ignoring");
		return -1;
	}
	seat_update_vt(seat);

	log_debug("Releasing VT");
	if (seat->active_client != NULL) {
		seat_disable_client(seat->active_client);
	}

	vt_ack(seat, true);
	seat->cur_vt = -1;
	return 0;
}
