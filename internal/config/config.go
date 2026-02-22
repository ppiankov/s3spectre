package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultFileName   = ".s3spectre.yaml"
	alternateFileName = ".s3spectre.yml"
)

// Config holds persistent defaults loaded from a config file.
type Config struct {
	Region          string   `yaml:"region"`
	ExcludeBuckets  []string `yaml:"exclude_buckets"`
	ExcludePrefixes []string `yaml:"exclude_prefixes"`
	StaleDays       int      `yaml:"stale_days"`
	Format          string   `yaml:"format"`
	Timeout         string   `yaml:"timeout"`
}

// TimeoutDuration parses the Timeout field as a Go duration.
// Returns 0 if empty or unparseable.
func (c *Config) TimeoutDuration() time.Duration {
	if c.Timeout == "" {
		return 0
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 0
	}
	return d
}

// Load searches for a config file in the given directory and the user's home
// directory. Returns a zero-value Config if no file is found.
func Load(dir string) (Config, error) {
	paths := searchPaths(dir)
	for _, p := range paths {
		cfg, found, err := loadPath(p)
		if err != nil {
			return Config{}, err
		}
		if found {
			return cfg, nil
		}
	}
	return Config{}, nil
}

func searchPaths(dir string) []string {
	var paths []string
	if dir != "" {
		paths = append(paths, filepath.Join(dir, defaultFileName))
		paths = append(paths, filepath.Join(dir, alternateFileName))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, defaultFileName))
		paths = append(paths, filepath.Join(home, alternateFileName))
	}
	return paths
}

func loadPath(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}
