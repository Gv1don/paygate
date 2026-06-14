package db

import (
	"log"
	"time"

	"github.com/gocql/gocql"
)

func Seed(session *gocql.Session, keyspace string) {
	var count int
	if err := session.Query("SELECT COUNT(*) FROM " + keyspace + ".payments").Scan(&count); err != nil {
		log.Printf("seed: count query: %v", err)
		return
	}
	if count > 0 {
		log.Printf("seed: %d payments already exist, skipping", count)
		return
	}
	insertSeedData(session, keyspace)
}

func SeedForce(session *gocql.Session, keyspace string) {
	if err := session.Query("TRUNCATE " + keyspace + ".payments").Exec(); err != nil {
		log.Printf("seed force: truncate: %v", err)
		return
	}
	insertSeedData(session, keyspace)
}

func insertSeedData(session *gocql.Session, keyspace string) {
	now := time.Now().UTC()
	payments := []struct {
		sessionID string
		orderID   string
		userID    string
		amount    int64
		currency  string
		status    string
		threeDS   string
		returnURL string
		createdAt time.Time
	}{
		{"bank_sys_seed000000000000000000000000000000000000000000000001", "order-001", "user-1", 150000, "RUB", "INITIATED", "", "http://localhost", now.Add(-24 * time.Hour)},
		{"bank_sys_seed000000000000000000000000000000000000000000000002", "order-002", "user-1", 2500, "USD", "INITIATED", "", "http://localhost", now.Add(-12 * time.Hour)},
		{"bank_sys_seed000000000000000000000000000000000000000000000003", "order-003", "user-2", 50000, "EUR", "PENDING_3DS", "t3d_003", "http://example.com", now.Add(-6 * time.Hour)},
		{"bank_sys_seed000000000000000000000000000000000000000000000004", "order-004", "user-2", 99999, "RUB", "PENDING_3DS", "t3d_004", "http://example.com", now.Add(-3 * time.Hour)},
		{"bank_sys_seed000000000000000000000000000000000000000000000005", "order-005", "user-3", 750, "USD", "CONFIRMED", "t3d_005", "http://localhost", now.Add(-2 * time.Hour)},
		{"bank_sys_seed000000000000000000000000000000000000000000000006", "order-006", "user-3", 12000, "RUB", "CONFIRMED", "t3d_006", "http://localhost", now.Add(-1 * time.Hour)},
		{"bank_sys_seed000000000000000000000000000000000000000000000007", "order-007", "user-1", 320000, "RUB", "COMPLETED", "t3d_007", "http://localhost", now.Add(-30 * time.Minute)},
		{"bank_sys_seed000000000000000000000000000000000000000000000008", "order-008", "user-4", 4500, "EUR", "COMPLETED", "t3d_008", "http://example.com", now.Add(-15 * time.Minute)},
		{"bank_sys_seed000000000000000000000000000000000000000000000009", "order-009", "user-4", 89000, "USD", "REFUNDED", "t3d_009", "http://example.com", now.Add(-5 * time.Minute)},
		{"bank_sys_seed000000000000000000000000000000000000000000000010", "order-010", "user-5", 19999, "RUB", "REFUNDED", "", "http://localhost", now},
	}

	for _, p := range payments {
		if err := session.Query(`
			INSERT INTO `+keyspace+`.payments (
				bank_session_id, order_id, user_id, amount, currency,
				status, three_ds_server_trans_id, return_url, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			p.sessionID, p.orderID, p.userID, p.amount, p.currency,
			p.status, p.threeDS, p.returnURL, p.createdAt, now,
		).Exec(); err != nil {
			log.Printf("seed: insert %s: %v", p.sessionID, err)
		}
	}

	log.Printf("seed: inserted %d test payments", len(payments))
}
