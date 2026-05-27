// Package config loads raidforge configuration. The CUE schema (schema.cue) is
// the single source of truth for structure and defaults; user-supplied YAML is
// unified against it — filling defaults and rejecting unknown fields — then
// decoded into a typed Config.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"gopkg.in/yaml.v3"
)

//go:embed schema.cue
var schemaCUE string

// Config is the resolved, typed application configuration.
type Config struct {
	Server    Server
	Logging   Logging
	Data      Data
	Optimizer Optimizer
}

// Server holds HTTP server settings.
type Server struct {
	Addr          string
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	IdleTimeout   time.Duration
	ShutdownGrace time.Duration
}

// Logging holds structured-logging settings.
type Logging struct {
	Level  string
	Format string
}

// Data selects the static dataset (see data/).
type Data struct {
	Dir  string
	Tier string
}

// Optimizer holds the objective weights and penalties (SPEC §4).
type Optimizer struct {
	Weights   Weights
	Penalties Penalties
}

// Weights are the objective-term coefficients.
type Weights struct {
	Throughput float64
	Buff       float64
	Debuff     float64
	Meta       float64
	Capability float64
	Empirical  float64
}

// Penalties are the soft-penalty coefficients.
type Penalties struct {
	RoleImbalance float64
	BenchTop      float64
}

// rawConfig mirrors the CUE schema field-for-field (durations as strings) for
// decoding; resolve converts it to the typed Config.
type rawConfig struct {
	Server struct {
		Addr          string `json:"addr"`
		ReadTimeout   string `json:"read_timeout"`
		WriteTimeout  string `json:"write_timeout"`
		IdleTimeout   string `json:"idle_timeout"`
		ShutdownGrace string `json:"shutdown_grace"`
	} `json:"server"`
	Logging struct {
		Level  string `json:"level"`
		Format string `json:"format"`
	} `json:"logging"`
	Data struct {
		Dir  string `json:"dir"`
		Tier string `json:"tier"`
	} `json:"data"`
	Optimizer struct {
		Weights struct {
			Throughput float64 `json:"throughput"`
			Buff       float64 `json:"buff"`
			Debuff     float64 `json:"debuff"`
			Meta       float64 `json:"meta"`
			Capability float64 `json:"capability"`
			Empirical  float64 `json:"empirical"`
		} `json:"weights"`
		Penalties struct {
			RoleImbalance float64 `json:"role_imbalance"`
			BenchTop      float64 `json:"bench_top"`
		} `json:"penalties"`
	} `json:"optimizer"`
}

// Load reads YAML config from path (with $ENV expansion), unifies it against the
// CUE schema (filling defaults, rejecting unknown fields), validates, and returns
// the resolved config. An empty path returns the built-in defaults.
func Load(path string) (*Config, error) {
	ctx := cuecontext.New()

	schema := ctx.CompileString(schemaCUE)
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("config: compile schema: %w", err)
	}
	def := schema.LookupPath(cue.ParsePath("#Config"))
	if err := def.Err(); err != nil {
		return nil, fmt.Errorf("config: lookup #Config: %w", err)
	}

	input := map[string]any{}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("config: read %s: %w", path, err)
		}
		if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(data))), &input); err != nil {
			return nil, fmt.Errorf("config: parse %s: %w", path, err)
		}
	}

	inputVal := ctx.Encode(input)
	if err := inputVal.Err(); err != nil {
		return nil, fmt.Errorf("config: encode input: %w", err)
	}

	unified := def.Unify(inputVal)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	var raw rawConfig
	if err := unified.Decode(&raw); err != nil {
		return nil, fmt.Errorf("config: decode: %w", err)
	}
	return raw.resolve()
}

func (r rawConfig) resolve() (*Config, error) {
	parseDur := func(name, s string) (time.Duration, error) {
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, fmt.Errorf("config: invalid duration %s=%q: %w", name, s, err)
		}
		return d, nil
	}

	var c Config
	var err error
	c.Server.Addr = r.Server.Addr
	if c.Server.ReadTimeout, err = parseDur("server.read_timeout", r.Server.ReadTimeout); err != nil {
		return nil, err
	}
	if c.Server.WriteTimeout, err = parseDur("server.write_timeout", r.Server.WriteTimeout); err != nil {
		return nil, err
	}
	if c.Server.IdleTimeout, err = parseDur("server.idle_timeout", r.Server.IdleTimeout); err != nil {
		return nil, err
	}
	if c.Server.ShutdownGrace, err = parseDur("server.shutdown_grace", r.Server.ShutdownGrace); err != nil {
		return nil, err
	}

	c.Logging = Logging{Level: r.Logging.Level, Format: r.Logging.Format}
	c.Data = Data{Dir: r.Data.Dir, Tier: r.Data.Tier}
	c.Optimizer.Weights = Weights{
		Throughput: r.Optimizer.Weights.Throughput,
		Buff:       r.Optimizer.Weights.Buff,
		Debuff:     r.Optimizer.Weights.Debuff,
		Meta:       r.Optimizer.Weights.Meta,
		Capability: r.Optimizer.Weights.Capability,
		Empirical:  r.Optimizer.Weights.Empirical,
	}
	c.Optimizer.Penalties = Penalties{
		RoleImbalance: r.Optimizer.Penalties.RoleImbalance,
		BenchTop:      r.Optimizer.Penalties.BenchTop,
	}
	return &c, nil
}
