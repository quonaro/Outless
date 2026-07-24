package service

import (
	"fmt"
	"log/slog"

	"outless/internal/domain"
	"outless/shared/vless"
)

const (
	originVlessScheme     = "vless"
	originSecurityNone    = "none"
	originSecurityTLS     = "tls"
	originSecurityReality = "reality"
	originNetworkTCP      = "tcp"
)

func (s *SubscriptionService) nodeUsesOrigins(
	node domain.Node,
	groupSettings map[string]domain.Group,
	token domain.Token,
) bool {
	allowedGroups := allowedGroupsForToken(token)
	allGroupsAllowed := len(allowedGroups) == 0
	for _, groupID := range node.GroupIDs {
		if !allGroupsAllowed {
			if _, ok := allowedGroups[groupID]; !ok {
				continue
			}
		}
		settings, ok := groupSettings[groupID]
		if ok && settings.ShowOrigins {
			return true
		}
	}
	return false
}

func (s *SubscriptionService) nodeUsesHub(
	node domain.Node,
	groupSettings map[string]domain.Group,
	token domain.Token,
) bool {
	allowedGroups := allowedGroupsForToken(token)
	allGroupsAllowed := len(allowedGroups) == 0
	for _, groupID := range node.GroupIDs {
		if !allGroupsAllowed {
			if _, ok := allowedGroups[groupID]; !ok {
				continue
			}
		}
		settings, ok := groupSettings[groupID]
		if ok && !settings.ShowOrigins {
			return true
		}
	}
	return false
}

func originRemark(parsed vless.Parsed, node domain.Node) string {
	if parsed.Name != "" {
		return parsed.Name
	}
	return node.ID
}

func originSecurity(parsed vless.Parsed) string {
	if parsed.Security != "" {
		return parsed.Security
	}
	return originSecurityNone
}

func (s *SubscriptionService) buildClashMetaProxyFromOrigin(node domain.Node) (ClashMetaProxy, string) {
	if node.URL == "" {
		return ClashMetaProxy{}, ""
	}
	parsed, err := vless.ParseURL(node.URL)
	if err != nil {
		s.logger.Warn("failed to parse node URL for Clash Meta origin", slog.String("node_id", node.ID), slog.String("error", err.Error()))
		return ClashMetaProxy{}, ""
	}
	remark := originRemark(parsed, node)
	security := originSecurity(parsed)
	tls := security == originSecurityTLS || security == originSecurityReality
	fingerprint := parsed.FP
	if fingerprint == "" {
		fingerprint = defaultFingerprint
	}
	sni := parsed.SNI
	if sni == "" {
		sni = defaultSNI
	}
	network := parsed.Network
	if network == "" {
		network = originNetworkTCP
	}
	var realityOpts *ClashRealityOpts
	if security == originSecurityReality {
		realityOpts = &ClashRealityOpts{
			PublicKey: parsed.PBK,
			ShortID:   parsed.SID,
		}
	}
	return ClashMetaProxy{
		Name:        remark,
		Type:        originVlessScheme,
		Server:      parsed.Host,
		Port:        parsed.Port,
		UUID:        parsed.UUID,
		UDP:         true,
		Flow:        parsed.Flow,
		Network:     network,
		TLS:         tls,
		ServerName:  sni,
		Fingerprint: fingerprint,
		RealityOpts: realityOpts,
		Encryption:  parsed.Encryption,
	}, remark
}

func (s *SubscriptionService) buildSingBoxOutboundFromOrigin(node domain.Node) SingBoxOutbound {
	remark := node.ID
	if node.URL == "" {
		return SingBoxOutbound{Type: originVlessScheme, Tag: remark}
	}
	parsed, err := vless.ParseURL(node.URL)
	if err != nil {
		s.logger.Warn("failed to parse node URL for Sing-box origin", slog.String("node_id", node.ID), slog.String("error", err.Error()))
		return SingBoxOutbound{Type: originVlessScheme, Tag: remark}
	}
	if parsed.Name != "" {
		remark = parsed.Name
	}
	security := originSecurity(parsed)
	tls := &SingBoxTLS{Enabled: security == originSecurityTLS || security == originSecurityReality, ServerName: parsed.SNI}
	if parsed.FP != "" {
		tls.UTLS = &SingBoxUTLS{Enabled: true, Fingerprint: parsed.FP}
	}
	if security == originSecurityReality {
		tls.Reality = &SingBoxReality{
			Enabled:   true,
			PublicKey: parsed.PBK,
			ShortID:   parsed.SID,
		}
	}
	network := parsed.Network
	if network == "" {
		network = originNetworkTCP
	}
	return SingBoxOutbound{
		Type:           originVlessScheme,
		Tag:            remark,
		Server:         parsed.Host,
		ServerPort:     parsed.Port,
		UUID:           parsed.UUID,
		Flow:           parsed.Flow,
		Network:        network,
		PacketEncoding: "xudp",
		TLS:            tls,
	}
}

func (s *SubscriptionService) buildSurgeProxyFromOrigin(node domain.Node) (string, string) {
	if node.URL == "" {
		return "", ""
	}
	parsed, err := vless.ParseURL(node.URL)
	if err != nil {
		s.logger.Warn("failed to parse node URL for Surge origin", slog.String("node_id", node.ID), slog.String("error", err.Error()))
		return "", ""
	}
	remark := originRemark(parsed, node)
	security := originSecurity(parsed)
	tls := security == originSecurityTLS || security == originSecurityReality
	sni := parsed.SNI
	if sni == "" {
		sni = defaultSNI
	}
	fingerprint := parsed.FP
	if fingerprint == "" {
		fingerprint = defaultFingerprint
	}
	pbk := parsed.PBK
	if pbk == "" {
		pbk = "-"
	}
	sid := parsed.SID
	if sid == "" {
		sid = defaultShortID
	}
	flow := parsed.Flow
	if flow == "" {
		flow = defaultFlow
	}
	line := fmt.Sprintf("%s = vless, %s, %d, uuid=%s, tls=%t, sni=%s, fp=%s, pbk=%s, sid=%s, flow=%s",
		remark, parsed.Host, parsed.Port, parsed.UUID, tls, sni, fingerprint, pbk, sid, flow)
	return line, remark
}
