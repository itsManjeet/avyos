package app

import "testing"

func TestParseFrameTraceConfigDefaults(t *testing.T) {
	cfg := parseFrameTraceConfig(func(string) string { return "" })
	if cfg.enabled {
		t.Fatal("expected tracing disabled by default")
	}
	if cfg.every != 60 {
		t.Fatalf("expected default cadence 60, got %d", cfg.every)
	}
}

func TestParseFrameTraceConfigEnv(t *testing.T) {
	env := map[string]string{
		"AVYOS_GRAPHICS_TRACE_FRAMES": "true",
		"AVYOS_GRAPHICS_TRACE_EVERY":  "15",
	}
	cfg := parseFrameTraceConfig(func(key string) string { return env[key] })
	if !cfg.enabled {
		t.Fatal("expected tracing enabled")
	}
	if cfg.every != 15 {
		t.Fatalf("expected cadence 15, got %d", cfg.every)
	}
}

func TestParseFrameTraceConfigIgnoresInvalidCadence(t *testing.T) {
	env := map[string]string{
		"AVYOS_GRAPHICS_TRACE_FRAMES": "1",
		"AVYOS_GRAPHICS_TRACE_EVERY":  "nope",
	}
	cfg := parseFrameTraceConfig(func(key string) string { return env[key] })
	if !cfg.enabled {
		t.Fatal("expected tracing enabled")
	}
	if cfg.every != 60 {
		t.Fatalf("expected fallback cadence 60, got %d", cfg.every)
	}
}
