package main

import (
	"log"
	"os"
	"time"

	"paygate/backend/internal/config"
	"paygate/backend/internal/db"

	"github.com/gocql/gocql"
)

func main() {
	cfg := config.Load()

	cluster := gocql.NewCluster(cfg.ScyllaAddr)
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Consistency = gocql.LocalQuorum
	cluster.NumConns = 3

	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("connect: %v", err)
	}

	if err := session.Query(`
		CREATE KEYSPACE IF NOT EXISTS ` + cfg.ScyllaKeyspace + `
		WITH replication = {
			'class': 'SimpleStrategy',
			'replication_factor': 1
		}
	`).Exec(); err != nil {
		log.Fatalf("create keyspace: %v", err)
	}

	cluster.Keyspace = cfg.ScyllaKeyspace
	session.Close()

	session, err = cluster.CreateSession()
	if err != nil {
		log.Fatalf("connect keyspace: %v", err)
	}
	defer session.Close()

	force := len(os.Args) > 1 && os.Args[1] == "--force"
	if force {
		db.SeedForce(session, cfg.ScyllaKeyspace)
	} else {
		db.Seed(session, cfg.ScyllaKeyspace)
	}
}
