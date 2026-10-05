package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/experimental/libbox"
)

// profileWithSelectableOutbound is the minimum proxy config needed for
// BuildConfig to produce a selector to route through.
const profileWithSelectableOutbound = `{
  "outbounds": [
    {"type": "socks", "tag": "proxy-a", "server": "127.0.0.1", "server_port": 1080}
  ]
}`

func buildFromSettings(t *testing.T, settingsJSON string) *HiddifyOptions {
	t.Helper()
	ResetRouteRuleWarnings()
	// Start from the defaults so a test only has to specify what it is about.
	// BuildConfig needs a scheme qualified DNS address and a mixed port.
	options := *DefaultHiddifyOptions()
	options.RemoteDnsAddress = "tcp://8.8.8.8"
	options.DirectDnsAddress = "223.5.5.5"
	options.MixedPort = 12334
	if err := json.Unmarshal([]byte(settingsJSON), &options); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	return &options
}

func buildAndMarshal(t *testing.T, settingsJSON string) (string, *HiddifyOptions) {
	t.Helper()
	opts := buildFromSettings(t, settingsJSON)
	ctx := libbox.BaseContext(nil)
	built, err := BuildConfig(ctx, opts, &ReadOptions{Content: profileWithSelectableOutbound})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}
	encoded, err := built.MarshalJSONContext(ctx)
	if err != nil {
		t.Fatalf("marshal built config: %v", err)
	}
	return string(encoded), opts
}

// This is the acceptance test from the task book: the exact payload the Flutter
// app sends (top level "route-rule", snake_case field names, outbound as the
// enum *name* string) must reach route.rules. Before the fix the domain count
// was always zero.
func TestUserRouteRuleFromAppPayloadReachesRoute(t *testing.T) {
	const settings = `{
	  "log-level": "warn",
	  "remote-dns-address": "tcp://8.8.8.8",
	  "direct-dns-address": "223.5.5.5",
	  "mixed-port": 12334,
	  "route-rule": {
	    "rules": [
	      {"list_order": 0, "enabled": true, "name": "netease direct",
	       "outbound": "direct", "domain_suffix": [".163.com"]}
	    ]
	  }
	}`
	encoded, opts := buildAndMarshal(t, settings)

	if len(opts.RouteRules.Rules) != 1 {
		t.Fatalf("expected 1 parsed route rule, got %d", len(opts.RouteRules.Rules))
	}
	if !strings.Contains(encoded, "163.com") {
		t.Fatalf("built config does not contain 163.com; route section:\n%s", extractRoute(t, encoded))
	}
	if !strings.Contains(extractRoute(t, encoded), OutboundDirectTag) {
		t.Fatalf("expected the rule to route to %q", OutboundDirectTag)
	}
}

// The same rule must also produce a DNS rule, otherwise a domain routed direct
// would still be resolved through the remote resolver.
func TestUserRouteRuleProducesMatchingDNSRule(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"outbound":"direct","domain_suffix":[".163.com"]}
	]}}`
	encoded, _ := buildAndMarshal(t, settings)

	route := extractRoute(t, encoded)
	dnsSection := extractDNS(t, encoded)
	if !strings.Contains(route, "163.com") {
		t.Fatalf("route rules missing the domain:\n%s", route)
	}
	if !strings.Contains(dnsSection, "163.com") {
		t.Fatalf("dns rules missing the domain:\n%s", dnsSection)
	}
	if !strings.Contains(dnsSection, DNSMultiDirectTag) {
		t.Fatalf("expected dns rule to use %q", DNSMultiDirectTag)
	}
}

// A proxy rule must resolve through the remote resolver.
func TestUserRouteRuleProxyUsesRemoteDNS(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"outbound":"proxy","domain_suffix":[".example-blocked.com"]}
	]}}`
	encoded, _ := buildAndMarshal(t, settings)
	dnsSection := extractDNS(t, encoded)
	if !strings.Contains(dnsSection, "example-blocked.com") {
		t.Fatalf("dns rules missing the domain:\n%s", dnsSection)
	}
	if !strings.Contains(dnsSection, DNSMultiRemoteTag) {
		t.Fatalf("expected dns rule to use %q", DNSMultiRemoteTag)
	}
}

// block has no outbound in this build, so it must be expressed as a reject
// action on both the route and the DNS side.
func TestUserRouteRuleBlockUsesRejectAction(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"outbound":"block","domain_suffix":[".ads.example.com"]}
	]}}`
	encoded, _ := buildAndMarshal(t, settings)
	route := extractRoute(t, encoded)
	if !strings.Contains(route, "ads.example.com") {
		t.Fatalf("route rules missing the domain:\n%s", route)
	}
	if !strings.Contains(route, `"action":"reject"`) {
		t.Fatalf("expected a reject action for a block rule:\n%s", route)
	}
}

// direct_with_fragment must map onto the fragment outbound.
func TestUserRouteRuleDirectWithFragment(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"outbound":"direct_with_fragment","domain":["frag.example.com"]}
	]}}`
	encoded, _ := buildAndMarshal(t, settings)
	if !strings.Contains(extractRoute(t, encoded), "direct-fragment") {
		t.Fatalf("expected the fragment outbound:\n%s", extractRoute(t, encoded))
	}
}

// The client serializes settings with FieldRename.kebab, but every option key
// must also be accepted in snake_case and camelCase, otherwise the whole
// settings object is dropped.
func TestSettingsKeySpellingsAreAccepted(t *testing.T) {
	spellings := []string{
		`{"route-rule":{"rules":[{"outbound":"direct","domain_suffix":[".a.test"]}]}}`,
		`{"route_rule":{"rules":[{"outbound":"direct","domain_suffix":[".a.test"]}]}}`,
		`{"routeRule":{"rules":[{"outbound":"direct","domain_suffix":[".a.test"]}]}}`,
		`{"routeRules":{"rules":[{"outbound":"direct","domain_suffix":[".a.test"]}]}}`,
	}
	for _, settings := range spellings {
		encoded, _ := buildAndMarshal(t, settings)
		if !strings.Contains(encoded, "a.test") {
			t.Fatalf("settings spelling was dropped: %s", settings)
		}
	}
	// snake_case option keys (e.g. enable_tun) must also be accepted.
	var options HiddifyOptions
	if err := json.Unmarshal([]byte(`{"enable_tun":true,"log_level":"debug","mixed_port":1234}`), &options); err != nil {
		t.Fatalf("unmarshal snake_case settings: %v", err)
	}
	if !options.EnableTun {
		t.Fatal("enable_tun was not applied")
	}
	if options.LogLevel != "debug" {
		t.Fatalf("log_level was not applied, got %q", options.LogLevel)
	}
	if options.MixedPort != 1234 {
		t.Fatalf("mixed_port was not applied, got %d", options.MixedPort)
	}
}

// The field names may also arrive in camelCase or with plural list names.
func TestRuleFieldSpellingsAreAccepted(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"listOrder":1,"outbound":"direct","domainSuffixes":[".camel.test"],"ruleSets":["my-set"]}
	]}}`
	opts := buildFromSettings(t, settings)
	if len(opts.RouteRules.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(opts.RouteRules.Rules))
	}
	rule := opts.RouteRules.Rules[0]
	if rule.ListOrder != 1 {
		t.Fatalf("listOrder not accepted, got %d", rule.ListOrder)
	}
	if len(rule.DomainSuffixes) != 1 || rule.DomainSuffixes[0] != ".camel.test" {
		t.Fatalf("domainSuffixes not accepted, got %v", rule.DomainSuffixes)
	}
	if len(rule.RuleSets) != 1 || rule.RuleSets[0] != "my-set" {
		t.Fatalf("ruleSets not accepted, got %v", rule.RuleSets)
	}
}

// Outbound may be sent as the enum number as well as the name.
func TestRuleOutboundNumbersAreAccepted(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"outbound":1,"domain_suffix":[".num.test"]},
	  {"outbound":3,"domain_suffix":[".num-block.test"]}
	]}}`
	opts := buildFromSettings(t, settings)
	if len(opts.RouteRules.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(opts.RouteRules.Rules))
	}
	if opts.RouteRules.Rules[0].Outbound != RouteOutboundDirect {
		t.Fatalf("outbound 1 should be direct, got %q", opts.RouteRules.Rules[0].Outbound)
	}
	if opts.RouteRules.Rules[1].Outbound != RouteOutboundBlock {
		t.Fatalf("outbound 3 should be block, got %q", opts.RouteRules.Rules[1].Outbound)
	}
}

// A rule that omits "enabled" must still be applied; only an explicit false
// disables it.
func TestRuleEnabledDefaultsToTrue(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"outbound":"direct","domain_suffix":[".omitted.test"]},
	  {"outbound":"direct","domain_suffix":[".explicit-false.test"],"enabled":false}
	]}}`
	encoded, _ := buildAndMarshal(t, settings)
	route := extractRoute(t, encoded)
	if !strings.Contains(route, "omitted.test") {
		t.Fatalf("rule without enabled should be applied:\n%s", route)
	}
	if strings.Contains(route, "explicit-false.test") {
		t.Fatalf("rule with enabled=false should be skipped:\n%s", route)
	}
}

// list_order decides the emitted order.
func TestRuleListOrderIsRespected(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"list_order":2,"outbound":"block","domain_suffix":[".third.test"]},
	  {"list_order":0,"outbound":"direct","domain_suffix":[".first.test"]},
	  {"list_order":1,"outbound":"proxy","domain_suffix":[".second.test"]}
	]}}`
	encoded, _ := buildAndMarshal(t, settings)
	route := extractRoute(t, encoded)
	first := strings.Index(route, "first.test")
	second := strings.Index(route, "second.test")
	third := strings.Index(route, "third.test")
	if first < 0 || second < 0 || third < 0 {
		t.Fatalf("not all rules were emitted:\n%s", route)
	}
	if !(first < second && second < third) {
		t.Fatalf("list_order was not respected: first=%d second=%d third=%d\n%s", first, second, third, route)
	}
}

// User rules must be evaluated before the region defaults so that they can
// override .cn / geosite-cn.
func TestUserRulesPrecedeRegionRules(t *testing.T) {
	const settings = `{"region":"cn","route-rule":{"rules":[
	  {"outbound":"proxy","domain_suffix":[".163.com"]}
	]}}`
	encoded, _ := buildAndMarshal(t, settings)
	route := extractRoute(t, encoded)
	userIndex := strings.Index(route, "163.com")
	regionIndex := strings.Index(route, "geosite-cn")
	if userIndex < 0 {
		t.Fatalf("user rule missing:\n%s", route)
	}
	if regionIndex < 0 {
		t.Fatalf("region rule missing:\n%s", route)
	}
	if userIndex > regionIndex {
		t.Fatalf("user rule must come before the region rule: user=%d region=%d\n%s", userIndex, regionIndex, route)
	}
}

// A declared rule set must be emitted in route.rule_set so the reference is
// resolvable.
func TestUserRuleSetDefinitionIsEmitted(t *testing.T) {
	const settings = `{
	  "route-rule": {"rules": [
	    {"outbound": "direct", "rule_set": ["my-cn"]}
	  ]},
	  "rule-set": [
	    {"tag": "my-cn", "type": "remote", "format": "binary",
	     "url": "https://example.com/cn.srs", "update_interval": "24h"}
	  ]
	}`
	encoded, _ := buildAndMarshal(t, settings)
	if !strings.Contains(encoded, "my-cn") {
		t.Fatalf("rule set not emitted:\n%s", encoded)
	}
	if !strings.Contains(encoded, "https://example.com/cn.srs") {
		t.Fatalf("rule set url not emitted:\n%s", encoded)
	}
	if warnings := RouteRuleWarnings(); len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

// Referencing a rule set that has no definition is a user mistake and must be
// reported rather than silently producing a config sing-box would reject.
func TestUndefinedRuleSetReferenceIsReported(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"outbound":"direct","rule_set":["does-not-exist"]}
	]}}`
	buildAndMarshal(t, settings)
	warnings := RouteRuleWarnings()
	if len(warnings) == 0 {
		t.Fatal("expected a warning about the undefined rule set")
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "does-not-exist") {
		t.Fatalf("warning does not name the rule set: %v", warnings)
	}
}

// A rule with no matcher would capture all traffic; it must be skipped instead.
func TestRuleWithoutMatcherIsSkipped(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"outbound":"direct"}
	]}}`
	encoded, _ := buildAndMarshal(t, settings)
	route := extractRoute(t, encoded)
	if strings.Count(route, OutboundDirectTag) > 1 {
		t.Fatalf("matcher-less rule should not be emitted:\n%s", route)
	}
}

// An unknown outbound value must be dropped rather than defaulting to proxy.
func TestUnknownOutboundIsSkipped(t *testing.T) {
	const settings = `{"route-rule":{"rules":[
	  {"outbound":"nonsense","domain_suffix":[".unknown-outbound.test"]}
	]}}`
	encoded, _ := buildAndMarshal(t, settings)
	if strings.Contains(encoded, "unknown-outbound.test") {
		t.Fatalf("rule with an unknown outbound should be skipped:\n%s", extractRoute(t, encoded))
	}
}

// A bare object without the rules wrapper, and a bare array, are both accepted.
func TestRouteRuleContainerShapes(t *testing.T) {
	for _, settings := range []string{
		`{"route-rule":[{"outbound":"direct","domain_suffix":[".shape.test"]}]}`,
	} {
		encoded, _ := buildAndMarshal(t, settings)
		if !strings.Contains(encoded, "shape.test") {
			t.Fatalf("container shape not accepted: %s", settings)
		}
	}
}

// The default CN rule set must come from a complete source. The historical
// hiddify-geo mirror excludes tencent / netease / jd / alibaba / bytedance, so
// major Chinese sites ended up proxied. This test pins the source so it cannot
// regress silently.
func TestDefaultRegionRuleSetUsesCompleteSource(t *testing.T) {
	const settings = `{"region":"cn"}`
	encoded, _ := buildAndMarshal(t, settings)
	route := extractRoute(t, encoded)

	if !strings.Contains(route, "meta-rules-dat") {
		t.Fatalf("region rule sets must come from the complete source:\n%s", route)
	}
	if !strings.Contains(route, "/geosite/cn.srs") {
		t.Fatalf("expected the complete geosite cn rule set:\n%s", route)
	}
	if !strings.Contains(route, "/geoip/cn.srs") {
		t.Fatalf("expected the complete geoip cn rule set:\n%s", route)
	}
	if strings.Contains(route, "hiddify-geo/rule-set/country") {
		t.Fatalf("region rule sets must not use the incomplete mirror by default:\n%s", route)
	}
	// The rule that routes the region rule sets direct must still be present.
	if !strings.Contains(route, "geosite-cn") || !strings.Contains(route, "geoip-cn") {
		t.Fatalf("region direct rule missing:\n%s", route)
	}
}

// The rule set source must be overridable, so a self hosted mirror keeps working.
// An override uses the flat "<base>/<name>.srs" layout.
func TestRegionRuleSetSourceIsOverridable(t *testing.T) {
	const settings = `{"region":"cn","rule_set_base_url":"https://mirror.example.com/rules"}`
	encoded, _ := buildAndMarshal(t, settings)
	route := extractRoute(t, encoded)
	if !strings.Contains(route, "https://mirror.example.com/rules/geosite-cn.srs") {
		t.Fatalf("override base url was not applied:\n%s", route)
	}
	if !strings.Contains(route, "https://mirror.example.com/rules/geoip-cn.srs") {
		t.Fatalf("override base url was not applied to geoip:\n%s", route)
	}
}

// The legacy flat mirror can still be selected deliberately.
func TestLegacyRegionRuleSetSourceCanBeSelected(t *testing.T) {
	settings := `{"region":"cn","rule_set_base_url":"` + LegacyRegionRuleSetBaseURL + `"}`
	encoded, _ := buildAndMarshal(t, settings)
	route := extractRoute(t, encoded)
	if !strings.Contains(route, LegacyRegionRuleSetBaseURL+"/geosite-cn.srs") {
		t.Fatalf("legacy base url layout was not preserved:\n%s", route)
	}
}

// The curated block lists keep using the flat layout and the upstream file
// names of the mirror (the tag and the file name differ for the ads list).
func TestBlockRuleSetsKeepFlatLayout(t *testing.T) {
	const settings = `{"region":"other","block-ads":true}`
	encoded, _ := buildAndMarshal(t, settings)
	route := extractRoute(t, encoded)
	for _, want := range []string{
		"hiddify-geo/rule-set/block/geosite-category-ads-all.srs",
		"hiddify-geo/rule-set/block/geosite-malware.srs",
		"hiddify-geo/rule-set/block/geoip-phishing.srs",
	} {
		if !strings.Contains(route, want) {
			t.Fatalf("block rule set url changed, missing %q:\n%s", want, route)
		}
	}
	if strings.Contains(route, "meta-rules-dat") {
		t.Fatalf("block lists must not be taken from the region source:\n%s", route)
	}
}

func extractRoute(t *testing.T, encoded string) string {
	t.Helper()
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(encoded), &parsed); err != nil {
		t.Fatalf("parse built config: %v", err)
	}
	raw, ok := parsed["route"]
	if !ok {
		t.Fatal("built config has no route section")
	}
	return string(raw)
}

func extractDNS(t *testing.T, encoded string) string {
	t.Helper()
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(encoded), &parsed); err != nil {
		t.Fatalf("parse built config: %v", err)
	}
	raw, ok := parsed["dns"]
	if !ok {
		t.Fatal("built config has no dns section")
	}
	return string(raw)
}
