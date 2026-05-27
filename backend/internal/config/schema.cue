// raidforge configuration schema — the single source of truth for config
// structure and defaults (see docs/SPEC.md §3.7, §4). Runtime YAML is unified
// against #Config, which fills defaults and rejects unknown fields.
package config

#Config: {
	// HTTP server.
	server: {
		addr:           string | *":8080"
		read_timeout:   string | *"30s"
		write_timeout:  string | *"30s"
		idle_timeout:   string | *"120s"
		shutdown_grace: string | *"10s"
	}

	// Structured logging.
	logging: {
		level:  "debug" | "info" | "warn" | "error" | *"info"
		format: "json" | "text" | *"json"
	}

	// Static dataset selection (see data/).
	data: {
		dir:  string | *"data"
		tier: string | *"midnight-s1"
	}

	// Optimizer objective weights and penalties (SPEC §4), tunable without a rebuild.
	optimizer: {
		weights: {
			throughput: number | *1.0
			buff:       number | *5.0
			debuff:     number | *5.0
			meta:       number | *2.0
			capability: number | *3.0
			// Comps proven in WCL kill logs can outrank the theoretical ideal (SPEC §3.6).
			empirical: number | *8.0
		}
		penalties: {
			role_imbalance: number | *10.0
			bench_top:      number | *1.0
		}
	}
}

// Concrete default instance (handy for emitting a documented default config).
config: #Config
