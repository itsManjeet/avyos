/*
 * Copyright © 2025 Red Hat, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice (including the next
 * paragraph) shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
 * DEALINGS IN THE SOFTWARE.
 */

#include "config.h"

#include <assert.h>
#include <lauxlib.h>
#include <libevdev/libevdev.h>
#include <lua.h>
#include <lualib.h>

#include "util-mem.h"
#include "util-strings.h"

#include "evdev-frame.h"
#include "libinput-feature.h"
#include "libinput-log.h"
#include "libinput-plugin-lua.h"
#include "libinput-plugin.h"
#include "libinput-util.h"
#include "timer.h"

const uint32_t LIBINPUT_PLUGIN_VERSION = 1U;

#define PLUGIN_METATABLE "LibinputPlugin"
#define EVDEV_DEVICE_METATABLE "EvdevDevice"

static const char libinput_lua_plugin_key = 'p'; /* key to lua registry */
static const char libinput_key = 'l';            /* key to lua registry */

DEFINE_TRIVIAL_CLEANUP_FUNC(lua_State *, lua_close);

struct udev_property {
	struct list link;
	char *key;
	char *value;
};

static inline struct udev_property *
udev_property_new(const char *key, const char *value)
{
	struct udev_property *prop = zalloc(sizeof(*prop));
	prop->key = safe_strdup(key);
	prop->value = safe_strdup(value);
	return prop;
}

static inline void
udev_property_destroy(struct udev_property *prop)
{
	list_remove(&prop->link);
	free(prop->key);
	free(prop->value);
	free(prop);
}

/* A thin wrapper struct that just needs to exist, all
 * the actual logic is struct libinput_lua_plugin */
typedef struct {
} LibinputPlugin;

typedef struct {
	struct list link;
	int refid;

	struct libinput_device *device;

	unsigned int id;
	unsigned int bustype;
	unsigned int vid;
	unsigned int pid;
	char *name;
	struct list udev_properties_list;

	struct libevdev *evdev;

	int device_removed_refid;
	int frame_refid;

	/* Caches any disable_feature calls during device_new */
	bool was_added;
	bitmask_t disabled_features;
} EvdevDevice;

struct libinput_lua_plugin {
	struct libinput_plugin *parent;
	lua_State *L;
	int sandbox_table_idx;
	bool register_called;

	struct list evdev_devices; /* EvdevDevice */

	size_t version;
	int device_new_refid;
	int timer_expired_refid;

	struct libinput_plugin_timer *timer;
	bool in_timer_func;
};

static struct libinput_lua_plugin *
lua_get_libinput_lua_plugin(lua_State *L)
{
	struct libinput_lua_plugin *plugin = NULL;

	lua_pushlightuserdata(L, (void *)&libinput_lua_plugin_key);
	lua_gettable(L, LUA_REGISTRYINDEX);
	plugin = lua_touserdata(L, -1);
	lua_pop(L, 1);

	return plugin;
}

/* Prints the current stack "layout" with a message */
#define lua_show_stack(L, ...) { \
	etrace(__VA_ARGS__); \
	etrace("pcall: stack has: %d", lua_gettop(L)); \
	for (int i = -1; i >= -lua_gettop(L); i--) \
		etrace("   stack %d: %s", i, lua_typename(L, lua_type(L, i))); \
}

static struct libinput *
lua_get_libinput(lua_State *L)
{
	struct libinput *libinput = NULL;

	lua_pushlightuserdata(L, (void *)&libinput_key);
	lua_gettable(L, LUA_REGISTRYINDEX);
	libinput = lua_touserdata(L, -1);
	lua_pop(L, 1);

	return libinput;
}

static void
lua_push_evdev_device(lua_State *L,
		      struct libinput_lua_plugin *plugin,
		      struct libinput_device *device,
		      struct libevdev *evdev,
		      struct udev_device *udev_device)
{
	EvdevDevice *lua_device = lua_newuserdata(L, sizeof(*lua_device));
	memset(lua_device, 0, sizeof(*lua_device));
	lua_device->device = libinput_device_ref(device);
	lua_device->evdev = evdev;
	lua_device->bustype = libinput_device_get_id_bustype(device);
	lua_device->vid = libinput_device_get_id_vendor(device);
	lua_device->pid = libinput_device_get_id_product(device);
	lua_device->name = strdup(libinput_device_get_name(device));
	lua_device->device_removed_refid = LUA_NOREF;
	lua_device->frame_refid = LUA_NOREF;
	list_init(&lua_device->udev_properties_list);

	struct udev_list_entry *e = udev_device_get_properties_list_entry(udev_device);
	while (e) {
		const char *key = udev_list_entry_get_name(e);
		if (strstartswith(key, "ID_INPUT_") &&
		    !streq(key, "ID_INPUT_WIDTH_MM") &&
		    !streq(key, "ID_INPUT_HEIGHT_MM")) {
			const char *value = udev_list_entry_get_value(e);
			if (!streq(value, "0")) {
				struct udev_property *prop =
					udev_property_new(key, value);
				list_insert(&lua_device->udev_properties_list,
					    &prop->link);
			}
		}
		e = udev_list_entry_get_next(e);
	}

	list_insert(&plugin->evdev_devices, &lua_device->link);

	lua_pushvalue(L, -1);                               /* Copy to top */
	lua_device->refid = luaL_ref(L, LUA_REGISTRYINDEX); /* ref to device */

	luaL_getmetatable(L, EVDEV_DEVICE_METATABLE);
	lua_setmetatable(L, -2);
}

static void
lua_push_evdev_frame(lua_State *L, struct evdev_frame *frame)
{
	size_t nevents;
	struct evdev_event *events = evdev_frame_get_events(frame, &nevents);

	lua_newtable(L);
	for (size_t i = 0; i < nevents; i++) {
		struct evdev_event *e = &events[i];

		if (evdev_usage_eq(e->usage, EVDEV_SYN_REPORT))
			break;

		lua_newtable(L);
		lua_pushinteger(L, evdev_usage_as_uint32_t(e->usage));
		lua_setfield(L, -2, "usage");
		lua_pushinteger(L, e->value);
		lua_setfield(L, -2, "value");
		lua_rawseti(L, -2, i + 1);
	}
}

static void
lua_pop_evdev_frame(struct libinput_lua_plugin *plugin,
		    struct libevdev *libevdev,
		    struct evdev_frame *frame_out)
{
	lua_State *L = plugin->L;

	if (lua_isnil(L, -1)) {
		goto out;
	}

	if (!lua_istable(L, -1)) {
		plugin_log_bug(plugin->parent,
			       "expected table like `{ events = { ... } }`, got %s",
			       lua_typename(L, lua_type(L, -1)));
		goto out;
	}

	struct evdev_event events[64] = { 0 };
	size_t nevents = 0;

	lua_pushnil(L);
	while (lua_next(L, -2) != 0 && nevents < ARRAY_LENGTH(events)) {

		/* -2 is the index, -1 our { usage = ... } table */
		if (!lua_istable(L, -1)) {
			plugin_log_bug(
				plugin->parent,
				"expected table like `{ type = ..., code = ...}`, got %s",
				lua_typename(L, lua_type(L, -1)));
			goto out;
		}

		lua_getfield(L, -1, "usage");
		uint32_t usage_value = luaL_checkinteger(L, -1);
		lua_pop(L, 1);

		lua_getfield(L, -1, "value");
		int32_t value = luaL_checkinteger(L, -1);
		lua_pop(L, 1);

		lua_pop(L, 1); /* pop { usage = ..., value = ...} */

		evdev_usage_t usage = evdev_usage_from_uint32_t(usage_value);
		unsigned int type = evdev_usage_type(usage);
		unsigned int code = evdev_usage_code(usage);
		if (libevdev_has_event_code(libevdev, type, code)) {
			struct evdev_event *e = &events[nevents++];
			e->usage = usage;
			e->value = value;

			if (evdev_usage_eq(e->usage, EVDEV_SYN_REPORT)) {
				lua_pop(L, 1); /* force-pop the nil */
				break;
			}
		}
	}

	if (nevents == 0) {
		events[0].usage = evdev_usage_from_uint32_t(EVDEV_SYN_REPORT);
		events[0].value = 0;
		nevents++;
	}

	if (evdev_frame_set(frame_out, events, nevents) == -ENOMEM) {
		plugin_log_bug(plugin->parent, "too many events in frame");
	}

out:
	lua_pop(L, 1);
}

static bool
libinput_lua_pcall(struct libinput_lua_plugin *plugin, int narg, int nres)
{
	lua_State *L = plugin->L;

	int rc = lua_pcall(L, narg, nres, 0);
	if (rc != LUA_OK) {
		auto libinput_plugin = plugin->parent;
		const char *errormsg = lua_tostring(L, -1);
		if (strstr(errormsg, "@@unregistering@@") == NULL) {
			plugin_log_bug(libinput_plugin,
				       "unloading after error: %s\n",
				       errormsg);
		}
		lua_pop(L, 1); /* pop error message */

		if (plugin->timer)
			libinput_plugin_timer_cancel(plugin->timer);
		libinput_plugin_unregister(libinput_plugin);
		/* plugin system will destroy the plugin later */
	}
	return rc == LUA_OK;
}

static void
libinput_lua_plugin_device_new(struct libinput_plugin *libinput_plugin,
			       struct libinput_device *device,
			       struct libevdev *evdev,
			       struct udev_device *udev_device)
{
	struct libinput_lua_plugin *plugin =
		libinput_plugin_get_user_data(libinput_plugin);

	lua_rawgeti(plugin->L, LUA_REGISTRYINDEX, plugin->device_new_refid);
	lua_push_evdev_device(plugin->L, plugin, device, evdev, udev_device);

	libinput_lua_pcall(plugin, 1, 0);
}

static void
remove_device(struct libinput_lua_plugin *plugin, EvdevDevice *evdev)
{
	/* Don't allow access to the libevdev context during remove */
	evdev->evdev = NULL;
	if (evdev->device_removed_refid != LUA_NOREF) {
		lua_rawgeti(plugin->L, LUA_REGISTRYINDEX, evdev->device_removed_refid);
		lua_rawgeti(plugin->L, LUA_REGISTRYINDEX, evdev->refid);

		if (!libinput_lua_pcall(plugin, 1, 0))
			return;
	}
	luaL_unref(plugin->L, evdev->refid, LUA_REGISTRYINDEX);
	evdev->refid = LUA_NOREF;
	list_remove(&evdev->link);
	list_init(&evdev->link); /* so we can list_remove in _gc */

	struct udev_property *prop;
	list_for_each_safe(prop, &evdev->udev_properties_list, link) {
		udev_property_destroy(prop);
	}
	free(evdev->name);
	evdev->name = NULL;
	evdev->device = libinput_device_unref(evdev->device);

	/* This device no longer exists but our lua code may have a
	 * reference to it */
}

static void
libinput_lua_plugin_device_ignored(struct libinput_plugin *libinput_plugin,
				   struct libinput_device *device)
{
	struct libinput_lua_plugin *plugin =
		libinput_plugin_get_user_data(libinput_plugin);

	EvdevDevice *evdev;
	list_for_each_safe(evdev, &plugin->evdev_devices, link) {
		if (evdev->device != device)
			continue;
		remove_device(plugin, evdev);
	}
}

static void
libinput_lua_plugin_device_removed(struct libinput_plugin *libinput_plugin,
				   struct libinput_device *device)
{
	struct libinput_lua_plugin *plugin =
		libinput_plugin_get_user_data(libinput_plugin);

	EvdevDevice *evdev;
	list_for_each_safe(evdev, &plugin->evdev_devices, link) {
		if (evdev->device != device)
			continue;
		remove_device(plugin, evdev);
	}
}

static void
libinput_lua_plugin_evdev_frame(struct libinput_plugin *libinput_plugin,
				struct libinput_device *device,
				struct evdev_frame *frame)
{
	struct libinput_lua_plugin *plugin =
		libinput_plugin_get_user_data(libinput_plugin);

	EvdevDevice *evdev;
	list_for_each_safe(evdev, &plugin->evdev_devices, link) {
		if (evdev->device != device)
			continue;

		if (evdev->frame_refid == LUA_NOREF)
			continue;

		lua_rawgeti(plugin->L, LUA_REGISTRYINDEX, evdev->frame_refid);
		lua_rawgeti(plugin->L, LUA_REGISTRYINDEX, evdev->refid);
		lua_push_evdev_frame(plugin->L, frame);
		lua_pushinteger(plugin->L,
				usec_as_uint64_t(evdev_frame_get_time(frame)));

		if (!libinput_lua_pcall(plugin, 3, 1))
			return;
		lua_pop_evdev_frame(plugin, evdev->evdev, frame);
	}
}

static void
register_func(struct lua_State *L, int stack_index, int *refid)
{
	if (*refid != LUA_NOREF)
		luaL_unref(L, LUA_REGISTRYINDEX, *refid);
	lua_pushvalue(L, stack_index);           /* Copy function to top */
	*refid = luaL_ref(L, LUA_REGISTRYINDEX); /* ref to function */
}

static void
unregister_func(struct lua_State *L, int *refid)
{
	if (*refid != LUA_NOREF) {
		luaL_unref(L, LUA_REGISTRYINDEX, *refid);
		*refid = LUA_NOREF;
	}
}

static int
libinputplugin_connect(lua_State *L)
{
	LibinputPlugin *p = luaL_checkudata(L, 1, PLUGIN_METATABLE);
	luaL_argcheck(L, p != NULL, 1, PLUGIN_METATABLE " expected");

	const char *name = luaL_checkstring(L, 2);
	luaL_checktype(L, 3, LUA_TFUNCTION);

	struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);

	/* Version 1 signals */
	if (streq(name, "new-evdev-device")) {
		register_func(L, 3, &plugin->device_new_refid);
	} else if (streq(name, "timer-expired")) {
		register_func(L, 3, &plugin->timer_expired_refid);
	} else {
		return luaL_error(L, "Unknown name: %s", name);
	}

	return 0;
}

static int
libinputplugin_now(lua_State *L)
{
	LibinputPlugin *p = luaL_checkudata(L, 1, PLUGIN_METATABLE);
	luaL_argcheck(L, p != NULL, 1, PLUGIN_METATABLE " expected");

	struct libinput *libinput = lua_get_libinput(L);
	usec_t now = libinput_now(libinput);

	lua_pushinteger(L, usec_as_uint64_t(now));

	return 1;
}

static int
libinputplugin_version(lua_State *L)
{
	LibinputPlugin *p = luaL_checkudata(L, 1, PLUGIN_METATABLE);
	luaL_argcheck(L, p != NULL, 1, PLUGIN_METATABLE " expected");

	struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);
	lua_pushinteger(L, plugin->version);
	return 1;
}

static int
libinputplugin_register(lua_State *L)
{
	LibinputPlugin *p = luaL_checkudata(L, 1, PLUGIN_METATABLE);
	luaL_argcheck(L, p != NULL, 1, PLUGIN_METATABLE " expected");

	struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);
	if (plugin->register_called) {
		return luaL_error(L, "plugin already registered");
	}

	uint32_t versions[16] = { 0 };
	size_t idx = 0;

	luaL_checktype(L, 2, LUA_TTABLE);
	lua_pushnil(L);
	while (idx < ARRAY_LENGTH(versions) && lua_next(L, -2) != 0) {
		int version = luaL_checkinteger(L, -1);
		lua_pop(L, 1);
		if (version <= 0) {
			return luaL_error(L, "Invalid version number");
		}
		versions[idx++] = version;
	}

	ARRAY_FOR_EACH(versions, v) {
		if (*v == 0)
			break;
		if (*v == LIBINPUT_PLUGIN_VERSION) {
			plugin->version = *v;
			plugin->register_called = true;

			lua_pushinteger(L, plugin->version);

			return 1;
		}
	}

	return luaL_error(L, "None of this plugin's versions are supported");
}

static int
libinputplugin_unregister(lua_State *L)
{
	LibinputPlugin *p = luaL_checkudata(L, 1, PLUGIN_METATABLE);
	luaL_argcheck(L, p != NULL, 1, PLUGIN_METATABLE " expected");

	/* Bit of a hack: unregister should work like os.exit(1)
	 * but we're in a lua context here so the easiest way
	 * to handle this is pretend we have an error, let
	 * our error handler unwind and just search for this
	 * magic string to *not* print log message */
	return luaL_error(L, "@@unregistering@@");
}

static int
libinputplugin_gc(lua_State *L)
{
	LibinputPlugin *p = luaL_checkudata(L, 1, PLUGIN_METATABLE);
	luaL_argcheck(L, p != NULL, 1, PLUGIN_METATABLE " expected");

	struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);
	if (plugin->timer)
		libinput_plugin_timer_cancel(plugin->timer);

	/* We're about to destroy the plugin so the timer is the only
	 * thing we need to stop, the rest will be cleaned up
	 * when we destroy the plugin */

	return 0;
}

static void
plugin_timer_func(struct libinput_plugin *libinput_plugin, usec_t now, void *data)
{
	struct libinput_lua_plugin *plugin = data;
	struct lua_State *L = plugin->L;

	lua_rawgeti(L, LUA_REGISTRYINDEX, plugin->timer_expired_refid);
	lua_pushinteger(L, usec_as_uint64_t(now));

	libinput_lua_pcall(plugin, 1, 0);
}

static int
libinputplugin_timer_set(lua_State *L, usec_t offset)
{
	LibinputPlugin *p = luaL_checkudata(L, 1, PLUGIN_METATABLE);
	luaL_argcheck(L, p != NULL, 1, PLUGIN_METATABLE " expected");

	struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);
	uint64_t timeout = luaL_checkinteger(L, 2);

	if (!plugin->timer) {
		plugin->timer = libinput_plugin_timer_new(
			plugin->parent,
			libinput_plugin_get_name(plugin->parent),
			plugin_timer_func,
			plugin);
	}

	libinput_plugin_timer_set(plugin->timer,
				  usec_add(offset, usec_from_uint64_t(timeout)));

	return 0;
}

static int
libinputplugin_timer_set_absolute(lua_State *L)
{
	return libinputplugin_timer_set(L, usec_from_uint64_t(0));
}

static int
libinputplugin_timer_set_relative(lua_State *L)
{
	auto libinput = lua_get_libinput(L);
	return libinputplugin_timer_set(L, libinput_now(libinput));
}

static int
libinputplugin_timer_cancel(lua_State *L)
{
	LibinputPlugin *p = luaL_checkudata(L, 1, PLUGIN_METATABLE);
	luaL_argcheck(L, p != NULL, 1, PLUGIN_METATABLE " expected");

	struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);
	if (plugin->timer)
		libinput_plugin_timer_cancel(plugin->timer);

	return 0;
}

static int
libinputplugin_log(lua_State *L, enum libinput_log_priority pri)
{
	LibinputPlugin *p = luaL_checkudata(L, 1, PLUGIN_METATABLE);
	luaL_argcheck(L, p != NULL, 1, PLUGIN_METATABLE " expected");

	const char *message = luaL_checkstring(L, 2);

	auto plugin = lua_get_libinput_lua_plugin(L);

	plugin_log_msg(plugin->parent, pri, "%s\n", message);

	return 0;
}

static int
libinputplugin_log_debug(lua_State *L)
{
	return libinputplugin_log(L, LIBINPUT_LOG_PRIORITY_DEBUG);
}

static int
libinputplugin_log_info(lua_State *L)
{
	return libinputplugin_log(L, LIBINPUT_LOG_PRIORITY_INFO);
}

static int
libinputplugin_log_error(lua_State *L)
{
	return libinputplugin_log(L, LIBINPUT_LOG_PRIORITY_ERROR);
}

static const struct luaL_Reg libinputplugin_vtable[] = {
	{ "now", libinputplugin_now },
	{ "version", libinputplugin_version },
	{ "connect", libinputplugin_connect },
	{ "register", libinputplugin_register },
	{ "unregister", libinputplugin_unregister },
	{ "timer_cancel", libinputplugin_timer_cancel },
	{ "timer_set_absolute", libinputplugin_timer_set_absolute },
	{ "timer_set_relative", libinputplugin_timer_set_relative },
	{ "log_debug", libinputplugin_log_debug },
	{ "log_info", libinputplugin_log_info },
	{ "log_error", libinputplugin_log_error },
	{ "__gc", libinputplugin_gc },
	{ NULL, NULL }
};

static void
libinputplugin_init(lua_State *L)
{
	luaL_newmetatable(L, PLUGIN_METATABLE);
	lua_pushstring(L, "__index");
	lua_pushvalue(L, -2); /* push metatable */
	lua_settable(L, -3);  /* metatable.__index = metatable */
	luaL_setfuncs(L, libinputplugin_vtable, 0);
}

static int
evdevdevice_info(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE " expected");

	lua_newtable(L); /* { bustype: ..., vid: ..., pid: ..., name: ... } */

	if (device->evdev == NULL)
		return 1;

	lua_pushinteger(L, device->bustype);
	lua_setfield(L, -2, "bustype");
	lua_pushinteger(L, device->vid);
	lua_setfield(L, -2, "vid");
	lua_pushinteger(L, device->pid);
	lua_setfield(L, -2, "pid");

	return 1;
}

static int
evdevdevice_name(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE "expected");

	lua_pushstring(L, device->name);

	return 1;
}

static int
evdevdevice_usages(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE "expected");

	lua_newtable(L); /* { evdev.REL_X: ... } */

	if (device->evdev == NULL)
		return 1;

	for (unsigned int t = 0; t <= EV_MAX; t++) {
		if (!libevdev_has_event_type(device->evdev, t))
			continue;

		int max = libevdev_event_type_get_max(t);
		for (unsigned int code = 0; (int)code < max; code++) {
			if (!libevdev_has_event_code(device->evdev, t, code))
				continue;

			evdev_usage_t usage = evdev_usage_from_code(t, code);
			lua_pushboolean(L, true);
			lua_rawseti(L, -2, evdev_usage_as_uint32_t(usage));
		}
	}

	return 1;
}

static int
evdevdevice_absinfos(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE "expected");

	lua_newtable(L); /* { ABS_X: { min: 1, max: 2, ... }, ... } */

	if (device->evdev == NULL)
		return 1;

	for (unsigned int code = 0; code <= ABS_MAX; code++) {
		const struct input_absinfo *abs =
			libevdev_get_abs_info(device->evdev, code);
		if (!abs)
			continue;

		lua_newtable(L);
		lua_pushinteger(L, abs->minimum);
		lua_setfield(L, -2, "minimum");
		lua_pushinteger(L, abs->maximum);
		lua_setfield(L, -2, "maximum");
		lua_pushinteger(L, abs->fuzz);
		lua_setfield(L, -2, "fuzz");
		lua_pushinteger(L, abs->flat);
		lua_setfield(L, -2, "flat");
		lua_pushinteger(L, abs->resolution);
		lua_setfield(L, -2, "resolution");

		evdev_usage_t usage = evdev_usage_from_code(EV_ABS, code);
		lua_rawseti(
			L,
			-2,
			evdev_usage_as_uint32_t(usage)); /* Assign to top-level table */
	}

	return 1;
}

static int
evdevdevice_udev_properties(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE "expected");

	lua_newtable(L); /* { ID_INPUT: { ... } , ... } */

	if (device->evdev == NULL)
		return 1;

	struct udev_property *prop;
	list_for_each(prop, &device->udev_properties_list, link) {
		lua_pushstring(L, prop->value);
		lua_setfield(L, -2, prop->key); /* Assign to top-level table */
	}

	return 1;
}

static int
evdevdevice_enable_evdev_usage(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE "expected");

	struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);

	evdev_usage_t usage = evdev_usage_from_uint32_t(luaL_checkinteger(L, 2));
	uint16_t type = evdev_usage_type(usage);
	uint16_t code = evdev_usage_code(usage);
	if (type > EV_MAX) {
		plugin_log_bug(plugin->parent,
			       "Ignoring invalid evdev usage %#x\n",
			       evdev_usage_as_uint32_t(usage));
		return 0;
	}

	if (device->evdev == NULL || type == EV_ABS)
		return 0;

	libevdev_enable_event_code(device->evdev, type, code, NULL);

	return 0;
}

static int
evdevdevice_disable_evdev_usage(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE "expected");

	evdev_usage_t usage = evdev_usage_from_uint32_t(luaL_checkinteger(L, 2));
	uint16_t type = evdev_usage_type(usage);
	uint16_t code = evdev_usage_code(usage);

	if (device->evdev == NULL || type > EV_MAX)
		return 0;

	libevdev_disable_event_code(device->evdev, type, code);

	return 0;
}

static int
evdevdevice_set_absinfo(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE "expected");

	evdev_usage_t usage = evdev_usage_from_uint32_t(luaL_checkinteger(L, 2));
	luaL_checktype(L, 3, LUA_TTABLE);

	if (evdev_usage_type(usage) != EV_ABS)
		return 0;

	if (!device->evdev)
		return 0;

	uint16_t code = evdev_usage_code(usage);
	const struct input_absinfo *absinfo =
		libevdev_get_abs_info(device->evdev, code);
	struct input_absinfo abs = {};
	if (absinfo)
		abs = *absinfo;

	lua_getfield(L, 3, "minimum");
	if (lua_isnumber(L, -1))
		abs.minimum = luaL_checkinteger(L, -1);
	lua_getfield(L, 3, "maximum");
	if (lua_isnumber(L, -1))
		abs.maximum = luaL_checkinteger(L, -1);
	lua_getfield(L, 3, "resolution");
	if (lua_isnumber(L, -1))
		abs.resolution = luaL_checkinteger(L, -1);
	lua_getfield(L, 3, "fuzz");
	if (lua_isnumber(L, -1))
		abs.fuzz = luaL_checkinteger(L, -1);
	lua_getfield(L, 3, "flat");
	if (lua_isnumber(L, -1))
		abs.flat = luaL_checkinteger(L, -1);

	libevdev_enable_event_code(device->evdev, EV_ABS, code, &abs);

	return 0;
}

static int
evdevdevice_connect(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE " expected");

	const char *name = luaL_checkstring(L, 2);
	luaL_checktype(L, 3, LUA_TFUNCTION);

	/* No refid means we got removed, so quietly
	 * drop any connect call */
	if (device->refid == LUA_NOREF)
		return 0;

	if (streq(name, "device-removed")) {
		register_func(L, 3, &device->device_removed_refid);
	} else if (streq(name, "evdev-frame")) {
		struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);
		libinput_plugin_enable_device_event_frame(plugin->parent,
							  device->device,
							  true);
		register_func(L, 3, &device->frame_refid);
	} else {
		return luaL_error(L, "Unknown name: %s", name);
	}

	return 0;
}

static int
evdevdevice_disconnect(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE " expected");

	const char *name = luaL_checkstring(L, 2);

	/* No refid means we got removed, so quietly
	 * drop any disconnect call */
	if (device->refid == LUA_NOREF)
		return 0;

	if (streq(name, "device-removed")) {
		unregister_func(L, &device->device_removed_refid);
	} else if (streq(name, "evdev-frame")) {
		struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);
		libinput_plugin_enable_device_event_frame(plugin->parent,
							  device->device,
							  false);
		unregister_func(L, &device->frame_refid);
	} else {
		return luaL_error(L, "Unknown name: %s", name);
	}

	return 0;
}

static struct evdev_frame *
evdevdevice_frame(lua_State *L, struct libinput_lua_plugin *plugin, EvdevDevice *device)
{
	auto frame = evdev_frame_new(64);
	lua_pop_evdev_frame(plugin, device->evdev, frame);

	struct libinput *libinput = lua_get_libinput(L);
	usec_t now = libinput_now(libinput);
	evdev_frame_set_time(frame, now);

	return frame;
}

static int
evdevdevice_prepend_frame(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE " expected");

	luaL_checktype(L, 2, LUA_TTABLE);

	/* No refid means we got removed, so quietly
	 * drop any disconnect call */
	if (device->refid == LUA_NOREF)
		return 0;

	struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);
	_unref_(evdev_frame) *frame = evdevdevice_frame(L, plugin, device);
	/* FIXME: need to really ensure that the device can never be dangling */
	libinput_plugin_prepend_evdev_frame(plugin->parent, device->device, frame);

	return 0;
}

static int
evdevdevice_append_frame(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE " expected");

	luaL_checktype(L, 2, LUA_TTABLE);

	/* No refid means we got removed, so quietly
	 * drop any disconnect call */
	if (device->refid == LUA_NOREF)
		return 0;

	struct libinput_lua_plugin *plugin = lua_get_libinput_lua_plugin(L);
	_unref_(evdev_frame) *frame = evdevdevice_frame(L, plugin, device);

	/* FIXME: need to really ensure that the device can never be dangling */
	libinput_plugin_append_evdev_frame(plugin->parent, device->device, frame);

	return 0;
}

static int
evdevdevice_disable_feature(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE " expected");

	const char *feature = luaL_checkstring(L, 2);

	/* No refid means we got removed, so quietly
	 * drop any call */
	if (device->refid == LUA_NOREF)
		return 0;

	const struct {
		const char *name;
		enum libinput_feature feature;
	} map[] = {
		{ "button-debouncing", LIBINPUT_FEATURE_BUTTON_DEBOUNCING },
		{ "wheel-debouncing", LIBINPUT_FEATURE_WHEEL_DEBOUNCING },
		{ "touchpad-jump-detection", LIBINPUT_FEATURE_TOUCHPAD_JUMP_DETECTION },
		{ "touchpad-palm-detection", LIBINPUT_FEATURE_TOUCHPAD_PALM_DETECTION },
		{ "touchpad-hysteresis", LIBINPUT_FEATURE_TOUCHPAD_HYSTERESIS },
	};

	ARRAY_FOR_EACH(map, m) {
		if (streq(feature, m->name)) {
			struct libinput_lua_plugin *plugin =
				lua_get_libinput_lua_plugin(L);
			libinput_plugin_disable_device_feature(plugin->parent,
							       device->device,
							       m->feature);
			return 0;
		}
	}

	return luaL_error(L, "Unknown feature: %s", feature);
}

static int
evdevdevice_gc(lua_State *L)
{
	EvdevDevice *device = luaL_checkudata(L, 1, EVDEV_DEVICE_METATABLE);
	luaL_argcheck(L, device != NULL, 1, EVDEV_DEVICE_METATABLE "expected");

	list_remove(&device->link);
	struct udev_property *prop;
	list_for_each_safe(prop, &device->udev_properties_list, link) {
		udev_property_destroy(prop);
	}
	free(device->name);

	return 0;
}

static const struct luaL_Reg evdevdevice_vtable[] = {
	{ "info", evdevdevice_info },
	{ "name", evdevdevice_name },
	{ "usages", evdevdevice_usages },
	{ "absinfos", evdevdevice_absinfos },
	{ "udev_properties", evdevdevice_udev_properties },
	{ "enable_evdev_usage", evdevdevice_enable_evdev_usage },
	{ "disable_evdev_usage", evdevdevice_disable_evdev_usage },
	{ "set_absinfo", evdevdevice_set_absinfo },
	{ "connect", evdevdevice_connect },
	{ "disconnect", evdevdevice_disconnect },
	{ "prepend_frame", evdevdevice_prepend_frame },
	{ "append_frame", evdevdevice_append_frame },
	{ "disable_feature", evdevdevice_disable_feature },
	{ "__gc", evdevdevice_gc },
	{ NULL, NULL }
};

static void
evdevdevice_init(lua_State *L)
{
	luaL_newmetatable(L, EVDEV_DEVICE_METATABLE);
	lua_pushstring(L, "__index");
	lua_pushvalue(L, -2); /* push metatable */
	lua_settable(L, -3);  /* metatable.__index = metatable */
	luaL_setfuncs(L, evdevdevice_vtable, 0);
}

static void
libinput_lua_plugin_destroy(struct libinput_lua_plugin *plugin)
{
	if (plugin->timer)
		libinput_plugin_timer_cancel(plugin->timer);

	EvdevDevice *evdev;
	list_for_each_safe(evdev, &plugin->evdev_devices, link) {
		remove_device(plugin, evdev);
	}

	if (plugin->timer)
		plugin->timer = libinput_plugin_timer_unref(plugin->timer);
	if (plugin->L)
		lua_close(plugin->L);
	free(plugin);
}

DEFINE_DESTROY_CLEANUP_FUNC(libinput_lua_plugin);

static void
libinput_plugin_destroy(struct libinput_plugin *libinput_plugin)
{
	struct libinput_lua_plugin *plugin =
		libinput_plugin_get_user_data(libinput_plugin);
	if (plugin)
		libinput_lua_plugin_destroy(plugin);
}

static void
libinput_lua_plugin_run(struct libinput_plugin *libinput_plugin)
{
	struct libinput_lua_plugin *plugin =
		libinput_plugin_get_user_data(libinput_plugin);
	struct lua_State *L = plugin->L;

	assert(lua_isfunction(L, -1));

	/* Main entry point into the plugin, so we need to push our sandbox
	 * as _ENV. This only needs to be done here because all other entry
	 * points into the Lua script are callbacks that are set up during
	 * this run (and thus share the _ENV)
	 */
	lua_pushvalue(L, plugin->sandbox_table_idx);
	const char *upval = lua_setupvalue(L, -2, 1);
	if (!upval || !streq(upval, "_ENV")) {
		plugin_log_bug_libinput(libinput_plugin, "Failed to set up sandbox\n");
		libinput_plugin_unregister(libinput_plugin);
		return;
	}

	if (libinput_lua_pcall(plugin, 0, 0) && !plugin->register_called) {
		plugin_log_bug(libinput_plugin,
			       "plugin never registered, unloading plugin\n");
		libinput_plugin_unregister(libinput_plugin);
		/* plugin system will destroy the plugin later */
	}
}

static void
libinput_lua_init_evdev_global(lua_State *L, int sandbox_table_idx)
{
	lua_newtable(L);
	for (unsigned int t = 0; t <= EV_MAX; t++) {
		const char *typename = libevdev_event_type_get_name(t);
		if (!typename)
			continue;

		int max = libevdev_event_type_get_max(t);
		if (max < 0)
			continue;

		for (int i = 0; i < max; i++) {
			const char *name = libevdev_event_code_get_name(t, i);
			if (!name)
				continue;

			evdev_usage_t usage = evdev_usage_from_code(t, i);
			lua_pushinteger(L, evdev_usage_as_uint32_t(usage));
			lua_setfield(L, -2, name);
		}
	}

#define pushbus(name, value) do { \
		lua_pushinteger(L, value); \
		lua_setfield(L, -2, #name); \
	} while (0)

	pushbus(BUS_PCI, 0x01);
	pushbus(BUS_ISAPNP, 0x02);
	pushbus(BUS_USB, 0x03);
	pushbus(BUS_HIL, 0x04);
	pushbus(BUS_BLUETOOTH, 0x05);
	pushbus(BUS_VIRTUAL, 0x06);

	pushbus(BUS_ISA, 0x10);
	pushbus(BUS_I8042, 0x11);
	pushbus(BUS_XTKBD, 0x12);
	pushbus(BUS_RS232, 0x13);
	pushbus(BUS_GAMEPORT, 0x14);
	pushbus(BUS_PARPORT, 0x15);
	pushbus(BUS_AMIGA, 0x16);
	pushbus(BUS_ADB, 0x17);
	pushbus(BUS_I2C, 0x18);
	pushbus(BUS_HOST, 0x19);
	pushbus(BUS_GSC, 0x1A);
	pushbus(BUS_ATARI, 0x1B);
	pushbus(BUS_SPI, 0x1C);
	pushbus(BUS_RMI, 0x1D);
	pushbus(BUS_CEC, 0x1E);
	pushbus(BUS_INTEL_ISHTP, 0x1F);
	pushbus(BUS_AMD_SFH, 0x20);

#undef pushbus

	lua_setfield(L, sandbox_table_idx, "evdev");
}

static const struct libinput_plugin_interface interface = {
	.run = libinput_lua_plugin_run,
	.destroy = libinput_plugin_destroy,
	.device_new = libinput_lua_plugin_device_new,
	.device_ignored = libinput_lua_plugin_device_ignored,
	.device_added = NULL,
	.device_removed = libinput_lua_plugin_device_removed,
	.evdev_frame = libinput_lua_plugin_evdev_frame,
};

static lua_State *
libinput_lua_plugin_init_lua(struct libinput *libinput,
			     struct libinput_lua_plugin *plugin)
{
	lua_State *L = luaL_newstate();
	if (!L)
		return NULL;

	/* This will be our _ENV later, see libinput_lua_pcall */
	lua_newtable(L);
	int sandbox_table_idx = lua_gettop(L);
	plugin->sandbox_table_idx = sandbox_table_idx;

	/* Load the modules we want to (partially) expose.
	 * An (outdated?) list of safe function is here:
	 * http://lua-users.org/wiki/SandBoxes
	 *
	 * Math, String and Table seem to be safe given that our plugins
	 * all have their own individual sandbox.
	 */

	luaL_requiref(L, LUA_GNAME, luaopen_base, 0);
	static const char *allowed_funcs[] = {
		"assert", "error",    "ipairs",   "next", "pcall",  "pairs",    "print",
		"select", "tonumber", "tostring", "type", "xpcall", "_VERSION",
	};
	ARRAY_FOR_EACH(allowed_funcs, func) {
		lua_getfield(L, -1, *func);
		lua_setfield(L, sandbox_table_idx, *func);
	}
	lua_pop(L, 1);

	/* Math is fine as a whole */
	luaL_requiref(L, LUA_MATHLIBNAME, luaopen_math, 0);
	lua_setfield(L, sandbox_table_idx, "math");

	/* Table is fine as a whole */
	luaL_requiref(L, LUA_TABLIBNAME, luaopen_table, 0);
	lua_setfield(L, sandbox_table_idx, "table");

	/* String is fine as a whole */
	luaL_requiref(L, LUA_STRLIBNAME, luaopen_string, 0);
	lua_setfield(L, sandbox_table_idx, "string");

	/* Override metatable to prevent access to unregistered globals */
	lua_pushvalue(L, sandbox_table_idx);
	lua_newtable(L);
	lua_pushnil(L);
	lua_setfield(L, -2, "__index");
	lua_setmetatable(L, sandbox_table_idx);

	/* Our objects */
	libinputplugin_init(L);
	evdevdevice_init(L);

	/* Our globals */
	lua_newtable(L);
	libinput_lua_init_evdev_global(L, sandbox_table_idx);

	/* The libinput global object */
	lua_newuserdata(L, sizeof(LibinputPlugin));
	luaL_getmetatable(L, PLUGIN_METATABLE);
	lua_setmetatable(L, -2);
	lua_setfield(L, sandbox_table_idx, "libinput");

	/* Make struct libinput available in our callbacks */
	lua_pushlightuserdata(L, (void *)&libinput_key);
	lua_pushlightuserdata(L, libinput);
	lua_settable(L, LUA_REGISTRYINDEX);

	/* Make struct libinput_lua_plugin available in our callbacks */
	lua_pushlightuserdata(L, (void *)&libinput_lua_plugin_key);
	lua_pushlightuserdata(L, plugin);
	lua_settable(L, LUA_REGISTRYINDEX);

	return L;
}

struct libinput_plugin *
libinput_lua_plugin_new_from_path(struct libinput *libinput, const char *path)
{
	_destroy_(libinput_lua_plugin) *plugin = zalloc(sizeof(*plugin));
	_autofree_ char *name = safe_strdup(safe_basename(path));

	/* libinput's plugin system keeps a ref, we don't need
	 * a separate ref here, the plugin system will outlast us.
	 */
	_unref_(libinput_plugin) *p =
		libinput_plugin_new(libinput, name, &interface, NULL);

	plugin->parent = p;
	plugin->register_called = false;
	plugin->version = LIBINPUT_PLUGIN_VERSION;
	plugin->device_new_refid = LUA_NOREF;
	plugin->timer_expired_refid = LUA_NOREF;
	list_init(&plugin->evdev_devices);

	_cleanup_(lua_closep) lua_State *L =
		libinput_lua_plugin_init_lua(libinput, plugin);
	if (!L) {
		plugin_log_bug(plugin->parent,
			       "Failed to create lua state for %s\n",
			       name);
		libinput_plugin_unregister(p);
		return NULL;
	}

	int ret = luaL_loadfile(L, path);
	if (ret == LUA_OK) {
		plugin->L = steal(&L);

		libinput_plugin_set_user_data(p, steal(&plugin));
		return p;
	} else {
		const char *lua_error = lua_tostring(L, -1);
		const char *error = lua_error;
		if (!error) {
			switch (ret) {
			case LUA_ERRMEM:
				error = "out of memory";
				break;
			case LUA_ERRFILE:
				error = "file not found or not readable";
				break;
			case LUA_ERRSYNTAX:
				error = "syntax error";
				break;
			default:
				break;
			}
		}

		if (ret == LUA_ERRSYNTAX &&
		    log_is_logged(libinput, LIBINPUT_LOG_PRIORITY_DEBUG)) {
			luaL_traceback(L, L, NULL, 1);
			for (int i = -1; i > -4; i--) {
				const char *msg = lua_tostring(L, i);
				if (!msg)
					break;
				log_debug(libinput, "%s %s\n", name, msg);
			}
			lua_pop(L, 1); /* traceback */
		}

		plugin_log_bug(plugin->parent, "Failed to load %s: %s\n", path, error);

		lua_pop(L, 1); /* the lua_error message */

		libinput_plugin_unregister(p);

		return NULL;
	}
}
