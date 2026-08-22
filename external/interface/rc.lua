pcall(require, "luarocks.loader")

local beautiful = require("beautiful")
local menubar   = require("menubar")
local config    = require("config")

local function get_self_path()
    local str = debug.getinfo(2, "S").source:sub(2)
    return str:match("(.*/)")
end

local THIS_DIR = get_self_path()


beautiful.init(THIS_DIR .. "theme/theme.lua")

require("awful.hotkeys_popup.keys")
require("awful.autofocus")

RC = {}
RC.config = config

require("internal.error-handling")


local internal = {
    layout       = require("internal.layouts"),
    global_menu  = require("internal.global-menu"),
    startup_apps = require("internal.startup-apps"),
}

RC.layouts = internal.layout()
RC.global_menu = internal.global_menu(config)

menubar.utils.terminal = config.Terminal

require("internal.wallpaper")
require("internal.global-buttons")
require("internal.global-keys")

require("panel")
require("client")

internal.startup_apps(config.Startups)
