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

#include <fcntl.h>
#include <inttypes.h>

#include "util-files.h"
#include "util-strings.h"
#include "util-time.h"

#include "libinput.h"
#include "litest.h"

static char *
_litest_write_plugin(const char *tmpdir, const char *filename, const char *content)
{
	static int counter = 0;
	counter += 10;

	char *path = strdup_printf("%s/%d-%s.lua", tmpdir, counter, filename);
	_autoclose_ int fd = open(path, O_WRONLY | O_CREAT, 0644);
	litest_assert_errno_success(fd);

	if (content) {
		write(fd, content, strlen(content));
		fsync(fd);
	}

	return path;
}

#define litest_write_plugin(tmpdir_, content_) \
	_litest_write_plugin(tmpdir_, __func__, content_)

START_TEST(lua_load_failure)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	const char *lua = "asfasdk1298'..asdfasdf'123@2;asd"; /* invalid lua */
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		size_t index = 0;
		litest_assert(
			strv_find_substring(capture->errors, "Failed to load", &index));
		litest_assert_str_in(path, capture->errors[index]);
	}
}
END_TEST

enum content {
	EMPTY,
	NOTHING,
	BASIC,
	COMMENT,
	DUPLICATE_CALL,
	VERSION_NOT_A_TABLE,
	MISSING_REGISTER,
};

START_TEST(lua_load_success_but_no_register)
{
	enum content content = litest_test_param_get_i32(test_env->params, "content");

	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);

	const char *lua = NULL;
	switch (content) {
	case DUPLICATE_CALL:
		lua = "v1 = libinput:register({1})\np2 = libinput:register({2})\n";
		break;
	case VERSION_NOT_A_TABLE:
		lua = "v1 = libinput:register(1)\n";
		break;
	case MISSING_REGISTER:
		lua = "libinput:connect(\"new-evdev-device\", function(device) assert(false) end)\n";
		break;
	case BASIC:
		lua = "a = 1 + 10";
		break;
	case COMMENT:
		lua = "-- near-empty file";
		break;
	case NOTHING:
		lua = "";
		break;
	case EMPTY:
		break;
	}

	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		switch (content) {
		case VERSION_NOT_A_TABLE:
			litest_assert_strv_substring(capture->errors,
						     "unloading after error");
			break;
		case DUPLICATE_CALL:
			litest_assert_strv_substring(capture->errors,
						     "plugin already registered");
			break;
		default:
			litest_assert_strv_substring(capture->errors,
						     "plugin never registered");
			break;
		}
	}
}
END_TEST

START_TEST(lua_register_noop)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	const char *lua = "libinput:register({1})";
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);
		litest_assert_logcapture_no_errors(capture);
	}
}
END_TEST

START_TEST(lua_unregister_is_last)
{
	const char *when = litest_test_param_get_string(test_env->params, "when");

	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	_autofree_ char *lua = strdup_printf(
		"libinput:register({1})\n"
		"libinput:connect(\"new-evdev-device\", function(device)\n  %s\n  libinput:log_error(\"abort abort\")\nend)\n"
		"%slibinput:log_error(\"must not happen\")",
		streq(when, "connect") ? "libinput:unregister()" : "",
		streq(when, "run") ? "libinput:unregister()\n" : "--");
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);

	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		_destroy_(litest_device) *device = litest_add_device(li, LITEST_MOUSE);
		litest_drain_events(li);
		litest_assert_logcapture_no_errors(capture);
	}
}
END_TEST

START_TEST(lua_test_logging)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);

	enum libinput_log_priority priority =
		litest_test_param_get_i32(test_env->params, "priority");

	const char *lua = NULL;
	switch (priority) {
	case LIBINPUT_LOG_PRIORITY_DEBUG:
		lua = "libinput:log_debug(\"deb-ug\");";
		break;
	case LIBINPUT_LOG_PRIORITY_INFO:
		lua = "libinput:log_info(\"inf-o\");";
		break;
	case LIBINPUT_LOG_PRIORITY_ERROR:
		lua = "libinput:log_error(\"err-or\");";
		break;
	default:
		litest_assert_not_reached();
		break;
	}

	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	libinput_log_set_priority(li, priority);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		switch (priority) {
		case LIBINPUT_LOG_PRIORITY_DEBUG:
			litest_assert(
				strv_find_substring(capture->debugs, "deb-ug", NULL));
			litest_assert(
				!strv_find_substring(capture->infos, "inf-o", NULL));
			litest_assert(
				!strv_find_substring(capture->errors, "err-or", NULL));
			break;
		case LIBINPUT_LOG_PRIORITY_INFO:
			litest_assert(
				!strv_find_substring(capture->debugs, "deb-ug", NULL));
			litest_assert(
				strv_find_substring(capture->infos, "inf-o", NULL));
			litest_assert(
				!strv_find_substring(capture->errors, "err-or", NULL));
			break;
		case LIBINPUT_LOG_PRIORITY_ERROR:
			litest_assert(
				!strv_find_substring(capture->debugs, "deb-ug", NULL));
			litest_assert(
				!strv_find_substring(capture->infos, "inf-o", NULL));
			litest_assert(
				strv_find_substring(capture->errors, "err-or", NULL));
			break;
		}
	}
}
END_TEST

START_TEST(lua_test_evdev_global)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);

	/* This is generated, if a few of them work the
	 * rest should work too */
	const char *lua =
		"libinput:register({1}); a = evdev.ABS_X\nb = evdev.REL_X\nc = evdev.KEY_A\nd = evdev.EV_SYN\n";
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		litest_assert_logcapture_no_errors(capture);
	}
}
END_TEST

START_TEST(lua_test_libinput_now)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	const char *lua = "libinput:log_error(\">>>\" .. libinput:now())";
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		usec_t test_now;
		int rc = now_in_us(&test_now);
		litest_assert_neg_errno_success(rc);

		size_t index = 0;
		litest_assert(strv_find_substring(capture->errors, ">>>", &index));
		size_t nelem = 0;
		_autostrvfree_ char **tokens =
			strv_from_string(capture->errors[index], ">>>", &nelem);
		litest_assert_int_eq(nelem, 2U);

		uint64_t plugin_now = strtoull(tokens[1], NULL, 10);

		litest_assert_int_le(plugin_now, usec_as_uint64_t(test_now));
		/* Even a slow test runner hopefully doesn't take >300ms to get to the
		 * log print */
		litest_assert_int_gt(
			plugin_now,
			usec_as_uint64_t(usec_sub(test_now, usec_from_millis(300))));
	}
}
END_TEST

START_TEST(lua_test_libinput_timer)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);

	const char *mode = litest_test_param_get_string(test_env->params, "mode");
	bool reschedule = litest_test_param_get_bool(test_env->params, "reschedule");

	_autofree_ char *timeout =
		strdup_printf("%s%" PRIu64,
			      streq(mode, "absolute") ? "libinput:now() + " : "",
			      usec_as_uint64_t(usec_from_millis(100)));
	_autofree_ char *reschedule_timeout =
		strdup_printf("libinput:timer_set_%s(%s%" PRIu64 ")\n",
			      mode,
			      streq(mode, "absolute") ? "t + " : "",
			      usec_as_uint64_t(usec_from_millis(100)));
	_autofree_ char *lua = strdup_printf(
		"libinput:register({1})\n"
		"libinput:connect(\"timer-expired\",\n"
		"          function(t)\n"
		"               libinput:log_error(\">>>\" .. t)\n"
		"               %s\n"
		"          end)\n"
		"libinput:timer_set_%s(%s)\n",
		reschedule ? reschedule_timeout : "",
		mode,
		timeout);

	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		litest_assert_logcapture_no_errors(capture);

		size_t nloops = reschedule ? 4 : 1;
		for (size_t i = 0; i < nloops; i++) {
			libinput_dispatch(li);
			msleep(100);
			libinput_dispatch(li);

			usec_t test_now;
			int rc = now_in_us(&test_now);
			litest_assert_neg_errno_success(rc);

			_autostrvfree_ char **msg = steal(&capture->errors);
			litest_assert_ptr_notnull(msg);
			size_t index;
			litest_assert(strv_find_substring(msg, ">>>", &index));

			size_t nelem = 0;
			_autostrvfree_ char **tokens =
				strv_from_string(msg[index], ">>>", &nelem);
			litest_assert_int_eq(nelem, 2U);

			uint64_t plugin_now = strtoull(tokens[1], NULL, 10);
			litest_assert_int_le(plugin_now, usec_as_uint64_t(test_now));
			/* Even a slow test runner hopefully doesn't take >300ms between
			 * dispatch and now_in_us */
			litest_assert_int_gt(
				plugin_now,
				usec_as_uint64_t(
					usec_sub(test_now, usec_from_millis(300))));
		}

		if (!reschedule) {
			libinput_dispatch(li);
			msleep(120);
			libinput_dispatch(li);
		}

		litest_assert_logcapture_no_errors(capture);
	}
}
END_TEST

enum connect_error {
	BAD_TYPE,
	TOO_FEW_ARGS,
	TOO_MANY_ARGS,
};

START_TEST(lua_bad_connect)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);

	const char *handler = litest_test_param_get_string(test_env->params, "handler");
	enum connect_error error = litest_test_param_get_i32(test_env->params, "error");

	const char *func = NULL;
	switch (error) {
	case BAD_TYPE:
		func = "a";
		break;
	case TOO_FEW_ARGS:
		func = "function(p) libinput:log_debug(\"few\"); end";
		break;
	case TOO_MANY_ARGS:
		func = "function(p, a, b) libinput:log_debug(\"many\"); end";
		break;
	}

	_autofree_ char *lua = strdup_printf(
		"libinput:register({1})\n"
		"a = 10\n"
		"libinput:connect(\"%s\", %s)\n",
		handler,
		func);

	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		switch (error) {
		/* These don't trigger a lua erro so we just test they don't segfault us
		 */
		case TOO_FEW_ARGS:
		case TOO_MANY_ARGS:
			litest_assert_logcapture_no_errors(capture);
			break;
		case BAD_TYPE:
			litest_assert_strv_substring(capture->errors,
						     "bad argument #2 to 'connect'");
			break;
		}
	}
}
END_TEST

START_TEST(lua_register_multiversions)
{

	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	const char *lua =
		"v = libinput:register({1, 3, 4, 10, 15})\nlibinput:log_info(\"VERSION:\" .. v)\n";
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	if (libinput_log_get_priority(li) > LIBINPUT_LOG_PRIORITY_INFO)
		libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_INFO);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		litest_assert_logcapture_no_errors(capture);
		litest_assert_strv_substring(capture->infos, "VERSION:1");
	}
}
END_TEST

START_TEST(lua_allowed_functions)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	/* This tests on the assumption that if some of these work,
	 * then the others we allow will work too. */
	const char *lua =
		"\n"
		"libinput:register({1})\n"
		"a = {10, 20}\n"
		"for _, v in ipairs(a) do\n"
		"   v = v + 1\n"
		"end\n"
		"b = {foo = 1}"
		"for k, v in pairs(a) do\n"
		"   v = v + 1\n"
		"end\n"
		"print(math.maxinteger)\n"
		"table.sort({10, 2, 4})\n"
		"assert(true)\n"
		"";
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		litest_assert_logcapture_no_errors(capture);
	}
}
END_TEST

START_TEST(lua_disallowed_functions)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	/* This tests on the assumption that if some of these work,
	 * then the others we allow will work too. */
	const char *lua =
		"\n"
		"libinput:register({1})\n"
		"assert(io == nil)\n"
		"assert(require == nil)\n"
		"assert(rawget == nil)\n"
		"assert(rawset == nil)\n"
		"assert(setfenv == nil)\n"
		"assert(getmetatable == nil)\n"
		"assert(setmetatable == nil)\n"
		"assert(package == nil)\n"
		"assert(os == nil)\n"
		"assert(debug == nil)\n"
		"";
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		litest_assert_logcapture_no_errors(capture);
	}
}
END_TEST

START_TEST(lua_frame_handler)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	const char *lua =
		"libinput:register({1})\n"
		"function frame_handler(_, frame, timestamp)\n"
		"  libinput:log_info(\"T:\" .. timestamp)\n"
		"  for _, e in ipairs(frame) do\n"
		"	libinput:log_info(\"E:\" .. e.usage .. \":\" .. e.value)\n"
		"  end\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", function(device) device:connect(\"evdev-frame\", frame_handler) end)\n";

	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	if (libinput_log_get_priority(li) > LIBINPUT_LOG_PRIORITY_INFO)
		libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_INFO);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		_destroy_(litest_device) *device = litest_add_device(li, LITEST_MOUSE);
		litest_drain_events(li);

		usec_t before, after;
		now_in_us(&before);
		msleep(1);
		litest_button_click_debounced(device, li, BTN_LEFT, 1);
		litest_button_click_debounced(device, li, BTN_LEFT, 0);
		litest_assert_logcapture_no_errors(capture);
		msleep(1);
		now_in_us(&after);

		/* EV_KEY << 16 | BTN_LEFT -> 65808 */

		litest_assert_strv_substring(capture->infos, "E:65808:1");
		litest_assert_strv_substring(capture->infos, "E:65808:0");
		/* SYN_REPORT shouldn't show up in the frame */
		litest_assert(!strv_find_substring(capture->infos, "E:0:0", NULL));

		size_t idx;
		litest_assert(strv_find_substring(capture->infos, "T:", &idx));

		_autofree_ char *str = safe_strdup(capture->infos[idx]);
		for (size_t i = 0; str[i]; i++) {
			if (str[i] == '\n') {
				str[i] = '\0';
				break;
			}
		}

		size_t nelems;
		_autostrvfree_ char **split = strv_from_string(str, ":", &nelems);
		litest_assert_int_gt(nelems, 1U);
		char *strtime = split[nelems - 1];
		uint64_t timestamp = 0;
		litest_assert(safe_atou64(strtime, &timestamp));
		litest_assert_int_gt(timestamp, usec_as_uint64_t(before));
		litest_assert_int_lt(timestamp, usec_as_uint64_t(after));
	}
}
END_TEST

START_TEST(lua_device_info)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	const char *lua =
		"libinput:register({1})\n"
		"function info_printer(device)\n"
		"  local info = device:info()\n"
		"  libinput:log_info(\"BUS:\" .. info.bustype)\n"
		"  libinput:log_info(\"VID:\" .. info.vid)\n"
		"  libinput:log_info(\"PID:\" .. info.pid)\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", info_printer)\n";

	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	if (libinput_log_get_priority(li) > LIBINPUT_LOG_PRIORITY_INFO)
		libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_INFO);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		_destroy_(litest_device) *device = litest_add_device(li, LITEST_MOUSE);
		litest_drain_events(li);

		litest_assert_strv_substring(capture->infos, "BUS:3");
		litest_assert_strv_substring(capture->infos, "VID:6127");
		litest_assert_strv_substring(capture->infos, "PID:24601");
	}
}
END_TEST

START_TEST(lua_set_absinfo)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	const char *lua =
		"libinput:register({1})\n"
		"function absinfo_setter(device)\n"
		"  local absinfos = device:absinfos()\n"
		"  for u, a in pairs(absinfos) do\n"
		"	libinput:log_info(\"A:\" .. u .. \":\" .. a.minimum .. \":\" .. a.maximum .. \":\" .. a.resolution .. \":\" .. a.fuzz .. \":\" .. a.flat)\n"
		"  end\n"
		"  device:set_absinfo(evdev.ABS_X, { minimum = 0, maximum = 1000, resolution = 100 })\n"
		"  device:set_absinfo(evdev.ABS_Y, { minimum = 0, maximum = 200, resolution = 10 })\n"
		"  device:set_absinfo(evdev.ABS_MT_POSITION_X, { minimum = 0, maximum = 1000, resolution = 100 })\n"
		"  device:set_absinfo(evdev.ABS_MT_POSITION_Y, { minimum = 0, maximum = 200, resolution = 10 })\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", absinfo_setter)\n";

	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	if (libinput_log_get_priority(li) > LIBINPUT_LOG_PRIORITY_INFO)
		libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_INFO);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		_destroy_(litest_device) *device =
			litest_add_device(li, LITEST_GENERIC_MULTITOUCH_SCREEN);
		litest_drain_events(li);

		litest_assert_logcapture_no_errors(capture);

		for (int code = 0; code <= ABS_MAX; code++) {
			if (!libevdev_has_event_code(device->evdev, EV_ABS, code)) {
				_autofree_ char *prefix =
					strdup_printf("A:%u", (EV_ABS << 16) | code);
				litest_assert(!strv_find_substring(capture->infos,
								   prefix,
								   NULL));
				continue;
			}

			const struct input_absinfo *absinfo =
				libevdev_get_abs_info(device->evdev, code);
			_autofree_ char *message = strdup_printf("A:%u:%d:%d:%d:%d:%d",
								 (EV_ABS << 16) | code,
								 absinfo->minimum,
								 absinfo->maximum,
								 absinfo->resolution,
								 absinfo->fuzz,
								 absinfo->flat);
			litest_assert_strv_substring(capture->infos, message);
		}

		/* If the absinfo worked, our device is 10x20mm big */
		double w, h;
		libinput_device_get_size(device->libinput_device, &w, &h);
		litest_assert_double_eq(w, 10.0);
		litest_assert_double_eq(h, 20.0);
	}
}
END_TEST

START_TEST(lua_enable_disable_evdev_usage)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	/* We have two plugins here, one that enables codes and one that prints
	 * the frame.
	 *
	 * The first plugin also inserts a REL_Z event into the frame since we
	 * can't send that through the kernel.
	 */
	const char *lua1 =
		"libinput:register({1})\n"
		"function frame_handler(_, frame, timestamp)\n"
		"  table.insert(frame, { usage = evdev.REL_Z, value = 3 })\n"
		"  return frame\n"
		"end\n"
		"function enabler(device)\n"
		"  device:enable_evdev_usage(evdev.REL_Z)\n"
		"  device:enable_evdev_usage(evdev.BTN_STYLUS2)\n"
		"  device:disable_evdev_usage(evdev.REL_WHEEL)\n"
		"  device:connect(\"evdev-frame\", frame_handler)\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", enabler)\n";

	const char *lua2 =
		"libinput:register({1})\n"
		"function frame_handler(_, frame, timestamp)\n"
		"  libinput:log_info(\"frame\")\n"
		"  for _, e in ipairs(frame) do\n"
		"	libinput:log_info(\"E:\" .. e.usage .. \":\" .. e.value)\n"
		"  end\n"
		"end\n"
		"function f(device)\n"
		"  libinput:log_info(\"F: \" .. device:name())\n"
		"  device:connect(\"evdev-frame\", frame_handler)\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", f)\n";

	_autofree_ char *p1 = litest_write_plugin(tmpdir->path, lua1);
	_autofree_ char *p2 = litest_write_plugin(tmpdir->path, lua2);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	if (libinput_log_get_priority(li) > LIBINPUT_LOG_PRIORITY_INFO)
		libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_INFO);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		_destroy_(litest_device) *device = litest_add_device(li, LITEST_MOUSE);
		litest_drain_events(li);

		/* We enabled that one ourselves */
		litest_assert(
			libinput_device_pointer_has_button(device->libinput_device,
							   BTN_STYLUS2));

		litest_assert_logcapture_no_errors(capture);

		litest_event(device, EV_REL, REL_X, 1);
		litest_event(device, EV_REL, REL_Y, 2);
		litest_event(device, EV_REL, REL_WHEEL, -1);
		litest_event(device, EV_SYN, SYN_REPORT, 0);
		litest_dispatch(li);

		litest_assert_logcapture_no_errors(capture);

		litest_assert_strv_substring(capture->infos, "E:131072:1");
		litest_assert_strv_substring(capture->infos, "E:131073:2");
		litest_assert_strv_substring(capture->infos, "E:131074:3");
		litest_assert(!strv_find_substring(capture->infos, "E:131080", NULL));
	}
}
END_TEST

START_TEST(lua_udev_properties)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	const char *lua =
		"libinput:register({1})\n"
		"function prop_printer(device)\n"
		"  local properties = device:udev_properties()\n"
		"  for k, v in pairs(properties) do\n"
		"	libinput:log_info(k .. \"=\" .. v)\n"
		"  end\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", prop_printer)\n";

	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	if (libinput_log_get_priority(li) > LIBINPUT_LOG_PRIORITY_INFO)
		libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_INFO);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		enum litest_device_type which =
			litest_test_param_get_i32(test_env->params, "which");
		_destroy_(litest_device) *device = litest_add_device(li, which);
		litest_drain_events(li);

		litest_assert_logcapture_no_errors(capture);

		switch (which) {
		case LITEST_TRACKPOINT:
			litest_assert_strv_substring(capture->infos,
						     "ID_INPUT_POINTINGSTICK=1");
			_fallthrough_;
		case LITEST_MOUSE:
			litest_assert_strv_substring(capture->infos,
						     "ID_INPUT_MOUSE=1");
			break;
		case LITEST_GENERIC_MULTITOUCH_SCREEN:
			litest_assert_strv_substring(capture->infos,
						     "ID_INPUT_TOUCHSCREEN=1");
			break;
		default:
			litest_assert_not_reached();
			break;
		}
		litest_assert(!strv_find_substring(capture->infos,
						   "ID_INPUT_WIDTH_MM",
						   NULL));
		litest_assert(!strv_find_substring(capture->infos,
						   "ID_INPUT_WIDTH_MM",
						   NULL));
	}
}
END_TEST

START_TEST(lua_append_prepend_frame)
{
	bool append = litest_test_param_get_bool(test_env->params, "append");
	bool in_timer = litest_test_param_get_bool(test_env->params, "in_timer");
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	_autofree_ char *lua = strdup_printf(
		"libinput:register({1})\n"
		"mydev = nil\n"
		"function frame_handler(device, frame, timestamp)\n"
		"    device:%s_frame({{ usage = evdev.BTN_LEFT, value = 1}})\n" /* commented
										   out
										   if
										   !in_timer
										 */
		"    return nil\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", function(device)\n"
		"    mydev = device\n"
		"    %sdevice:connect(\"evdev-frame\", frame_handler)\n"
		"    %slibinput:timer_set_relative(200000)\n" /* commented out if
								 !in_timer */
		"end)\n"
		"function timer_expired(t)\n"
		"    mydev:%s_frame({{ usage = evdev.BTN_LEFT, value = 1 }})\n"
		"end\n"
		"libinput:connect(\"timer-expired\", timer_expired)\n",
		append ? "append" : "prepend",
		in_timer ? "-- " : "",
		in_timer ? "" : "-- ",
		append ? "append" : "prepend");
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	libinput_plugin_system_load_plugins(li, LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
	litest_drain_events(li);

	_destroy_(litest_device) *device = litest_add_device(li, LITEST_MOUSE);
	litest_drain_events(li);
	msleep(10); /* trigger the timer, if any */
	litest_dispatch(li);

	if (in_timer) {
		litest_assert_button_event(li, BTN_LEFT, LIBINPUT_BUTTON_STATE_PRESSED);
	}

	litest_event(device, EV_REL, REL_X, 1);
	litest_event(device, EV_REL, REL_Y, 2);
	litest_event(device, EV_SYN, SYN_REPORT, 0);
	litest_dispatch(li);
	litest_timeout_debounce(li);
	litest_dispatch(li);

	if (!in_timer && !append) {
		litest_assert_button_event(li, BTN_LEFT, LIBINPUT_BUTTON_STATE_PRESSED);
	}

	_destroy_(libinput_event) *ev = libinput_get_event(li);
	litest_is_motion_event(ev);

	if (!in_timer && append) {
		litest_assert_button_event(li, BTN_LEFT, LIBINPUT_BUTTON_STATE_PRESSED);
	}

	litest_assert_empty_queue(li);
}
END_TEST

START_TEST(lua_ignore_unsupported_codes)
{
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	_autofree_ char *lua = strdup_printf(
		"libinput:register({1})\n"
		"function frame_handler(device, frame, timestamp)\n"
		"    local events = {}\n"
		"    for _, v in ipairs(frame) do\n"
		"       table.insert(events, { usage = v.usage, value = v.value })\n"
		"    end\n"
		"    table.insert(events, { usage = evdev.ABS_X, value = 1000 })\n"
		"    table.insert(events, { usage = evdev.ABS_Y, value = 100 })\n"
		"    table.insert(events, { usage = evdev.BTN_BACK, value = 1 })\n"
		"    table.insert(events, { usage = evdev.BTN_LEFT, value = 1 })\n" /* this
										       one actually exists */
		"    return events\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", function(device)\n"
		"    device:connect(\"evdev-frame\", frame_handler)\n"
		"end)\n");
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);
	libinput_plugin_system_load_plugins(li, LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
	litest_drain_events(li);

	_destroy_(litest_device) *device = litest_add_device(li, LITEST_MOUSE);
	litest_drain_events(li);

	litest_event(device, EV_REL, REL_X, 1);
	litest_event(device, EV_REL, REL_Y, 2);
	litest_event(device, EV_SYN, SYN_REPORT, 0);
	litest_dispatch(li);
	litest_timeout_debounce(li);
	litest_dispatch(li);

	_destroy_(libinput_event) *ev = libinput_get_event(li);
	litest_is_motion_event(ev);
	litest_assert_button_event(li, BTN_LEFT, LIBINPUT_BUTTON_STATE_PRESSED);

	litest_assert_empty_queue(li);
}
END_TEST

enum when {
	DEVICE_NEW,
	FIRST_FRAME,
};

START_TEST(lua_disable_button_debounce)
{
	enum when when = litest_test_param_get_i32(test_env->params, "when");
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	_autofree_ char *lua = strdup_printf(
		"libinput:register({1})\n"
		"function frame_handler(device, _, _)\n"
		"  device:disable_feature(\"button-debouncing\")\n"
		"end\n"
		"function new_device(device)\n"
		"  %s device:disable_feature(\"button-debouncing\")\n"
		"  %s device:connect(\"evdev-frame\", frame_handler)\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", new_device)\n",
		when == DEVICE_NEW ? "" : "--",
		when == FIRST_FRAME ? "" : "--");
	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	etrace("%s", lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);

	if (libinput_log_get_priority(li) > LIBINPUT_LOG_PRIORITY_DEBUG)
		libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_DEBUG);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		_destroy_(litest_device) *dev = litest_add_device(li, LITEST_MOUSE);
		litest_drain_events(li);

		litest_disable_middleemu(dev);

		litest_event(dev, EV_KEY, BTN_LEFT, 1);
		litest_event(dev, EV_SYN, SYN_REPORT, 0);
		litest_event(dev, EV_KEY, BTN_LEFT, 0);
		litest_event(dev, EV_SYN, SYN_REPORT, 0);
		litest_event(dev, EV_KEY, BTN_LEFT, 1);
		litest_event(dev, EV_SYN, SYN_REPORT, 0);
		litest_event(dev, EV_KEY, BTN_LEFT, 0);
		litest_event(dev, EV_SYN, SYN_REPORT, 0);
		litest_timeout_debounce(li);

		litest_assert_button_event(li, BTN_LEFT, LIBINPUT_BUTTON_STATE_PRESSED);
		litest_assert_button_event(li,
					   BTN_LEFT,
					   LIBINPUT_BUTTON_STATE_RELEASED);
		litest_assert_button_event(li, BTN_LEFT, LIBINPUT_BUTTON_STATE_PRESSED);
		litest_assert_button_event(li,
					   BTN_LEFT,
					   LIBINPUT_BUTTON_STATE_RELEASED);
		litest_assert_empty_queue(li);

		litest_assert_strv_substring(capture->debugs,
					     "disabled button debouncing on request");
	}
}
END_TEST

START_TEST(lua_disable_touchpad_jump_detection)
{
	enum when when = litest_test_param_get_i32(test_env->params, "when");
	_destroy_(tmpdir) *tmpdir = tmpdir_create(NULL);
	_autofree_ char *lua = strdup_printf(
		"libinput:register({1})\n"
		"function frame_handler(dev, f, ts)\n"
		"  dev:disable_feature(\"touchpad-jump-detection\")\n"
		"end\n"
		"function new_device(device)\n"
		"  %sdevice:disable_feature(\"touchpad-jump-detection\")\n"
		"  %sdevice:connect(\"evdev-frame\", frame_handler)\n"
		"end\n"
		"libinput:connect(\"new-evdev-device\", new_device)\n",
		when == DEVICE_NEW ? "" : "-- ",
		when == FIRST_FRAME ? "" : "-- ");

	etrace("plugin data:\n%s", lua);

	_autofree_ char *path = litest_write_plugin(tmpdir->path, lua);
	_litest_context_destroy_ struct libinput *li =
		litest_create_context_with_plugindir(tmpdir->path);

	if (libinput_log_get_priority(li) > LIBINPUT_LOG_PRIORITY_DEBUG)
		libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_DEBUG);

	litest_with_logcapture(li, capture) {
		libinput_plugin_system_load_plugins(li,
						    LIBINPUT_PLUGIN_SYSTEM_FLAG_NONE);
		litest_drain_events(li);

		_destroy_(litest_device) *dev =
			litest_add_device(li, LITEST_SYNAPTICS_RMI4);
		litest_drain_events(li);

		litest_touch_down(dev, 0, 40, 50);
		litest_touch_move(dev, 0, 80, 80);
		litest_touch_move(dev, 0, 90, 90);
		litest_touch_up(dev, 0);

		litest_assert_only_typed_events(li, LIBINPUT_EVENT_POINTER_MOTION);
		litest_assert_empty_queue(li);

		litest_assert(!strv_find_substring(capture->infos,
						   "Touch jump detected and discarded",
						   NULL));
	}
}
END_TEST

TEST_COLLECTION(lua)
{
	/* clang-format off */
	litest_add_no_device(lua_load_failure);
	litest_with_parameters(params,
			       "content", 'I', 6,
					litest_named_i32(EMPTY),
					litest_named_i32(BASIC),
					litest_named_i32(COMMENT),
					litest_named_i32(DUPLICATE_CALL),
					litest_named_i32(MISSING_REGISTER),
					litest_named_i32(VERSION_NOT_A_TABLE)) {
		litest_add_parametrized_no_device(lua_load_success_but_no_register,
						  params);
	}
	litest_add_no_device(lua_register_noop);
	litest_with_parameters(params, "when", 's', 2, "run", "connect") {
		litest_add_parametrized_no_device(lua_unregister_is_last, params);
	}
	litest_add_no_device(lua_test_evdev_global);
	litest_add_no_device(lua_test_libinput_now);
	litest_with_parameters(params,
			       "mode", 's', 2, "absolute", "relative",
			       "reschedule", 'b') {
		litest_add_parametrized_no_device(lua_test_libinput_timer, params);
	}

	litest_with_parameters(params,
			       "priority", 'I', 3,
					litest_named_i32(LIBINPUT_LOG_PRIORITY_DEBUG),
					litest_named_i32(LIBINPUT_LOG_PRIORITY_INFO),
					litest_named_i32(LIBINPUT_LOG_PRIORITY_ERROR)) {
		litest_add_parametrized_no_device(lua_test_logging, params);
	}

	litest_with_parameters(params,
			       "handler", 's', 2, "new-evdev-device", "timer-expired",
			       "error", 'I', 3,
					litest_named_i32(BAD_TYPE),
					litest_named_i32(TOO_FEW_ARGS),
					litest_named_i32(TOO_MANY_ARGS)) {
		litest_add_parametrized_no_device(lua_bad_connect, params);
	}

	litest_add_no_device(lua_register_multiversions);
	litest_add_no_device(lua_allowed_functions);
	litest_add_no_device(lua_disallowed_functions);

	litest_add_no_device(lua_frame_handler);
	litest_add_no_device(lua_device_info);
	litest_add_no_device(lua_set_absinfo);
	litest_add_no_device(lua_enable_disable_evdev_usage);
	litest_add_no_device(lua_ignore_unsupported_codes);

	litest_with_parameters(params,
			       "which", 'I', 3,
					litest_named_i32(LITEST_MOUSE),
					litest_named_i32(LITEST_TRACKPOINT),
					litest_named_i32(LITEST_GENERIC_MULTITOUCH_SCREEN)) {
		litest_add_parametrized_no_device(lua_udev_properties, params);
	}

	litest_with_parameters(params, "append", 'b', "in_timer", 'b') {
		litest_add_parametrized_no_device(lua_append_prepend_frame, params);
	}

	litest_with_parameters(params, "when", 'I', 2,
					litest_named_i32(DEVICE_NEW),
					litest_named_i32(FIRST_FRAME)) {
		litest_add_parametrized_no_device(lua_disable_button_debounce, params);
		litest_add_parametrized_no_device(lua_disable_touchpad_jump_detection, params);
	}
	/* clang-format on */
}
