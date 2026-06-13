package db

import (
	"fmt"
	"log"
	"time"

	"github.com/gocql/gocql"
)

var Session *gocql.Session

func Connect(addr, keyspace string) error {
	cluster := gocql.NewCluster(addr)
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Consistency = gocql.LocalQuorum
	cluster.NumConns = 3

	session, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	if err := createKeyspace(session, keyspace); err != nil {
		return fmt.Errorf("create keyspace: %w", err)
	}

	cluster.Keyspace = keyspace
	Session, err = cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("create keyspace session: %w", err)
	}

	if err := createTables(Session); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	log.Printf("Connected to ScyllaDB at %s (keyspace: %s)", addr, keyspace)
	return nil
}

func Close() {
	if Session != nil {
		Session.Close()
	}
}

func createKeyspace(session *gocql.Session, keyspace string) error {
	return session.Query(fmt.Sprintf(`
		CREATE KEYSPACE IF NOT EXISTS %s
		WITH replication = {
			'class': 'SimpleStrategy',
			'replication_factor': 1
		}
	`, keyspace)).Exec()
}

func createTables(session *gocql.Session) error {
	return session.Query(`
		CREATE TABLE IF NOT EXISTS payments (
			bank_session_id  text,
			order_id         text,
			user_id          text,
			amount           bigint,
			currency         text,
			status           text,
			three_ds_url     text,
			fail_reason      text,
			return_url       text,
			idempotency_key  text,
			created_at       timestamp,
			updated_at       timestamp,
			PRIMARY KEY (bank_session_id)
		)
	`).Exec()
}
