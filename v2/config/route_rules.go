package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
)

// Enum values accepted from the client. Two encodings are supported because the
// clients send these values in different shapes:
//   - the Flutter app sends the proto3-JSON *name strings* published in
//     v2/config/route_rule.proto, i.e. "direct", "direct_with_fragment", "block"
//     (Dart's toProto3Json() emits the enum name, not its number);
//   - hand written settings and older configs use plain integers or short words.
//
// The top level key is also accepted under several spellings (see
// parseRouteRules) so that a mismatch can never again silently drop rules.
type RouteOutbound string

const (
	RouteOutboundProxy              RouteOutbound = "proxy"
	RouteOutboundDirect             RouteOutbound = "direct"
	RouteOutboundDirectWithFragment RouteOutbound = "direct_with_fragment"
	RouteOutboundBlock              RouteOutbound = "block"
)

var routeOutboundByNumber = map[int64]RouteOutbound{
	0: RouteOutboundProxy,
	1: RouteOutboundDirect,
	2: RouteOutboundDirectWithFragment,
	3: RouteOutboundBlock,
}

// UnmarshalJSON accepts a string or a number. Unknown values become "" and the
// rule is skipped, which is safer than defaulting an unknown value to proxy.
func (o *RouteOutbound) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*o = ""
		return nil
	}
	if trimmed[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		*o = normalizeRouteOutbound(raw)
		return nil
	}
	var number int64
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("invalid outbound value %s", trimmed)
	}
	*o = routeOutboundByNumber[number]
	return nil
}

func normalizeRouteOutbound(raw string) RouteOutbound {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(raw, "-", "_"))) {
	case "proxy":
		return RouteOutboundProxy
	case "direct":
		return RouteOutboundDirect
	case "direct_with_fragment", "directwithfragment", "fragment":
		return RouteOutboundDirectWithFragment
	case "block", "reject":
		return RouteOutboundBlock
	// the legacy model (v2/hiddifyoptions) called direct "bypass"
	case "bypass":
		return RouteOutboundDirect
	default:
		return ""
	}
}

// RouteNetwork mirrors the proto Network enum: all/tcp/udp.
type RouteNetwork string

func (n *RouteNetwork) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*n = ""
		return nil
	}
	if trimmed[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		*n = normalizeRouteNetwork(raw)
		return nil
	}
	var number int64
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("invalid network value %s", trimmed)
	}
	switch number {
	case 1:
		*n = "tcp"
	case 2:
		*n = "udp"
	default:
		*n = ""
	}
	return nil
}

func normalizeRouteNetwork(raw string) RouteNetwork {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "all", "":
		return ""
	case "tcp":
		return "tcp"
	case "udp":
		return "udp"
	default:
		return ""
	}
}

// RouteProtocol mirrors the proto Protocol enum.
type RouteProtocol string

func (p *RouteProtocol) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*p = ""
		return nil
	}
	if trimmed[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		*p = normalizeRouteProtocol(raw)
		return nil
	}
	var number int64
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("invalid protocol value %s", trimmed)
	}
	switch number {
	case 0:
		*p = C.ProtocolTLS
	case 1:
		*p = C.ProtocolHTTP
	case 2:
		*p = C.ProtocolQUIC
	case 3:
		*p = C.ProtocolSTUN
	case 4:
		*p = C.ProtocolDNS
	case 5:
		*p = C.ProtocolBitTorrent
	default:
		*p = ""
	}
	return nil
}

func normalizeRouteProtocol(raw string) RouteProtocol {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "tls":
		return C.ProtocolTLS
	case "http":
		return C.ProtocolHTTP
	case "quic":
		return C.ProtocolQUIC
	case "stun":
		return C.ProtocolSTUN
	case "dns":
		return C.ProtocolDNS
	case "bittorrent":
		return C.ProtocolBitTorrent
	default:
		return ""
	}
}

// RouteRuleEntry is one user defined routing rule. Every list field accepts either a
// JSON array or a single string so that both the app and hand written settings
// parse.
type RouteRuleEntry struct {
	ListOrder  int    `json:"list_order,omitempty"`
	Enabled    bool   `json:"enabled,omitempty"`
	Name       string `json:"name,omitempty"`
	Outbound   RouteOutbound `json:"outbound,omitempty"`
	RuleSets   ListableStrings `json:"rule_set,omitempty"`
	PackageNames ListableStrings `json:"package_name,omitempty"`
	ProcessNames ListableStrings `json:"process_name,omitempty"`
	ProcessPaths ListableStrings `json:"process_path,omitempty"`
	Network    RouteNetwork `json:"network,omitempty"`
	PortRanges ListableStrings `json:"port_range,omitempty"`
	SourcePortRanges ListableStrings `json:"source_port_range,omitempty"`
	Protocols  ListableRouteProtocols `json:"protocol,omitempty"`
	IPCIDRs    ListableStrings `json:"ip_cidr,omitempty"`
	SourceIPCIDRs ListableStrings `json:"source_ip_cidr,omitempty"`
	Domains    ListableStrings `json:"domain,omitempty"`
	DomainSuffixes ListableStrings `json:"domain_suffix,omitempty"`
	DomainKeywords ListableStrings `json:"domain_keyword,omitempty"`
	DomainRegexes ListableStrings `json:"domain_regex,omitempty"`

	// enabledExplicit records whether the client sent "enabled" at all. The app
	// always sends it, but hand written rules may omit it and must default to
	// enabled rather than to disabled.
	enabledExplicit bool
}

// UnmarshalJSON accepts both the canonical snake_case keys and the camelCase
// spellings, and both singular and plural list key names. Anything unknown is
// ignored so that adding a field on the client can never break the build.
func (r *RouteRuleEntry) UnmarshalJSON(data []byte) error {
	type plain RouteRuleEntry
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	normalized := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		normalized[normalizeRouteRuleKey(key)] = value
	}
	if _, ok := normalized["enabled"]; ok {
		r.enabledExplicit = true
	}
	reencoded, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	return json.Unmarshal(reencoded, (*plain)(r))
}

// normalizeRouteRuleKey maps every accepted spelling onto the canonical
// snake_case key used by RouteRuleEntry.
func normalizeRouteRuleKey(key string) string {
	canonical := toSnakeCase(key)
	switch canonical {
	// plural forms (proto field names) collapse onto the singular json_name
	case "rule_sets":
		return "rule_set"
	case "package_names":
		return "package_name"
	case "process_names":
		return "process_name"
	case "process_paths":
		return "process_path"
	case "port_ranges":
		return "port_range"
	case "source_port_ranges":
		return "source_port_range"
	case "protocols":
		return "protocol"
	case "ip_cidrs":
		return "ip_cidr"
	case "source_ip_cidrs":
		return "source_ip_cidr"
	case "domains":
		return "domain"
	case "domain_suffixes":
		return "domain_suffix"
	case "domain_keywords":
		return "domain_keyword"
	case "domain_regexes":
		return "domain_regex"
	// legacy aliases
	case "domain_suffixe":
		return "domain_suffix"
	case "source_ip":
		return "source_ip_cidr"
	case "ip":
		return "ip_cidr"
	}
	return canonical
}

// toSnakeCase lowercases the key and inserts underscores before upper case
// letters, so "listOrder" and "ListOrder" both become "list_order". Hyphens are
// treated like underscores.
func toSnakeCase(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return key
	}
	var builder strings.Builder
	builder.Grow(len(key) + 4)
	for i, char := range key {
		switch {
		case char == '-' || char == ' ':
			builder.WriteByte('_')
		case char >= 'A' && char <= 'Z':
			if i > 0 {
				previous := rune(key[i-1])
				if previous != '_' && previous != '-' && !(previous >= 'A' && previous <= 'Z') {
					builder.WriteByte('_')
				} else if previous >= 'A' && previous <= 'Z' && i+1 < len(key) {
					next := rune(key[i+1])
					if next >= 'a' && next <= 'z' {
						builder.WriteByte('_')
					}
				}
			}
			builder.WriteRune(char - 'A' + 'a')
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

// RouteRules wraps the list so that a bare array is accepted as well as the
// {"rules": [...]} object the app sends.
type RouteRules struct {
	Rules []RouteRuleEntry `json:"rules"`
}

func (r *RouteRules) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if trimmed[0] == '[' {
		return json.Unmarshal(data, &r.Rules)
	}
	type plain RouteRules
	return json.Unmarshal(data, (*plain)(r))
}

// ListableStrings accepts an array of strings, a single string, or a number.
type ListableStrings []string

func (l *ListableStrings) UnmarshalJSON(data []byte) error {
	raw, err := decodeFlexibleList(data)
	if err != nil {
		return err
	}
	*l = raw
	return nil
}

type ListableRouteProtocols []RouteProtocol

func (l *ListableRouteProtocols) UnmarshalJSON(data []byte) error {
	raw, err := decodeFlexibleList(data)
	if err != nil {
		return err
	}
	out := make([]RouteProtocol, 0, len(raw))
	for _, item := range raw {
		var protocol RouteProtocol
		if err := protocol.UnmarshalJSON([]byte(strconvQuote(item))); err != nil {
			return err
		}
		if protocol != "" {
			out = append(out, protocol)
		}
	}
	*l = out
	return nil
}

// decodeFlexibleList normalizes "a", ["a"], 3 and [3] into a []string.
func decodeFlexibleList(data []byte) ([]string, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, err
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			value, err := decodeFlexibleScalar(item)
			if err != nil {
				return nil, err
			}
			if value != "" {
				out = append(out, value)
			}
		}
		return out, nil
	}
	value, err := decodeFlexibleScalar(data)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}
	return []string{value}, nil
}

func decodeFlexibleScalar(data []byte) (string, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}
	if trimmed[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return "", err
		}
		return strings.TrimSpace(raw), nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return "", fmt.Errorf("unsupported value %s", trimmed)
	}
	return number.String(), nil
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// The config package has no logger injected into BuildConfig, so problems found
// while translating user rules are collected here. Tests assert on this, and the
// GUI build surfaces it so a typo in a rule does not fail silently.
var (
	routeRuleWarningsMu sync.Mutex
	routeRuleWarnings   []string
)

func addRouteRuleWarning(message string) {
	routeRuleWarningsMu.Lock()
	defer routeRuleWarningsMu.Unlock()
	routeRuleWarnings = append(routeRuleWarnings, message)
}

// RouteRuleWarnings returns the warnings accumulated so far.
func RouteRuleWarnings() []string {
	routeRuleWarningsMu.Lock()
	defer routeRuleWarningsMu.Unlock()
	out := make([]string, len(routeRuleWarnings))
	copy(out, routeRuleWarnings)
	return out
}

// ResetRouteRuleWarnings clears the collector (used by tests).
func ResetRouteRuleWarnings() {
	routeRuleWarningsMu.Lock()
	defer routeRuleWarningsMu.Unlock()
	routeRuleWarnings = nil
}

// IsEnabled reports whether the rule should be applied. A rule that omits
// "enabled" is treated as enabled.
func (r RouteRuleEntry) IsEnabled() bool {
	if !r.enabledExplicit {
		return true
	}
	return r.Enabled
}

// outboundTag resolves the outbound to the tag that actually exists in the
// generated config. Note there is no block outbound: blocking is expressed with
// a reject action by the callers.
func (r RouteRuleEntry) outboundTag() (string, bool) {
	switch r.Outbound {
	case RouteOutboundProxy:
		return OutboundMainDetour, true
	case RouteOutboundDirect:
		return OutboundDirectTag, true
	case RouteOutboundDirectWithFragment:
		return OutboundDirectFragmentTag, true
	}
	return "", false
}

// matchesAnything reports whether the rule would match every connection. Such a
// rule is meaningless (and would silently capture all traffic), so callers skip
// it.
func (r RouteRuleEntry) matchesAnything() bool {
	return len(r.IPCIDRs) == 0 &&
		len(r.SourceIPCIDRs) == 0 &&
		len(r.Domains) == 0 &&
		len(r.DomainSuffixes) == 0 &&
		len(r.DomainKeywords) == 0 &&
		len(r.DomainRegexes) == 0 &&
		len(r.RuleSets) == 0 &&
		len(r.PortRanges) == 0 &&
		len(r.SourcePortRanges) == 0 &&
		len(r.ProcessNames) == 0 &&
		len(r.ProcessPaths) == 0 &&
		len(r.PackageNames) == 0 &&
		len(r.Protocols) == 0 &&
		r.Network == ""
}

func listableNetwork(network RouteNetwork) []string {
	if network == "" {
		return nil
	}
	return []string{string(network)}
}

func protocolStrings(protocols []RouteProtocol) []string {
	if len(protocols) == 0 {
		return nil
	}
	out := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		if protocol != "" {
			out = append(out, string(protocol))
		}
	}
	return out
}

// MakeRouteRule converts the rule into a sing-box route rule. The second return
// value is false when the rule cannot be represented (empty matcher or unknown
// outbound).
func (r RouteRuleEntry) MakeRouteRule() (option.Rule, bool) {
	if r.matchesAnything() {
		return option.Rule{}, false
	}
	rule := option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{
				Network:         badoption.Listable[string](listableNetwork(r.Network)),
				Protocol:        badoption.Listable[string](protocolStrings(r.Protocols)),
				Domain:          badoption.Listable[string](r.Domains),
				DomainSuffix:    badoption.Listable[string](r.DomainSuffixes),
				DomainKeyword:   badoption.Listable[string](r.DomainKeywords),
				DomainRegex:     badoption.Listable[string](r.DomainRegexes),
				IPCIDR:          badoption.Listable[string](r.IPCIDRs),
				SourceIPCIDR:    badoption.Listable[string](r.SourceIPCIDRs),
				PortRange:       badoption.Listable[string](r.PortRanges),
				SourcePortRange: badoption.Listable[string](r.SourcePortRanges),
				ProcessName:     badoption.Listable[string](r.ProcessNames),
				ProcessPath:     badoption.Listable[string](r.ProcessPaths),
				PackageName:     badoption.Listable[string](r.PackageNames),
				RuleSet:         badoption.Listable[string](r.RuleSets),
			},
		},
	}
	if r.Outbound == RouteOutboundBlock {
		rule.DefaultOptions.RuleAction = option.RuleAction{
			Action: C.RuleActionTypeReject,
			RejectOptions: option.RejectActionOptions{
				Method: C.RuleActionRejectMethodDefault,
			},
		}
		return rule, true
	}
	tag, ok := r.outboundTag()
	if !ok {
		return option.Rule{}, false
	}
	rule.DefaultOptions.RuleAction = option.RuleAction{
		Action: C.RuleActionTypeRoute,
		RouteOptions: option.RouteActionOptions{
			Outbound: tag,
		},
	}
	return rule, true
}

// MakeDNSRule keeps DNS resolution aligned with the routing decision. Without
// this a domain routed direct would still be resolved through the remote
// resolver, which defeats the split tunnel.
//
// RawDefaultDNSRule is a standalone matcher (it does not embed RawDefaultRule),
// so the fields are set explicitly here.
func (r RouteRuleEntry) MakeDNSRule(hopt *HiddifyOptions) (option.DefaultDNSRule, bool) {
	if r.matchesAnything() {
		return option.DefaultDNSRule{}, false
	}
	rule := option.DefaultDNSRule{
		RawDefaultDNSRule: option.RawDefaultDNSRule{
			Network:       badoption.Listable[string](listableNetwork(r.Network)),
			Protocol:      badoption.Listable[string](protocolStrings(r.Protocols)),
			Domain:        badoption.Listable[string](r.Domains),
			DomainSuffix:  badoption.Listable[string](r.DomainSuffixes),
			DomainKeyword: badoption.Listable[string](r.DomainKeywords),
			DomainRegex:   badoption.Listable[string](r.DomainRegexes),
			ProcessName:   badoption.Listable[string](r.ProcessNames),
			ProcessPath:   badoption.Listable[string](r.ProcessPaths),
			PackageName:   badoption.Listable[string](r.PackageNames),
			RuleSet:       badoption.Listable[string](r.RuleSets),
		},
	}
	if r.Outbound == RouteOutboundBlock {
		rejectRCode := option.DNSRCode(dnsRcodeRefused)
		rule.DNSRuleAction = option.DNSRuleAction{
			Action: C.RuleActionTypePredefined,
			PredefinedOptions: option.DNSRouteActionPredefined{
				Rcode: &rejectRCode,
			},
		}
		return rule, true
	}
	if _, ok := r.outboundTag(); !ok {
		return option.DefaultDNSRule{}, false
	}
	server := DNSMultiDirectTag
	disableCache := false
	strategy := hopt.DirectDnsDomainStrategy
	rewriteTTL := DEFAULT_DNS_TTL
	if r.Outbound == RouteOutboundProxy {
		server = DNSMultiRemoteTag
		strategy = hopt.RemoteDnsDomainStrategy
		disableCache = hopt.EnableFakeDNS
	}
	rule.DNSRuleAction = option.DNSRuleAction{
		Action: C.RuleActionTypeRoute,
		RouteOptions: option.DNSRouteActionOptions{
			Server: server,
			AbstractDNSRouteActionOptions: option.AbstractDNSRouteActionOptions{
				Strategy:       strategy,
				RewriteTTL:     &rewriteTTL,
				DisableCache:   disableCache,
				BypassIfFailed: false,
			},
		},
	}
	return rule, true
}

// dnsRcodeRefused mirrors the RCODE used by the built in ad blocker so that
// blocked names fail fast instead of timing out (sdns.RcodeRefused == 5).
const dnsRcodeRefused = 5

// DefaultRegionRuleSetBaseURL serves complete region rule sets, with the layout
// "<base>/geosite/<name>.srs" and "<base>/geoip/<name>.srs".
//
// The historical hiddify-geo mirror is NOT used by default: its
// country/geosite-cn.srs is a 41 KB file copied from an unrelated (Iranian)
// project, and the upstream aggregate it derives from deliberately excludes
// tencent / netease / jd / alibaba / bytedance and friends, so major Chinese
// sites such as 163.com and jd.com are not matched and end up proxied.
// MetaCubeX's rule sets are generated from a complete aggregate.
const DefaultRegionRuleSetBaseURL = "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo"

// LegacyRegionRuleSetBaseURL is the previous mirror, where files are flat
// ("<base>/geosite-cn.srs"). It is kept so a user can deliberately pin the old
// behaviour.
const LegacyRegionRuleSetBaseURL = "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set/country"

// legacyFlatRuleSetBaseURL is the flat layout used for the curated block lists
// (ads, malware, ...), which hiddify-geo still serves.
const legacyFlatRuleSetBaseURL = "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set"

// defaultRuleSetURL builds the download URL for a built-in default rule set.
//
// A configured RuleSetBaseURL is used verbatim with a "<base>/<name>.srs"
// layout, so a flat self hosted mirror keeps working. Without an override the
// URL depends on which project actually publishes that file, which is spelled
// out in the two cases below rather than inferred from the name.
func defaultRuleSetURL(hopt *HiddifyOptions, name string) string {
	if base := strings.TrimSpace(hopt.RuleSetBaseURL); base != "" {
		base = strings.TrimSuffix(base, "/")
		if strings.Contains(base, "meta-rules-dat") {
			switch {
			case strings.HasPrefix(name, "geosite-"):
				return base + "/geosite/" + strings.TrimPrefix(name, "geosite-") + ".srs"
			case strings.HasPrefix(name, "geoip-"):
				return base + "/geoip/" + strings.TrimPrefix(name, "geoip-") + ".srs"
			}
		}
		return base + "/" + name + ".srs"
	}
	// Region rule sets: DefaultRegionRuleSetBaseURL keeps geosite and geoip in
	// separate subdirectories and takes the bare region name.
	if region, ok := strings.CutPrefix(name, "geosite-"); ok && !isCuratedBlockListName(region) {
		return DefaultRegionRuleSetBaseURL + "/geosite/" + region + ".srs"
	}
	if region, ok := strings.CutPrefix(name, "geoip-"); ok && !isCuratedBlockListName(region) {
		return DefaultRegionRuleSetBaseURL + "/geoip/" + region + ".srs"
	}
	// Curated block lists. The mirror serves them flat under "block/".
	if upstream, ok := curatedBlockListUpstreamName[name]; ok {
		return legacyFlatRuleSetBaseURL + "/block/" + upstream + ".srs"
	}
	return legacyFlatRuleSetBaseURL + "/block/" + name + ".srs"
}

// curatedBlockListUpstreamName maps the tag used in the generated config onto
// the file name the mirror publishes.
var curatedBlockListUpstreamName = map[string]string{
	"geosite-ads":          "geosite-category-ads-all",
	"geosite-malware":      "geosite-malware",
	"geosite-phishing":     "geosite-phishing",
	"geosite-cryptominers": "geosite-cryptominers",
	"geoip-phishing":       "geoip-phishing",
	"geoip-malware":        "geoip-malware",
}

// isCuratedBlockListName reports whether the suffix after "geosite-"/"geoip-"
// belongs to a curated block list tag rather than a region code.
func isCuratedBlockListName(suffix string) bool {
	switch suffix {
	case "ads", "malware", "phishing", "cryptominers", "category-ads-all":
		return true
	}
	return false
}

// RuleSetEntry declares a rule set that RouteRules (or the region defaults) can
// reference by tag. Without this the client could reference a rule set that has
// no definition, and sing-box would reject the config.
type RuleSetEntry struct {
	Tag            string `json:"tag,omitempty"`
	Type           string `json:"type,omitempty"`   // remote | local
	Format         string `json:"format,omitempty"` // binary | source
	URL            string `json:"url,omitempty"`
	Path           string `json:"path,omitempty"`
	UpdateInterval string `json:"update_interval,omitempty"`
	DownloadDetour string `json:"download_detour,omitempty"`
}

// ruleSetEntryKeyAliases maps the spelling-independent form of a key onto the
// canonical json tag, so that updateInterval / update-interval / update_interval
// all work.
var ruleSetEntryKeyAliases = map[string]string{
	"tag":            "tag",
	"type":           "type",
	"format":         "format",
	"url":            "url",
	"path":           "path",
	"updateinterval": "update_interval",
	"downloaddetour": "download_detour",
}

func (e *RuleSetEntry) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	normalized := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		canonical, ok := ruleSetEntryKeyAliases[normalizeOptionKey(key)]
		if !ok {
			continue
		}
		normalized[canonical] = value
	}
	reencoded, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	type plain RuleSetEntry
	return json.Unmarshal(reencoded, (*plain)(e))
}

// MakeRuleSet converts the entry into a sing-box rule set definition. The second
// return value is false when the entry cannot be represented.
func (e RuleSetEntry) MakeRuleSet() (option.RuleSet, bool) {
	tag := strings.TrimSpace(e.Tag)
	if tag == "" {
		return option.RuleSet{}, false
	}
	format := C.RuleSetFormatBinary
	if strings.EqualFold(strings.TrimSpace(e.Format), "source") {
		format = C.RuleSetFormatSource
	}
	switch strings.ToLower(strings.TrimSpace(e.Type)) {
	case "local":
		if strings.TrimSpace(e.Path) == "" {
			return option.RuleSet{}, false
		}
		return option.RuleSet{
			Type:   C.RuleSetTypeLocal,
			Tag:    badoption.Listable[string]{tag},
			Format: format,
			LocalOptions: option.LocalRuleSet{
				Path: strings.TrimSpace(e.Path),
			},
		}, true
	default: // remote
		if strings.TrimSpace(e.URL) == "" {
			return option.RuleSet{}, false
		}
		detour := OutboundSelectTag
		if strings.TrimSpace(e.DownloadDetour) != "" {
			detour = strings.TrimSpace(e.DownloadDetour)
		}
		remote := option.RemoteRuleSet{
			URL:            strings.TrimSpace(e.URL),
			DownloadDetour: detour,
		}
		if interval, err := parseRuleSetInterval(e.UpdateInterval); err == nil && interval > 0 {
			remote.UpdateInterval = badoption.Duration(interval)
		}
		return option.RuleSet{
			Type:          C.RuleSetTypeRemote,
			Tag:           badoption.Listable[string]{tag},
			Format:        format,
			RemoteOptions: remote,
		}, true
	}
}

// parseRuleSetInterval accepts a bare number of seconds or a Go duration string.
func parseRuleSetInterval(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	if seconds, err := strconv.Atoi(trimmed); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}
	return time.ParseDuration(trimmed)
}

