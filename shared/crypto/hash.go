package crypto

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

// HashTokenNode generates an MD5 hash from tokenID and nodeID combination.
func HashTokenNode(tokenID, nodeID string) string {
	if tokenID == "" || nodeID == "" {
		return ""
	}
	h := md5.New()
	h.Write([]byte(tokenID))
	h.Write([]byte(nodeID))
	return hex.EncodeToString(h.Sum(nil))
}

// HashEmail formats the hash as a sing-box user email.
func HashEmail(hash string) string {
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("h-%s@outless", hash)
}
