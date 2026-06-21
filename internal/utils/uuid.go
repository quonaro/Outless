package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

// GenerateUUIDFromTokenNode generates a deterministic UUID from tokenID and nodeID.
func GenerateUUIDFromTokenNode(tokenID, nodeID string) string {
	if tokenID == "" || nodeID == "" {
		return ""
	}
	h := md5.New()
	h.Write([]byte(tokenID))
	h.Write([]byte(nodeID))
	hash := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hash[0:8], hash[8:12], hash[12:16], hash[16:20], hash[20:32])
}
