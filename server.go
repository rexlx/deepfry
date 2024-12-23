package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	real   = flag.Bool("real", false, "no docker mode")
	dbAddr = flag.String("dbAddr", "fairlady:5432", "postgres database address")
	dbName = flag.String("dbName", "addresses", "postgres database name")
	dbUser = flag.String("dbUser", "rxlx", "postgres database user")
	dbPass = flag.String("dbPass", "thereISnosp0)n", "postgres database password")
)

type Server struct {
	Stopch   chan struct{}
	Serverch chan Message
	Memory   *sync.RWMutex
	Addr     string
	Gateway  *http.ServeMux
	DB       *pgx.Conn
	Intel    Intel
}

type Intel struct {
	Ip4Addresses      map[string][]Ip4 `json:"ip4_addresses"`
	Md5Values         []MD5            `json:"md5_values"`
	SavedMd5Values    map[MD5]int      `json:"saved_md5_values"`
	SavedIp4Addresses map[string]Ip4   `json:"saved_ip4_addresses"`
}

type Message struct {
	Message string `json:"message"`
	Error   bool   `json:"error"`
}

func NewServer(dsn string) *Server {
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		time.Sleep(5 * time.Second)
		conn, err = pgx.Connect(context.Background(), dsn)
		if err != nil {
			panic(err)
		}
	}
	memory := &sync.RWMutex{}
	messagech := make(chan Message, 256)
	s := &Server{
		Serverch: messagech,
		Addr:     ":8080",
		DB:       conn,
		Memory:   memory,
		Gateway:  http.NewServeMux(),
		Intel: Intel{
			Ip4Addresses:      make(map[string][]Ip4),
			SavedMd5Values:    make(map[MD5]int),
			SavedIp4Addresses: make(map[string]Ip4),
		},
	}
	s.Gateway.HandleFunc("/ip4", s.Ip4Handler)
	s.Gateway.HandleFunc("/bulk/ip4", s.BulkIp4Handler)
	s.Gateway.HandleFunc("/get/ip4", s.GetIp4Handler) //
	s.Gateway.HandleFunc("/cache/ips", s.CachedIp4Handler)
	s.Gateway.HandleFunc("/api/ip4", s.GetIP4FromFormHandler)
	s.Gateway.HandleFunc("/api/ips", s.GetIpsAPIHandler)
	s.Gateway.HandleFunc("/view", s.Ipv4ViewHandler)
	s.Gateway.Handle("/static/", http.StripPrefix("/static/", s.FileServer()))
	// s.Gateway.HandleFunc("/md5", s.Md5Handler)
	s.Intel.SavedIp4Addresses["127.0.0.1"] = Ip4{Value: "127.0.0.1"}
	return s
}

func (s *Server) FileServer() http.Handler {
	return http.FileServer(http.Dir("./static"))
}

func (s *Server) AddIp4(ip4 Ip4) {
	firstOctect := string(ip4.Value[:1])
	if firstOctect == "0" || firstOctect == "127" {
		return
	}
	s.Memory.Lock()
	defer s.Memory.Unlock()
	_, ok := s.Intel.Ip4Addresses[firstOctect]
	if !ok {
		s.Intel.Ip4Addresses[firstOctect] = []Ip4{}
	}
	s.Intel.Ip4Addresses[firstOctect] = append(s.Intel.Ip4Addresses[firstOctect], ip4)
	// if we want to enforce a maximum length, we could do it here
}

func DsnFromEnv() string {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	fmt.Println("postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPort + "/" + dbName)
	return "postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPort + "/" + dbName
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
      <form hx-post="/api/ip4" hx-target="#result" class="search-form" id="search-form" hx-on::after-request="clearInput()">
        <input type="text" name="ip" placeholder="Search IP4s" id="search-input">
        <button type="submit">search</button>
      </form>
	  <div id="result" class="results"></div>
    </div>

  <script src="/static/f.js"></script>

</body>

</html>
`
