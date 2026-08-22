local config = {}

local function get_self_path()
    local str = debug.getinfo(2, "S").source:sub(2)
    return str:match("(.*/)")
end

local THIS_DIR  = get_self_path()

config.Terminal = os.getenv("TERMINAL") or "/apps/alacritty"
config.Editor   = os.getenv("EDITOR") or "vim"
config.ModKey   = "Mod4"
config.Startups = {
    "picom --config " .. THIS_DIR .. "/picom.conf",
}


return config
