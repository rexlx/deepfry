package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	real   = flag.Bool("real", false, "no docker mode")
	dbAddr = flag.String("dbAddr", "fairlady:5432", "postgres database address")
	dbName = flag.String("dbName", "addresses", "postgres database name")
	dbUser = flag.String("dbUser", "rxlx", "postgres database user")
	dbPass = flag.String("dbPass", "thereISnosp0)n", "postgres database password")
)

type Server struct {
	Stats    Stats
	Stopch   chan struct{}
	Serverch chan Message
	Memory   *sync.RWMutex
	Addr     string
	Gateway  *http.ServeMux
	DB       *pgxpool.Pool
	Intel    Intel
}

type Intel struct {
	Stats             Stats                `json:"stats"`
	RuntimeStats      map[string][]float64 `json:"runtime_stats"`
	Ip4Addresses      map[string][]Ip4     `json:"ip4_addresses"`
	Md5Values         []MD5                `json:"md5_values"`
	SavedMd5Values    map[MD5]int          `json:"saved_md5_values"`
	SavedIp4Addresses map[string]Ip4       `json:"saved_ip4_addresses"`
}

func NewIntel() Intel {
	return Intel{
		Stats:             make(Stats),
		Ip4Addresses:      make(map[string][]Ip4),
		SavedMd5Values:    make(map[MD5]int),
		SavedIp4Addresses: make(map[string]Ip4),
		RuntimeStats:      make(map[string][]float64),
	}
}

type Message struct {
	Message string `json:"message"`
	Error   bool   `json:"error"`
}

func NewServer(dsn string) *Server {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		panic(fmt.Sprintf("Unable to parse DSN: %v\n", err))
	}

	var pool *pgxpool.Pool
	maxRetries := 5
	retryDelay := 5 * time.Second
	for i := 0; i < maxRetries; i++ {
		pool, err = pgxpool.NewWithConfig(context.Background(), config)
		if err == nil {
			conn, connErr := pool.Acquire(context.Background())
			if connErr == nil {
				pingErr := conn.Ping(context.Background())
				conn.Release()
				if pingErr == nil {
					fmt.Println("Successfully connected to PostgreSQL pool.")
					break
				}
				fmt.Printf("Ping failed after connecting to pool: %v\n", pingErr)
				err = pingErr
			} else {
				fmt.Printf("Failed to acquire connection from pool for initial check: %v\n", connErr)
				err = connErr
			}
		}
		if i < maxRetries-1 {
			fmt.Printf("Failed to connect to database (attempt %d/%d): %v. Retrying in %v...\n", i+1, maxRetries, err, retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if err != nil {
		panic(fmt.Sprintf("Unable to connect to database after %d retries: %v\n", maxRetries, err))
	}

	memory := &sync.RWMutex{}
	messagech := make(chan Message, 256)
	stopch := make(chan struct{})

	s := &Server{
		Stats:    make(Stats),
		Serverch: messagech,
		Stopch:   stopch,
		Addr:     ":8080",
		DB:       pool,
		Memory:   memory,
		Gateway:  http.NewServeMux(),
		Intel:    NewIntel(),
	}

	s.Gateway.HandleFunc("/ip4", s.Ip4Handler)
	s.Gateway.HandleFunc("/displaystats", s.DisplayStatsHandler)
	s.Gateway.HandleFunc("/bulk/ip4", s.BulkIp4Handler)
	s.Gateway.HandleFunc("/get/ip4", s.GetIp4Handler)
	s.Gateway.HandleFunc("/cache/ips", s.CachedIp4Handler)
	s.Gateway.HandleFunc("/api/ip4", s.GetIP4FromFormHandler)
	s.Gateway.HandleFunc("/api/ips", s.GetIpsAPIHandler)
	s.Gateway.HandleFunc("/view", s.Ipv4ViewHandler)
	s.Gateway.HandleFunc("/stats", s.GetStatsHandler)
	s.Gateway.HandleFunc("/urlstats", s.GetUrlStatsHandler)
	s.Gateway.HandleFunc("/toprequests", s.TopTenRequestHandler)
	s.Gateway.HandleFunc("/sortedurlstats", s.GetSortedStatsHandler)
	s.Gateway.HandleFunc("/urls", s.RequestedURLHandler)
	s.Gateway.Handle("/static/", http.StripPrefix("/static/", s.FileServer()))

	s.Stats["server_started"] = int(time.Now().Unix())
	return s
}

func (s *Server) FileServer() http.Handler {
	return http.FileServer(http.Dir("./static"))
}

func (s *Server) AddIp4(ip4 Ip4) {
	if !ip4.IsValid() {
		fmt.Printf("[AddIp4] Invalid IP format: %s\n", ip4.Value)
		return
	}
	if len(ip4.Value) == 0 {
		return
	}

	firstOctect := string(ip4.Value[0])
	if firstOctect == "0" || firstOctect == "127" {
		return
	}

	s.Memory.Lock()
	defer s.Memory.Unlock()

	if _, ok := s.Intel.Ip4Addresses[firstOctect]; !ok {
		s.Intel.Ip4Addresses[firstOctect] = []Ip4{}
	}
	s.Intel.Ip4Addresses[firstOctect] = append(s.Intel.Ip4Addresses[firstOctect], ip4)

}

func DsnFromEnv() string {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	if dbHost == "" || dbPort == "" || dbUser == "" || dbName == "" {
		fmt.Println("Warning: One or more DB environment variables (DB_HOST, DB_PORT, DB_USER, DB_NAME) are not set.")

		if dbHost == "" {
			panic("DB_HOST not set")
		}
		if dbPort == "" {
			panic("DB_PORT not set")
		}
		if dbUser == "" {
			panic("DB_USER not set")
		}
		if dbName == "" {
			panic("DB_NAME not set")
		}

	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbPort, dbName)
	return dsn
}

type Stats map[string]int
type OrderedStats []Stat
type Stat struct {
	Key   string `json:"key"`
	Value int    `json:"value"`
}

func (st Stats) DeleteStats(key string) {
	delete(st, key)
}

func (st Stats) IsSaved() bool {
	_, saved := st["saved"]
	return saved
}

func (st Stats) Save() {
	st["saved"] = 1
}

func (st Stats) Reset() {
	delete(st, "saved")
}

func (st Stats) ToSlice() []Stat {
	var statsSlice []Stat
	for key, value := range st {
		statsSlice = append(statsSlice, Stat{Key: key, Value: value})
	}
	return statsSlice
}

func (i *Intel) SetRuntimeStats(stats Stats) {
	if i.RuntimeStats == nil {
		i.RuntimeStats = make(map[string][]float64)
	}
	currentTime := float64(time.Now().Unix())

	for key, value := range stats {

		if key == "BulkSaveIp4_calls" {
			if _, ok := i.RuntimeStats[key]; !ok {
				i.RuntimeStats[key] = []float64{}
			}

			i.RuntimeStats[key] = append(i.RuntimeStats[key], currentTime, float64(value))

			maxHistory := 200
			if len(i.RuntimeStats[key]) > maxHistory {
				i.RuntimeStats[key] = i.RuntimeStats[key][len(i.RuntimeStats[key])-maxHistory:]
			}
		}
	}
}

func SortStatsMax(stats Stats) OrderedStats {
	ordered := stats.ToSlice()
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Value > ordered[j].Value
	})
	return ordered
}

var BaseHtml string = `
<!DOCTYPE html>
<html>

<head>
  <meta http-equiv="Content-Security-Policy" content="connect-src 'self' *">
  <script src="https://unpkg.com/htmx.org@2.0.4"
    integrity="sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+"
    crossorigin="anonymous"></script>
  <link rel="stylesheet" href="/static/cowpower.css">
</head>

<body>
  <div class="container">
    <div id="loader">Loading...</div>
    <div class="scrollbar">
      <div class="thumb"></div>
    </div>
    <h1>ip addresses</h1>
    <ul id="data-list"></ul>
  </div>
  <div class="search-area">
    <h1>search</h1>
    <form hx-post="/api/ip4" hx-target="#result" class="search-form" id="search-form"
      hx-on::after-request="clearInput()">
      <input type="text" name="ip" placeholder="Search IP4s" id="search-input">
      <button type="submit">search</button>
    </form>
    <div id="result" class="results"></div>
  </div>
  <div class="search-area" hx-get="/toprequests" hx-trigger="load" hx-target="#urls" hx-swap="outerHTML">
    <h1>top requests</h1>
	<div id="urls" class="urls"></div>
	</div>
  <div class="search-area" hx-get="/displaystats" hx-trigger="load" hx-target="#stats" hx-swap="outerHTML">
    <h1>stats</h1>
    <div id="stats" class="stats"></div>
  </div>

  <script src="/static/f.js"></script>

</body>

</html>
`
