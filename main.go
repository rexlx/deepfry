package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rexlx/parser"
)

func main() {
	flag.Parse()
	myLog := log.New(log.Writer(), "server: ", log.LstdFlags|log.Lshortfile)
	myDBConf, err := pgxpool.ParseConfig(*conn)
	if err != nil {
		myLog.Fatalf("failed to parse database config: %v", err)
	}
	var pool *pgxpool.Pool
	pool, err = pgxpool.NewWithConfig(context.Background(), myDBConf)
	if err != nil {
		myLog.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	server := &Server{
		ID:      "server1",
		Logger:  myLog,
		Parser:  parser.NewContextualizer(&parser.PrivateChecks{Ipv4: true}),
		DB:      pool,
		Memory:  &sync.RWMutex{},
		Gateway: http.NewServeMux(),
	}
	server.Gateway.HandleFunc("/search", server.HandleFindMatchByValue)
	server.Gateway.HandleFunc("/parse", server.ParserHandler)
	fmt.Println("Server is running on port 8080")
	if err := http.ListenAndServe(":8080", server.Gateway); err != nil {
		server.Logger.Fatalf("failed to start server: %v", err)
	}

}
