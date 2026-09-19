// Copyright (c) 2026 Visvasity LLC

package report

// This file is the single source of truth for the set of collection modules:
// their stable keys, default enablement, privacy sensitivity, and the roles
// (identity-critical, gateway-essential) that some modules play. The collection
// agent uses it to decide what to run; the gateway uses it to validate uploads.

// Sensitivity is the privacy sensitivity tier of a module's collected data.
type Sensitivity int

const (
	SensitivityLow Sensitivity = iota
	SensitivityModerate
	SensitivityHigh
)

func (s Sensitivity) String() string {
	switch s {
	case SensitivityLow:
		return "low"
	case SensitivityModerate:
		return "moderate"
	case SensitivityHigh:
		return "high"
	default:
		return "unknown"
	}
}

// Module keys. These are stable identifiers used in configuration, in the
// Report.EnabledModules list, and by the gateway; they must not change.
const (
	KeyAgent        = "agent"
	KeyHostIdentity = "host-identity"
	KeyPublicIP     = "public-ip"

	KeyFirewallPolicy        = "firewall-policy"
	KeyFirewallAcceptedPorts = "firewall-accepted-ports"
	KeyFirewallRuleset       = "firewall-ruleset"
	KeyListeningTCP          = "listening-tcp"
	KeyListeningUDP          = "listening-udp"
	KeyNATForwarding         = "nat-forwarding"
	KeyInterfaces            = "interfaces"
	KeyRoutes                = "routes"

	KeySSHDConfig        = "sshd-config"
	KeySSHAuthorizedKeys = "ssh-authorized-keys"
	KeySSHHostKeys       = "ssh-hostkeys"

	KeyAccounts         = "accounts"
	KeySudoers          = "sudoers"
	KeyPrivilegedGroups = "privileged-groups"
	KeyPasswordPolicy   = "password-policy"

	KeyEnabledUnits       = "enabled-units"
	KeyTimersCron         = "timers-cron"
	KeyStartupPersistence = "startup-persistence"

	KeySysctlHardening = "sysctl-hardening"
	KeyOSProtections   = "os-protections"
	KeySecureBoot      = "secure-boot"

	KeyOSUpdates        = "os-updates"
	KeyPackageInventory = "package-inventory"
	KeyPendingRestart   = "pending-restart"

	KeyContainers = "containers"

	KeyCriticalFileIntegrity = "critical-file-integrity"

	KeySSHLogins         = "ssh-logins"
	KeySSHAuthFailures   = "ssh-auth-failures"
	KeySudoActivity      = "sudo-activity"
	KeySudoAuthFailures  = "sudo-auth-failures"
	KeyIntrusionResponse = "intrusion-response"

	KeyServiceVersions   = "service-versions"
	KeyLanguageLibraries = "language-libraries"
)

// Descriptor is the catalog metadata for one collection module.
type Descriptor struct {
	// Key is the module's stable identifier.
	Key string
	// DefaultEnabled is whether the module is collected out of the box.
	DefaultEnabled bool
	// Sensitivity is the privacy tier of the collected data.
	Sensitivity Sensitivity
	// IdentityCritical marks modules without which a report may be
	// unattributable or non-actionable. Disabling one warrants a warning.
	IdentityCritical bool
	// GatewayEssential marks modules the HostCheck Gateway requires to perform
	// its core functions (attributing a host, rendering the exposure dashboard,
	// and alerting on open-port changes). The gateway may reject an upload that
	// omits any of these.
	GatewayEssential bool
}

// Catalog lists every collection module in Report field order. It is the
// authoritative registry of modules.
var Catalog = []Descriptor{
	// Agent & host identity (identity-critical). The three identity modules plus
	// the two listening-socket modules are the gateway-essential set: identity so
	// the gateway can attribute a host and render its dashboard, and the
	// listeners because open-port change detection is the product's core promise.
	{KeyAgent, true, SensitivityLow, true, true},
	{KeyHostIdentity, true, SensitivityModerate, true, true},
	{KeyPublicIP, true, SensitivityHigh, true, true},

	// Network exposure.
	{KeyFirewallPolicy, true, SensitivityLow, false, false},
	{KeyFirewallAcceptedPorts, true, SensitivityModerate, false, false},
	{KeyFirewallRuleset, false, SensitivityHigh, false, false},
	{KeyListeningTCP, true, SensitivityModerate, false, true},
	{KeyListeningUDP, true, SensitivityModerate, false, true},
	{KeyNATForwarding, true, SensitivityModerate, false, false},
	{KeyInterfaces, false, SensitivityHigh, false, false},
	{KeyRoutes, false, SensitivityHigh, false, false},

	// Remote access (SSH).
	{KeySSHDConfig, true, SensitivityLow, false, false},
	{KeySSHAuthorizedKeys, true, SensitivityModerate, false, false},
	{KeySSHHostKeys, true, SensitivityLow, false, false},

	// Accounts & privilege.
	{KeyAccounts, true, SensitivityHigh, false, false},
	{KeySudoers, true, SensitivityHigh, false, false},
	{KeyPrivilegedGroups, true, SensitivityModerate, false, false},
	{KeyPasswordPolicy, false, SensitivityHigh, false, false},

	// Services & scheduled execution.
	{KeyEnabledUnits, true, SensitivityModerate, false, false},
	{KeyTimersCron, true, SensitivityModerate, false, false},
	{KeyStartupPersistence, true, SensitivityModerate, false, false},

	// Kernel & platform hardening.
	{KeySysctlHardening, true, SensitivityLow, false, false},
	{KeyOSProtections, true, SensitivityLow, false, false},
	{KeySecureBoot, false, SensitivityLow, false, false},

	// Software & patch posture.
	{KeyOSUpdates, true, SensitivityModerate, false, false},
	{KeyPackageInventory, false, SensitivityHigh, false, false},
	{KeyPendingRestart, true, SensitivityLow, false, false},

	// Containers & virtualization.
	{KeyContainers, true, SensitivityModerate, false, false},

	// File integrity.
	{KeyCriticalFileIntegrity, true, SensitivityLow, false, false},

	// Authentication & access activity.
	{KeySSHLogins, true, SensitivityHigh, false, false},
	{KeySSHAuthFailures, true, SensitivityModerate, false, false},
	{KeySudoActivity, true, SensitivityHigh, false, false},
	{KeySudoAuthFailures, true, SensitivityHigh, false, false},
	{KeyIntrusionResponse, true, SensitivityModerate, false, false},

	// Software versions for vulnerability matching.
	{KeyServiceVersions, true, SensitivityModerate, false, false},
	{KeyLanguageLibraries, false, SensitivityHigh, false, false},
}

// catalogByKey indexes Catalog for O(1) lookup.
var catalogByKey = func() map[string]Descriptor {
	m := make(map[string]Descriptor, len(Catalog))
	for _, d := range Catalog {
		m[d.Key] = d
	}
	return m
}()

// Lookup returns the descriptor for a module key.
func Lookup(key string) (Descriptor, bool) {
	d, ok := catalogByKey[key]
	return d, ok
}

// Keys returns every module key in catalog order.
func Keys() []string {
	keys := make([]string, len(Catalog))
	for i, d := range Catalog {
		keys[i] = d.Key
	}
	return keys
}

// keysWhere returns the keys, in catalog order, whose descriptor satisfies pred.
func keysWhere(pred func(Descriptor) bool) []string {
	var keys []string
	for _, d := range Catalog {
		if pred(d) {
			keys = append(keys, d.Key)
		}
	}
	return keys
}

// GatewayEssentialKeys returns the modules the gateway requires to be present.
func GatewayEssentialKeys() []string {
	return keysWhere(func(d Descriptor) bool { return d.GatewayEssential })
}

// IdentityCriticalKeys returns the identity-critical modules.
func IdentityCriticalKeys() []string {
	return keysWhere(func(d Descriptor) bool { return d.IdentityCritical })
}
