// Copyright (c) 2026 Visvasity LLC

package report

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Config selects which collection modules run. The zero value collects every
// module at its catalog default, so Config{} is the default policy.
//
// Two independent controls are provided:
//
//   - Modules overrides individual modules on or off by key.
//   - MaxSensitivity caps collection by privacy tier for modules that are not
//     named in Modules.
//
// An explicit entry in Modules always wins over MaxSensitivity, so a
// privacy-conscious operator can cap the tier and still opt a specific module
// back in (or out).
type Config struct {
	// Modules maps a module key to an explicit on/off decision. A key absent
	// from the map uses the module's catalog default. Unknown keys are ignored.
	Modules map[string]bool

	// MaxSensitivity, when set to a valid tier, disables every module above that
	// tier unless the module is named explicitly in Modules. Use CapSensitivity
	// to set it; the zero value (SensitivityLow) does not cap on its own — see
	// hasCap.
	MaxSensitivity Sensitivity

	// hasCap records whether MaxSensitivity is active, so that the zero value of
	// Config imposes no cap.
	hasCap bool
}

// CapSensitivity returns a copy of c that drops every module above tier (unless
// individually named in Modules).
func (c Config) CapSensitivity(tier Sensitivity) Config {
	c.MaxSensitivity = tier
	c.hasCap = true
	return c
}

// With returns a copy of c with the given module keys forced to enabled.
func (c Config) With(keys ...string) Config { return c.set(true, keys...) }

// Without returns a copy of c with the given module keys forced to disabled.
func (c Config) Without(keys ...string) Config { return c.set(false, keys...) }

func (c Config) set(on bool, keys ...string) Config {
	m := make(map[string]bool, len(c.Modules)+len(keys))
	maps.Copy(m, c.Modules)
	for _, k := range keys {
		m[k] = on
	}
	c.Modules = m
	return c
}

// Enabled reports whether the module with the given key will be collected under
// this config. Unknown keys are never enabled.
func (c Config) Enabled(key string) bool {
	d, ok := Lookup(key)
	if !ok {
		return false
	}
	if v, set := c.Modules[key]; set {
		return v // explicit override wins over the sensitivity cap
	}
	if !d.DefaultEnabled {
		return false
	}
	if c.hasCap && d.Sensitivity > c.MaxSensitivity {
		return false
	}
	return true
}

// EnabledKeys returns the keys of all enabled modules, in catalog order.
func (c Config) EnabledKeys() []string {
	return keysWhere(func(d Descriptor) bool { return c.Enabled(d.Key) })
}

// DisabledIdentityCritical returns the identity-critical modules that are
// disabled under this config. The agent should warn when this is non-empty,
// because such reports may be unattributable.
func (c Config) DisabledIdentityCritical() []string {
	return keysWhere(func(d Descriptor) bool { return d.IdentityCritical && !c.Enabled(d.Key) })
}

// ValidateForGateway returns an error if any gateway-essential module is
// disabled. Callers uploading to the HostCheck Gateway should reject such a
// config, and the gateway may reject the resulting upload.
func (c Config) ValidateForGateway() error {
	var missing []string
	for _, key := range GatewayEssentialKeys() {
		if !c.Enabled(key) {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf("gateway-essential modules disabled: %s", strings.Join(missing, ", "))
	}
	return nil
}
