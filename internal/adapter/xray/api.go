package xray

import (
	"encoding/json"

	xrayconf "github.com/xtls/xray-core/infra/conf"
)

// VerifyRoutingRuleJSON ensures a minimal routing rule can be built (used in tests).
func VerifyRoutingRuleJSON(ruleJSON []byte) error {
	rc := xrayconf.RouterConfig{
		RuleList: []json.RawMessage{ruleJSON},
	}
	_, err := rc.Build()
	return err
}
