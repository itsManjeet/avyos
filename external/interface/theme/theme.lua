local function get_self_path()
    local str = debug.getinfo(2, "S").source:sub(2)
    return str:match("(.*/)")
end

local THIS_DIR     = get_self_path()

local theme_assets = require("beautiful.theme_assets")
local xresources   = require("beautiful.xresources")
local dpi          = xresources.apply_dpi

local gfs          = require("gears.filesystem")
local themes_path  = THIS_DIR

local theme        = {}

theme.font         = "sans 11"



theme.bg_normal = "#222222"
theme.bg_focus  = "#1E2320"
theme.bg_urgent = "#3F3F3F"



theme.fg_normal = "#FEFEFE"
theme.fg_focus  = "#32D6FF"
theme.fg_urgent = "#C83F11"



theme.useless_gap   = dpi(2)
theme.border_width  = dpi(1)
theme.border_normal = "#3F3F3F"
theme.border_focus  = "#6F6F6F"
theme.border_marked = "#CC9393"



local taglist_square_size   = dpi(4)
theme.taglist_squares_sel   = theme_assets.taglist_squares_sel(taglist_square_size, theme.fg_normal)
theme.taglist_squares_unsel = theme_assets.taglist_squares_unsel(taglist_square_size, theme.fg_normal)



theme.menu_submenu_icon = themes_path .. "submenu.png"
theme.menu_height       = dpi(27)
theme.menu_width        = dpi(140)
theme.menu_border_color = "#3F3F3F"
theme.menu_border_width = dpi(2)



theme.titlebar_close_button_normal              = themes_path .. "titlebar/close-active.png"
theme.titlebar_close_button_focus               = themes_path .. "titlebar/close-inactive.png"

theme.titlebar_minimize_button_normal           = themes_path .. "titlebar/minimize_normal.png"
theme.titlebar_minimize_button_focus            = themes_path .. "titlebar/minimize_focus.png"

theme.titlebar_ontop_button_normal_inactive     = themes_path .. "titlebar/ontop_normal_inactive.png"
theme.titlebar_ontop_button_focus_inactive      = themes_path .. "titlebar/ontop_focus_inactive.png"
theme.titlebar_ontop_button_normal_active       = themes_path .. "titlebar/ontop_normal_active.png"
theme.titlebar_ontop_button_focus_active        = themes_path .. "titlebar/ontop_focus_active.png"

theme.titlebar_sticky_button_normal_inactive    = themes_path .. "titlebar/sticky_normal_inactive.png"
theme.titlebar_sticky_button_focus_inactive     = themes_path .. "titlebar/sticky_focus_inactive.png"
theme.titlebar_sticky_button_normal_active      = themes_path .. "titlebar/sticky_normal_active.png"
theme.titlebar_sticky_button_focus_active       = themes_path .. "titlebar/sticky_focus_active.png"

theme.titlebar_floating_button_normal_inactive  = themes_path .. "titlebar/floating_normal_inactive.png"
theme.titlebar_floating_button_focus_inactive   = themes_path .. "titlebar/floating_focus_inactive.png"
theme.titlebar_floating_button_normal_active    = themes_path .. "titlebar/floating_normal_active.png"
theme.titlebar_floating_button_focus_active     = themes_path .. "titlebar/floating_focus_active.png"

theme.titlebar_maximized_button_normal_inactive = themes_path .. "titlebar/maximized_normal_inactive.png"
theme.titlebar_maximized_button_focus_inactive  = themes_path .. "titlebar/maximized_focus_inactive.png"
theme.titlebar_maximized_button_normal_active   = themes_path .. "titlebar/maximized_normal_active.png"
theme.titlebar_maximized_button_focus_active    = themes_path .. "titlebar/maximized_focus_active.png"

theme.wallpaper                                 = themes_path .. "background.jpeg"


-- You can use your own layout icons like this:
theme.layout_fairh      = themes_path .. "layouts/fairhw.png"
theme.layout_fairv      = themes_path .. "layouts/fairvw.png"
theme.layout_floating   = themes_path .. "layouts/floatingw.png"
theme.layout_magnifier  = themes_path .. "layouts/magnifierw.png"
theme.layout_max        = themes_path .. "layouts/maxw.png"
theme.layout_fullscreen = themes_path .. "layouts/fullscreenw.png"
theme.layout_tilebottom = themes_path .. "layouts/tilebottomw.png"
theme.layout_tileleft   = themes_path .. "layouts/tileleftw.png"
theme.layout_tile       = themes_path .. "layouts/tilew.png"
theme.layout_tiletop    = themes_path .. "layouts/tiletopw.png"
theme.layout_spiral     = themes_path .. "layouts/spiralw.png"
theme.layout_dwindle    = themes_path .. "layouts/dwindlew.png"
theme.layout_cornernw   = themes_path .. "layouts/cornernww.png"
theme.layout_cornerne   = themes_path .. "layouts/cornernew.png"
theme.layout_cornersw   = themes_path .. "layouts/cornersww.png"
theme.layout_cornerse   = themes_path .. "layouts/cornersew.png"


theme.awesome_icon = theme_assets.awesome_icon(theme.menu_height, theme.bg_focus, theme.fg_focus)

theme.icon_theme   = nil

return theme
