// seed fills database with demo data for screenshots.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Demo data configuration
var (
	countries = []struct {
		Code string
		Name string
		Flag string
	}{
		{"NL", "Netherlands", "🇳🇱"},
		{"DE", "Germany", "🇩🇪"},
		{"US", "United States", "🇺🇸"},
		{"GB", "United Kingdom", "🇬🇧"},
		{"FR", "France", "🇫🇷"},
		{"SG", "Singapore", "🇸🇬"},
		{"JP", "Japan", "🇯🇵"},
		{"CA", "Canada", "🇨🇦"},
		{"AU", "Australia", "🇦🇺"},
		{"FI", "Finland", "🇫🇮"},
	}

	groupNames = []string{
		"Premium EU",
		"Americas",
		"Asia Pacific",
		"Free Tier",
		"Test Group",
	}
)

// Group model
type Group struct {
	ID              string    `gorm:"primaryKey"`
	Name            string    `gorm:"uniqueIndex"`
	SourceURL       string
	TotalNodes      int
	HealthyNodes    int
	UnhealthyNodes  int
	UnknownNodes    int
	RandomEnabled   bool
	RandomLimit     *int
	LastSyncedAt    *time.Time
	CreatedAt       time.Time
}

// Node model
type Node struct {
	ID            string     `gorm:"primaryKey"`
	URL           string     `gorm:"primaryKey"`
	GroupID       string     `gorm:"primaryKey"`
	LatencyMs     int64
	Status        string
	Country       string
	LastCheckedAt *time.Time
}

// Token model
type Token struct {
	ID         string    `gorm:"primaryKey"`
	Owner      string
	TokenHash  string    `gorm:"uniqueIndex"`
	TokenPlain string    `gorm:"uniqueIndex"`
	UUID       string    `gorm:"uniqueIndex"`
	GroupID    *string
	IsActive   bool
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// TokenGroup model
type TokenGroup struct {
	TokenID   string    `gorm:"primaryKey"`
	GroupID   string    `gorm:"primaryKey"`
	CreatedAt time.Time
}

// Admin model
type Admin struct {
	ID           string    `gorm:"primaryKey"`
	Username     string    `gorm:"uniqueIndex"`
	PasswordHash string
	CreatedAt    time.Time
}

func main() {
	ctx := context.Background()

	dsn := getDSN()
	db, err := connectDB(dsn)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	if err := seed(ctx, db); err != nil {
		slog.Error("Failed to seed database", "error", err)
		os.Exit(1)
	}

	slog.Info("Database seeded successfully")
}

func getDSN() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://outless:outless@localhost:5432/outless?sslmode=disable"
}

func connectDB(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}

func seed(ctx context.Context, db *gorm.DB) error {
	// Clear existing data (in correct order due to FK constraints)
	db.Exec("DELETE FROM token_groups")
	db.Exec("DELETE FROM tokens")
	db.Exec("DELETE FROM nodes")
	db.Exec("DELETE FROM groups")
	db.Exec("DELETE FROM admins")

	// Seed groups
	groups, err := seedGroups(db)
	if err != nil {
		return fmt.Errorf("seeding groups: %w", err)
	}

	// Seed nodes
	if err := seedNodes(db, groups); err != nil {
		return fmt.Errorf("seeding nodes: %w", err)
	}

	// Seed tokens
	if err := seedTokens(db, groups); err != nil {
		return fmt.Errorf("seeding tokens: %w", err)
	}

	// Seed admin
	if err := seedAdmin(db); err != nil {
		return fmt.Errorf("seeding admin: %w", err)
	}

	return nil
}

func seedGroups(db *gorm.DB) ([]Group, error) {
	now := time.Now()
	groups := make([]Group, len(groupNames))

	for i, name := range groupNames {
		groups[i] = Group{
			ID:     fmt.Sprintf("grp_%d_%s", now.Unix(), uuid.New().String()[:8]),
			Name:   name,
			CreatedAt: now,
		}
		if name == "Free Tier" {
			groups[i].SourceURL = "https://example.com/free-nodes.txt"
			groups[i].LastSyncedAt = &now
		}
	}

	if err := db.Create(&groups).Error; err != nil {
		return nil, err
	}

	slog.Info("Created groups", "count", len(groups))
	return groups, nil
}

func seedNodes(db *gorm.DB, groups []Group) error {
	now := time.Now()
	baseHosts := []string{
		"proxy%d.example.com",
		"node%d.vpn-provider.net",
		"exit%d.outless.io",
		"vless%d.cloudproxy.org",
	}

	var nodes []Node
	nodeCount := 0

	for _, group := range groups {
		healthyCount := 0
		unhealthyCount := 0
		unknownCount := 0

		// Generate 8-15 nodes per group
		numNodes := 8 + (nodeCount % 8)
		
		for i := 0; i < numNodes; i++ {
			country := countries[nodeCount%len(countries)]
			host := fmt.Sprintf(baseHosts[nodeCount%len(baseHosts)], i+1)
			port := 443 + (nodeCount % 3)
			
			// Generate VLESS URL
			vlessUUID := uuid.New().String()
			url := fmt.Sprintf("vless://%s@%s:%d?encryption=none&flow=xtls-rprx-vision&security=reality&sni=www.google.com&fp=chrome&pbk=ZXZhbHVlZ2VuZXJhdGVkcHVibGlja2V5&type=tcp#%s%%20%s",
				vlessUUID, host, port, country.Name, group.Name)

			// Determine status with weighted randomness
			statusRoll := nodeCount % 10
			var status string
			var latency int64
			var lastChecked *time.Time

			switch {
			case statusRoll < 6: // 60% healthy
				status = "healthy"
				latency = int64(20 + (nodeCount % 150))
				t := now.Add(-time.Duration(nodeCount%60) * time.Minute)
				lastChecked = &t
				healthyCount++
			case statusRoll < 8: // 20% unhealthy
				status = "unhealthy"
				latency = 0
				t := now.Add(-time.Duration(nodeCount%120) * time.Minute)
				lastChecked = &t
				unhealthyCount++
			default: // 20% unknown
				status = "unknown"
				latency = 0
				unknownCount++
			}

			node := Node{
				ID:            fmt.Sprintf("node_%s_%d", group.ID[:8], i),
				URL:           url,
				GroupID:       group.ID,
				LatencyMs:     latency,
				Status:        status,
				Country:       country.Code,
				LastCheckedAt: lastChecked,
			}
			nodes = append(nodes, node)
			nodeCount++
		}

		// Update group stats
		db.Model(&Group{}).Where("id = ?", group.ID).Updates(map[string]any{
			"total_nodes":     numNodes,
			"healthy_nodes":   healthyCount,
			"unhealthy_nodes": unhealthyCount,
			"unknown_nodes":   unknownCount,
		})
	}

	if err := db.CreateInBatches(nodes, 50).Error; err != nil {
		return err
	}

	slog.Info("Created nodes", "count", len(nodes))
	return nil
}

func seedTokens(db *gorm.DB, groups []Group) error {
	now := time.Now()
	
	owners := []struct {
		Name   string
		Groups int // Number of groups to assign
		Active bool
	}{
		{"demo_user", 2, true},
		{"test_account", 1, true},
		{"premium_sub", 3, true},
		{"expired_user", 1, false},
		{"multi_group_user", 4, true},
		{"basic_plan", 1, true},
	}

	var tokens []Token
	var tokenGroups []TokenGroup

	for i, owner := range owners {
		tokenID := fmt.Sprintf("tok_%d_%s", now.Unix(), uuid.New().String()[:8])
		tokenUUID := uuid.New().String()
		tokenPlain := fmt.Sprintf("outless_%s_%s", owner.Name, uuid.New().String()[:12])
		
		// Hash the token
		hash, err := bcrypt.GenerateFromPassword([]byte(tokenPlain), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hashing token: %w", err)
		}

		expiresAt := now.AddDate(0, 1, 0) // 1 month from now
		if !owner.Active {
			expiresAt = now.AddDate(0, -1, 0) // Expired 1 month ago
		}

		// Select groups for this token
		selectedGroups := groups[:owner.Groups]
		var primaryGroupID *string
		if len(selectedGroups) > 0 {
			primaryGroupID = &selectedGroups[0].ID
		}

		token := Token{
			ID:         tokenID,
			Owner:      owner.Name,
			TokenHash:  string(hash),
			TokenPlain: tokenPlain,
			UUID:       tokenUUID,
			GroupID:    primaryGroupID,
			IsActive:   owner.Active,
			ExpiresAt:  expiresAt,
			CreatedAt:  now.Add(-time.Duration(i*24) * time.Hour),
		}
		tokens = append(tokens, token)

		// Create token-group associations
		for _, g := range selectedGroups {
			tokenGroups = append(tokenGroups, TokenGroup{
				TokenID:   tokenID,
				GroupID:   g.ID,
				CreatedAt: now,
			})
		}
	}

	if err := db.CreateInBatches(tokens, 10).Error; err != nil {
		return err
	}

	if err := db.CreateInBatches(tokenGroups, 20).Error; err != nil {
		return err
	}

	slog.Info("Created tokens", "count", len(tokens), "associations", len(tokenGroups))
	return nil
}

func seedAdmin(db *gorm.DB) error {
	now := time.Now()
	
	// Default admin credentials: admin / admin123
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing admin password: %w", err)
	}

	admin := Admin{
		ID:           fmt.Sprintf("adm_%d_%s", now.Unix(), uuid.New().String()[:8]),
		Username:     "admin",
		PasswordHash: string(passwordHash),
		CreatedAt:    now,
	}

	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	slog.Info("Created admin", "username", admin.Username, "password", "admin123")
	return nil
}
