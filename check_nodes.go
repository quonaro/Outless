package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Node struct {
	ID        string    `gorm:"column:id"`
	GroupID   string    `gorm:"column:group_id"`
	URL       string    `gorm:"column:url"`
	Status    string    `gorm:"column:status"`
	Country   string    `gorm:"column:country"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

type Token struct {
	ID        string    `gorm:"column:id"`
	Owner     string    `gorm:"column:owner"`
	GroupID   *string   `gorm:"column:group_id"`
	UUID      string    `gorm:"column:uuid"`
	IsActive  bool      `gorm:"column:is_active"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

type TokenGroup struct {
	TokenID   string    `gorm:"column:token_id"`
	GroupID   string    `gorm:"column:group_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func main() {
	dsn := "host=localhost user=outless password=outless dbname=outless port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	ctx := context.Background()

	// Check nodes
	var nodes []Node
	if err := db.WithContext(ctx).Find(&nodes).Error; err != nil {
		log.Fatal("Failed to fetch nodes:", err)
	}

	fmt.Println("=== NODES ===")
	for _, node := range nodes {
		fmt.Printf("ID: %s, Group: %s, Status: %s, URL: %s\n", 
			node.ID, node.GroupID, node.Status, node.URL)
	}

	// Check tokens
	var tokens []Token
	if err := db.WithContext(ctx).Find(&tokens).Error; err != nil {
		log.Fatal("Failed to fetch tokens:", err)
	}

	fmt.Println("\n=== TOKENS ===")
	for _, token := range tokens {
		groupID := ""
		if token.GroupID != nil {
			groupID = *token.GroupID
		}
		fmt.Printf("ID: %s, UUID: %s, Active: %v, Group: %s, Expires: %s\n",
			token.ID, token.UUID, token.IsActive, groupID, token.ExpiresAt.Format(time.RFC3339))
	}

	// Check token groups
	var tokenGroups []TokenGroup
	if err := db.WithContext(ctx).Find(&tokenGroups).Error; err != nil {
		log.Fatal("Failed to fetch token groups:", err)
	}

	fmt.Println("\n=== TOKEN GROUPS ===")
	for _, tg := range tokenGroups {
		fmt.Printf("Token: %s -> Group: %s\n", tg.TokenID, tg.GroupID)
	}
}