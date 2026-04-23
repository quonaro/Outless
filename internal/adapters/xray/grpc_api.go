package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	handlercmd "github.com/xtls/xray-core/app/proxyman/command"
	routercmd "github.com/xtls/xray-core/app/router/command"
	xserial "github.com/xtls/xray-core/common/serial"
	xrayconf "github.com/xtls/xray-core/infra/conf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CheckXrayAPI verifies that gRPC HandlerService and RoutingService respond on adminURL.
func CheckXrayAPI(ctx context.Context, adminURL string) error {
	target, err := parseGRPCTarget(adminURL)
	if err != nil {
		return fmt.Errorf("parse admin url: %w", err)
	}
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("grpc dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	hs := handlercmd.NewHandlerServiceClient(conn)
	if _, err = hs.ListOutbounds(ctx, &handlercmd.ListOutboundsRequest{}); err != nil {
		return fmt.Errorf("handler list outbounds: %w", err)
	}

	rs := routercmd.NewRoutingServiceClient(conn)
	ruleTag := "_outless_health_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	probeRule, err := buildProbeRoutingConfig(ruleTag, defaultSocksInboundTag, "example.invalid", "direct")
	if err != nil {
		return fmt.Errorf("build health routing: %w", err)
	}
	tm := xserial.ToTypedMessage(probeRule)
	if _, err = rs.AddRule(ctx, &routercmd.AddRuleRequest{Config: tm, ShouldAppend: true}); err != nil {
		return fmt.Errorf("routing add rule (health): %w", err)
	}
	if _, err = rs.RemoveRule(ctx, &routercmd.RemoveRuleRequest{RuleTag: ruleTag}); err != nil {
		return fmt.Errorf("routing remove rule (health): %w", err)
	}

	return nil
}

// VerifyRoutingRuleJSON ensures a minimal routing rule can be built (used in tests).
func VerifyRoutingRuleJSON(ruleJSON []byte) error {
	rc := xrayconf.RouterConfig{
		RuleList: []json.RawMessage{ruleJSON},
	}
	_, err := rc.Build()
	return err
}
