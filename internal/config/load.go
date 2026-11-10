package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/afplana/delegate/internal/runner"

	"go.yaml.in/yaml/v3"
)

// Config is the set of profiles delegate can run and the one it uses when
// --profile is not given.
type Config struct {
	Default  string
	Profiles map[string]runner.Profile
}

// Defaults returns the built-in profiles with DefaultProfile as the default.
func Defaults() Config {
	return Config{Default: DefaultProfile, Profiles: Builtin()}
}

// outputs maps a profile's `output` key to the parser for that CLI's stdout.
var outputs = map[string]runner.ParseFunc{
	"text":        nil,
	"claude-json": runner.ParseClaudeJSON,
	"cursor-json": runner.ParseCursorJSON,
}

// fileConfig is the YAML shape. Pointers tell "not set" from "set to zero",
// so a file can override one field of a profile and keep the rest.
type fileConfig struct {
	DefaultProfile *string                `yaml:"default_profile"`
	Profiles       map[string]fileProfile `yaml:"profiles"`
}

type fileProfile struct {
	Command *string   `yaml:"command"`
	Args    *[]string `yaml:"args"`
	Timeout *string   `yaml:"timeout"`
	Output  *string   `yaml:"output"`
}

// WithUserFile merges the user's own config file over c. The user's file is
// trusted: it can add profiles and change what they run. A missing file
// returns c unchanged.
func (c Config) WithUserFile(path string) (Config, error) {
	return c.merge(path, true)
}

// WithRepoFile merges a repository's .delegate.yaml over c. A repo is not
// trusted to choose what runs, so it may only set default_profile and the
// timeout of existing profiles. A missing file returns c unchanged.
func (c Config) WithRepoFile(path string) (Config, error) {
	return c.merge(path, false)
}

func (c Config) merge(path string, trusted bool) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var file fileConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}

	out := Config{Default: c.Default, Profiles: maps.Clone(c.Profiles)}
	for _, name := range slices.Sorted(maps.Keys(file.Profiles)) {
		profile, err := mergeProfile(out.Profiles, name, file.Profiles[name], trusted)
		if err != nil {
			return Config{}, fmt.Errorf("%s: profiles.%s%w", path, name, err)
		}
		out.Profiles[name] = profile
	}
	if file.DefaultProfile != nil {
		out.Default = *file.DefaultProfile
	}
	if _, ok := out.Profiles[out.Default]; !ok {
		return Config{}, fmt.Errorf("%s: default_profile: unknown profile %q", path, out.Default)
	}
	return out, nil
}

// mergeProfile applies the fields set in fp to the existing profile called
// name, or builds a new one. Errors start with ".<key>: " or ": " so the
// caller can prefix "profiles.<name>".
func mergeProfile(profiles map[string]runner.Profile, name string, fp fileProfile, trusted bool) (runner.Profile, error) {
	profile, exists := profiles[name]
	if !trusted {
		if !exists {
			return runner.Profile{}, errors.New(": a repo file cannot add profiles")
		}
		for key, set := range map[string]bool{"command": fp.Command != nil, "args": fp.Args != nil, "output": fp.Output != nil} {
			if set {
				return runner.Profile{}, fmt.Errorf(".%s: not allowed in a repo file (only default_profile and timeout)", key)
			}
		}
	}
	if !exists {
		if fp.Command == nil {
			return runner.Profile{}, errors.New(".command: required for a new profile")
		}
		profile = runner.Profile{Name: name, Timeout: defaultTimeout}
	}

	if fp.Command != nil {
		profile.Command = *fp.Command
	}
	if fp.Args != nil {
		profile.Args = *fp.Args
	}
	if !slices.ContainsFunc(profile.Args, func(a string) bool { return strings.Contains(a, "{prompt}") }) {
		return runner.Profile{}, errors.New(".args: must contain {prompt}, or the task never reaches the worker")
	}
	if fp.Timeout != nil {
		timeout, err := time.ParseDuration(*fp.Timeout)
		if err != nil {
			return runner.Profile{}, fmt.Errorf(".timeout: %w", err)
		}
		profile.Timeout = timeout
	}
	if fp.Output != nil {
		parse, ok := outputs[*fp.Output]
		if !ok {
			return runner.Profile{}, fmt.Errorf(".output: unknown %q (want text, claude-json, cursor-json)", *fp.Output)
		}
		profile.Parse = parse
	}
	return profile, nil
}
