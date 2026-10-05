package config

import (
	"encoding/json"
	"fmt"
	reflect "reflect"
	"strconv"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

type HiddifyOptions struct {
	EnableFullConfig        bool   `json:"enable-full-config,omitempty" overridable:"true"`
	LogLevel                string `json:"log-level,omitempty"`
	LogFile                 string `json:"log-file,omitempty"`
	EnableClashApi          bool   `json:"enable-clash-api,omitempty"`
	ClashApiPort            uint16 `json:"clash-api-port,omitempty"`
	ClashApiSecret          string `json:"web-secret,omitempty"`
	Region                  string `json:"region,omitempty"`
	BlockAds                bool   `json:"block-ads,omitempty" overridable:"true"`
	UseXrayCoreWhenPossible bool   `json:"use-xray-core-when-possible,omitempty" overridable:"true"`
	BalancerStrategy        string `json:"balancer-strategy,omitempty" overridable:"true"`
	// GeoIPPath        string      `json:"geoip-path"`
	// GeoSitePath      string      `json:"geosite-path"`
	// Rules is the legacy rule list. It is kept for JSON compatibility only: the
	// type it referenced no longer exists, so it is never populated.
	Rules []Rule `json:"rules,omitempty" overridable:"true"`

	// RouteRules carries the routing rules created in the client. This field has
	// no json tag on purpose: UnmarshalJSON (see below) accepts every spelling the
	// clients have used ("route-rule", "route_rule", "routeRule", ...) so that a
	// key mismatch can never again silently drop every rule.
	RouteRules RouteRules `json:"-"`
	// RuleSets declares the rule sets referenced by RouteRules.
	RuleSets []RuleSetEntry `json:"-"`
	// RuleSetBaseURL overrides where the default region rule sets are downloaded
	// from. The layout is "<base>/geosite-<region>.srs" and
	// "<base>/geoip-<region>.srs". It defaults to DefaultRegionRuleSetBaseURL,
	// which serves complete rule sets; the historical hiddify-geo mirror is kept
	// as an explicit fallback (LegacyRegionRuleSetBaseURL).
	RuleSetBaseURL string `json:"rule_set_base_url,omitempty"`

	Warp      WarpOptions `json:"warp,omitempty"`
	Warp2     WarpOptions `json:"warp2,omitempty"`
	Mux       MuxOptions  `json:"mux,omitempty" overridable:"true"`
	TLSTricks TLSTricks   `json:"tls-tricks,omitempty"`
	EnableNTP bool        `json:"enable-ntp,omitempty"`

	DNSOptions
	InboundOptions
	URLTestOptions
	RouteOptions
	ChainOptions
}

// routeRuleKeys / ruleSetKeys are the top level spellings the clients have used.
// All of them are accepted so that a rename on the client side degrades to
// "rules ignored" instead of silently losing the whole feature.
var routeRuleKeys = []string{"route-rule", "route_rule", "routeRule", "routeRules", "route_rules"}

var ruleSetKeys = []string{"rule-set", "rule_set", "ruleSet", "ruleSets", "rule_sets"}

// UnmarshalJSON is a compatibility layer in front of the generated decoder. It
//   - folds the camelCase / snake_case / kebab-case spellings of every option key
//     onto the canonical kebab-case tag, because the client serializes this
//     struct with FieldRename.kebab while the tags are a mix of both;
//   - collects the routing rules and rule set declarations, which are accepted
//     under several top level key names.
func (h *HiddifyOptions) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	normalized := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		canonical := canonicalHiddifyOptionKey(key)
		if _, taken := normalized[canonical]; !taken {
			normalized[canonical] = value
		}
	}
	for _, key := range routeRuleKeys {
		if value, ok := normalized[key]; ok {
			var rules RouteRules
			if err := json.Unmarshal(value, &rules); err != nil {
				return fmt.Errorf("invalid route rules: %w", err)
			}
			h.RouteRules = rules
			delete(normalized, key)
			break
		}
	}
	for _, key := range ruleSetKeys {
		if value, ok := normalized[key]; ok {
			var entries []RuleSetEntry
			if err := json.Unmarshal(value, &entries); err != nil {
				return fmt.Errorf("invalid rule sets: %w", err)
			}
			h.RuleSets = entries
			delete(normalized, key)
			break
		}
	}
	reencoded, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	type plain HiddifyOptions
	return json.Unmarshal(reencoded, (*plain)(h))
}

// hiddifyOptionKeyAliases maps the snake_case spelling produced by
// toSnakeCase onto the kebab-case key the struct tag uses.
var hiddifyOptionKeyAliases = map[string]string{
	"enable_full_config":            "enable-full-config",
	"log_level":                     "log-level",
	"log_file":                      "log-file",
	"enable_clash_api":              "enable-clash-api",
	"clash_api_port":                "clash-api-port",
	"web_secret":                    "web-secret",
	"block_ads":                     "block-ads",
	"use_xray_core_when_possible":   "use-xray-core-when-possible",
	"balancer_strategy":             "balancer-strategy",
	"tls_tricks":                    "tls-tricks",
	"enable_ntp":                    "enable-ntp",
	"remote_dns_address":            "remote-dns-address",
	"remote_dns_domain_strategy":    "remote-dns-domain-strategy",
	"direct_dns_address":            "direct-dns-address",
	"direct_dns_domain_strategy":    "direct-dns-domain-strategy",
	"independent_dns_cache":         "independent-dns-cache",
	"enable_fake_dns":               "enable-fake-dns",
	"enable_tun":                    "enable-tun",
	"enable_tun_service":            "enable-tun-service",
	"set_system_proxy":              "set-system-proxy",
	"mixed_port":                    "mixed-port",
	"tproxy_port":                   "tproxy-port",
	"redirect_port":                 "redirect-port",
	"direct_port":                   "direct-port",
	"strict_route":                  "strict-route",
	"tun_implementation":            "tun-implementation",
	"connection_test_url":           "connection-test-url",
	"connection_test_urls":          "connection-test-urls",
	"url_test_interval":             "url-test-interval",
	"resolve_destination":           "resolve-destination",
	"ipv6_mode":                     "ipv6-mode",
	"bypass_lan":                    "bypass-lan",
	"allow_connection_from_lan":     "allow-connection-from-lan",
	"block_quic":                    "block-quic",
	"enable_fragment":               "enable-fragment",
	"fragment_size":                 "fragment-size",
	"fragment_sleep":                "fragment-sleep",
	"mixed_sni_case":                "mixed-sni-case",
	"enable_padding":                "enable-padding",
	"padding_size":                  "padding-size",
	"max_streams":                   "max-streams",
	"clean_ip":                      "clean-ip",
	"clean_port":                    "clean-port",
	"wireguard_config":              "wireguard-config",
	"extra_security":                "extra-security",
	"unblocker":                     "unblocker",
	"chain_status":                  "chain-status",
	"rule_set_base_url":             "rule_set_base_url",
}

func canonicalHiddifyOptionKey(key string) string {
	// The struct tags are kebab-case, so a kebab-case key is already canonical
	// and must be passed through untouched (toSnakeCase would destroy it).
	if strings.ContainsRune(key, '-') {
		return key
	}
	snake := toSnakeCase(key)
	if alias, ok := hiddifyOptionKeyAliases[snake]; ok {
		return alias
	}
	return snake
}

type DNSOptions struct {
	RemoteDnsAddress        string                `json:"remote-dns-address,omitempty" overridable:"true"`
	RemoteDnsDomainStrategy option.DomainStrategy `json:"remote-dns-domain-strategy,omitempty" overridable:"true"`
	DirectDnsAddress        string                `json:"direct-dns-address,omitempty" overridable:"true"`
	DirectDnsDomainStrategy option.DomainStrategy `json:"direct-dns-domain-strategy,omitempty" overridable:"true"`
	IndependentDNSCache     bool                  `json:"independent-dns-cache,omitempty"`
	EnableFakeDNS           bool                  `json:"enable-fake-dns,omitempty"`
	// EnableDNSRouting        bool                  `json:"enable-dns-routing,omitempty"`
}

type InboundOptions struct {
	EnableTun        bool   `json:"enable-tun,omitempty"`
	EnableTunService bool   `json:"enable-tun-service,omitempty"`
	SetSystemProxy   bool   `json:"set-system-proxy,omitempty"`
	MixedPort        uint16 `json:"mixed-port,omitempty"`
	TProxyPort       uint16 `json:"tproxy-port,omitempty"`
	RedirectPort     uint16 `json:"redirect-port,omitempty"`
	DirectPort       uint16 `json:"direct-port,omitempty"`
	MTU              uint32 `json:"mtu,omitempty"`
	StrictRoute      bool   `json:"strict-route,omitempty"`
	TUNStack         string `json:"tun-implementation,omitempty"`
}

type URLTestOptions struct {
	ConnectionTestUrl  string            `json:"connection-test-url,omitempty" overridable:"true"`
	ConnectionTestUrls []string          `json:"connection-test-urls,omitempty" overridable:"true"`
	URLTestInterval    DurationInSeconds `json:"url-test-interval,omitempty" overridable:"true"`
	// URLTestIdleTimeout DurationInSeconds `json:"url-test-idle-timeout"`
}

type RouteOptions struct {
	ResolveDestination     bool                  `json:"resolve-destination,omitempty"`
	IPv6Mode               option.DomainStrategy `json:"ipv6-mode,omitempty"`
	BypassLAN              bool                  `json:"bypass-lan,omitempty"`
	AllowConnectionFromLAN bool                  `json:"allow-connection-from-lan,omitempty"`
	BlockQuic              bool                  `json:"block-quic,omitempty"`
}

type TLSTricks struct {
	EnableFragment bool   `json:"enable-fragment,omitempty" overridable:"true"`
	FragmentSize   string `json:"fragment-size,omitempty" overridable:"true"`
	FragmentSleep  string `json:"fragment-sleep,omitempty" overridable:"true"`
	MixedSNICase   bool   `json:"mixed-sni-case,omitempty" overridable:"true"`
	EnablePadding  bool   `json:"enable-padding,omitempty" overridable:"true"`
	PaddingSize    string `json:"padding-size,omitempty" overridable:"true"`
}

type MuxOptions struct {
	Enable     bool   `json:"enable,omitempty" overridable:"true"`
	Padding    bool   `json:"padding,omitempty" overridable:"true"`
	MaxStreams int    `json:"max-streams,omitempty" overridable:"true"`
	Protocol   string `json:"protocol,omitempty" overridable:"true"`
}

type WarpOptions struct {
	Id                 string              `json:"id,omitempty"`
	EnableWarp         bool                `json:"enable,omitempty"`
	Mode               string              `json:"mode,omitempty"`
	WireguardConfigStr string              `json:"wireguard-config,omitempty"`
	WireguardConfig    WarpWireguardConfig `json:"wireguardConfig,omitempty"` // TODO check
	FakePackets        string              `json:"noise,omitempty"`
	FakePacketSize     string              `json:"noise-size,omitempty"`
	FakePacketDelay    string              `json:"noise-delay,omitempty"`
	FakePacketMode     string              `json:"noise-mode,omitempty"`
	CleanIP            string              `json:"clean-ip,omitempty"`
	CleanPort          uint16              `json:"clean-port,omitempty"`
	Account            WarpAccount
}

func DefaultHiddifyOptions() *HiddifyOptions {
	return &HiddifyOptions{
		EnableNTP: true,
		DNSOptions: DNSOptions{
			RemoteDnsAddress:        "1.1.1.1",
			RemoteDnsDomainStrategy: option.DomainStrategy(C.DomainStrategyAsIS),
			DirectDnsAddress:        "1.1.1.1",
			DirectDnsDomainStrategy: option.DomainStrategy(C.DomainStrategyAsIS),
			IndependentDNSCache:     false,
			EnableFakeDNS:           false,
			// EnableDNSRouting:        false,
		},
		InboundOptions: InboundOptions{
			EnableTun:      false,
			SetSystemProxy: false,
			MixedPort:      12334,
			TProxyPort:     12335,
			RedirectPort:   12336,
			DirectPort:     12337,
			MTU:            9000,
			StrictRoute:    true,
			TUNStack:       "mixed",
		},
		URLTestOptions: URLTestOptions{
			ConnectionTestUrl: "http://cp.cloudflare.com/",
			URLTestInterval:   DurationInSeconds(600),
			// URLTestIdleTimeout: DurationInSeconds(6000),
		},
		RouteOptions: RouteOptions{
			ResolveDestination:     false,
			IPv6Mode:               option.DomainStrategy(C.DomainStrategyAsIS),
			BypassLAN:              false,
			AllowConnectionFromLAN: false,
		},
		LogLevel: "warn",
		// LogFile:        "/dev/null",
		LogFile:        "data/box.log",
		Region:         "other",
		EnableClashApi: true,

		ClashApiPort:   16756,
		ClashApiSecret: "",
		// GeoIPPath:      "geoip.db",
		// GeoSitePath:    "geosite.db",
		Rules: []Rule{},
		Mux: MuxOptions{
			Enable:     false,
			Padding:    true,
			MaxStreams: 8,
			Protocol:   "h2mux",
		},
		TLSTricks: TLSTricks{
			EnableFragment: false,
			FragmentSize:   "10-100",
			FragmentSleep:  "50-200",
			MixedSNICase:   false,
			EnablePadding:  false,
			PaddingSize:    "1200-1500",
		},
		UseXrayCoreWhenPossible: false,
	}
}

// Recursively set the fields marked as overridable
func setOverridableFields(v reflect.Value, t reflect.Type, overrides map[string]interface{}) {
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Check if the field has an "overridable" tag set to "true"
		overridableTag := fieldType.Tag.Get("overridable")
		if overridableTag == "true" {
			// Get the field's JSON tag name
			jsonTag := strings.Split(fieldType.Tag.Get("json"), ",")[0]
			if jsonTag == "" {
				continue
			}

			// Check if an override exists for this field
			if overrideValue, ok := overrides[jsonTag]; ok {
				// Ensure the override value can be set to the field type
				var parsedValue reflect.Value
				switch field.Kind() {
				case reflect.Bool:
					if boolVal, err := parseBool(overrideValue); err == nil {
						parsedValue = reflect.ValueOf(boolVal)
					}
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					if intVal, err := parseInt(overrideValue); err == nil {
						parsedValue = reflect.ValueOf(intVal)
					}
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					if uintVal, err := parseUint(overrideValue); err == nil {
						parsedValue = reflect.ValueOf(uintVal)
					}
				case reflect.String:
					parsedValue = reflect.ValueOf(overrideValue.(string))
					// Add more cases for other types as needed
				}

				// Set the field if we have a parsed value
				if parsedValue.IsValid() && parsedValue.Type().AssignableTo(field.Type()) {
					field.Set(parsedValue)
				}
			}
		}

		// If the field is a nested struct, recurse into it
		if field.Kind() == reflect.Struct {
			jsonTag := strings.Split(fieldType.Tag.Get("json"), ",")[0]

			data := overrides
			if jsonTag != "" {
				data1 := overrides[jsonTag]
				if data1 == nil {
					continue
				}
				data = data1.(map[string]interface{})
			}
			neastedType := fieldType.Type
			if data != nil {
				setOverridableFields(field, neastedType, data)
			}

		}
	}
}

// Helper functions for parsing
func parseBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case string:
		return strconv.ParseBool(v)
	case bool:
		return v, nil
	}
	return false, fmt.Errorf("invalid bool value")
}

func parseInt(value interface{}) (int64, error) {
	switch v := value.(type) {
	case string:
		return strconv.ParseInt(v, 10, 64)
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int(), nil
	}
	return 0, fmt.Errorf("invalid int value")
}

func parseUint(value interface{}) (uint64, error) {
	switch v := value.(type) {
	case string:
		return strconv.ParseUint(v, 10, 64)
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint(), nil
	}
	return 0, fmt.Errorf("invalid uint value")
}

func GetOverridableHiddifyOptions(overrides map[string][]string) *HiddifyOptions {
	overrideHiddify := HiddifyOptions{}

	// Convert flat overrides to nested structure
	nestedOverrides := convertFlatToNested(overrides)

	// Use reflection to iterate over the fields of HiddifyOptions
	v := reflect.ValueOf(&overrideHiddify).Elem()
	t := reflect.TypeOf(overrideHiddify)

	// Recursively set the fields that are marked as overridable
	setOverridableFields(v, t, nestedOverrides)

	return &overrideHiddify
}

// Converts the flat overrides map to a nested structure without removing underscores
func convertFlatToNested(overrides map[string][]string) map[string]interface{} {
	nested := make(map[string]interface{})
	for key, value := range overrides {
		keys := strings.Split(key, ".")
		current := nested

		for i, k := range keys {
			if i == len(keys)-1 {
				// Set the final value with underscores preserved
				current[k] = value[0]
			} else {
				// Create nested maps if they do not exist
				if _, exists := current[k]; !exists {
					current[k] = make(map[string]interface{})
				}
				current = current[k].(map[string]interface{})
			}
		}
	}
	return nested
}
