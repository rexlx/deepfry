package main

import (
	"context"
	"flag"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5"
)

var (
	dbAddr = flag.String("dbAddr", "fairlady:5432", "postgres database address")
	dbName = flag.String("dbName", "addresses", "postgres database name")
	dbUser = flag.String("dbUser", "rxlx", "postgres database user")
	dbPass = flag.String("dbPass", "thereISnosp0)n", "postgres database password")
)

type Server struct {
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
		panic(err)
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
			Ip4Addresses: make(map[string][]Ip4),
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

var BaseHtml string = `
<!DOCTYPE html>
<html>

<head>
  <meta http-equiv="Content-Security-Policy" content="connect-src 'self' https://localhost:8080">
  <script src="https://unpkg.com/htmx.org@2.0.4"
    integrity="sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+"
    crossorigin="anonymous"></script>
  <style>
    @font-face {
      font-family: 'Intel One Mono';
      src: url('/static/IntelOneMono-Regular.woff2') format('woff2'),
        url('/static/IntelOneMono-Regular.woff') format('woff');
      font-weight: normal;
      font-style: normal;
    }

    body {
      font-family: 'Intel One Mono', monospace;
      background-color: #040008;
      color: #4a7887;
    }

    .search-form {
      display: flex;
      margin-bottom: 1rem;
    }

    .search-form input[type="text"] {
      flex: 1;
      padding: 0.5rem;
      border: 1px solid #4a7887;
      border-radius: 4px 0 0 4px;
      background-color: #000;
      color: #4a7887;
    }

    .search-form button {
      padding: 0.5rem 1rem;
      border: 1px solid #4a7887;
      border-radius: 0 4px 4px 0;
      background-color: #4a7887;
      color: #000;
      cursor: pointer;
    }

    .search-form button:hover {
      background-color:rgb(89, 58, 119);
    }

    .container {
      background-color: #000;
      position: relative;
      width: 500px;
      /* Add a width to the container */
      height: 400px;
      overflow-y: scroll;
      margin: 0 auto;
      box-shadow: -10px 10px 20px #09b576;
    }
	
	.search-area {
	  background-color: #000;
	  position: relative;
	  width: 500px;
	  margin: 0 auto;
}

    .scrollbar {
      position: absolute;
      width: 10px;
      /* Add a width to the scrollbar */
      right: 0;
      top: 0;
      background-color: #222;
      /* Add background color for visibility */
    }

    .thumb {
      width: 100%;
      min-height: 20px;
      /* Ensure minimum height */
      background: #444;
      cursor: pointer;
    }

    .results {
      margin-bottom: 1rem;
      color:rgb(12, 160, 86);
    }

    /* Loading indicator styles */
    .loading {
      opacity: 0.5;
      /* Dim the content when loading */
      pointer-events: none;
      /* Prevent interactions while loading */
    }

    #loader {
      position: absolute;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      z-index: 10;
      display: none;
      /* Initially hidden */
      color: #0f0;
      font-size: 1.2em;
    }
    h1 {
      color:rgb(93, 104, 143);
    }
  </style>
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

  <script>
    const dataList = document.getElementById('data-list');
    const scrollbar = document.querySelector('.scrollbar');
    const thumb = document.querySelector('.thumb');
    const container = document.querySelector('.container');
    const loader = document.getElementById('loader');
	let currentColor = '#09b576';

    const itemsPerPage = 50; // Reduce items per page for smoother loading
    let startIndex = 0;
    let isFetching = false; // Flag to prevent concurrent fetches

    // Function to fetch data from the server
    async function fetchData(start, end) {
      if (isFetching) return; // Prevent fetching if already fetching
      isFetching = true;
      loader.style.display = 'block'; // Show loading indicator
      container.classList.add('loading'); // Dim the content

      try {
        const response = await fetch("http://localhost:8080/cache/ips?start=" + start + "&end=" + end);
        const data = await response.json();
        return data;
      } catch (error) {
        console.error("Failed to fetch data:", error);
        return []; // Return an empty array on error
      } finally {
        isFetching = false;
        loader.style.display = 'none'; // Hide loading indicator
        container.classList.remove('loading'); // Restore content visibility
      }
    }

    // Function to append data to the list
    function appendData(data) {
      data.forEach(item => {
        const li = document.createElement('li');
        li.textContent = item;
        dataList.appendChild(li);
      });
    }

    // Function to update the scrollbar thumb position and size
    function updateScrollbar() {
      const scrollTop = container.scrollTop;
      const scrollHeight = container.scrollHeight - container.clientHeight;
      const scrollPercent = scrollTop / scrollHeight;

      const thumbHeight = Math.max(20, container.clientHeight * (container.clientHeight / container.scrollHeight));
      const thumbTop = scrollPercent * (container.clientHeight - thumbHeight);

      thumb.style.height = thumbHeight + "px";
      thumb.style.top = thumbTop + "px";
    }

    // Initial data fetch and render
    fetchData(startIndex, startIndex + itemsPerPage)
      .then(data => {
        appendData(data);
        updateScrollbar();
        startIndex += data.length; // Update start index based on fetched data
      });

    container.addEventListener('scroll', () => {
      const { scrollTop, scrollHeight, clientHeight } = container;

      // Check if the user has scrolled to the bottom
      if (scrollTop + clientHeight >= scrollHeight - 50) { // 50px threshold
        fetchData(startIndex, startIndex + itemsPerPage)
          .then(data => {
            if (data.length > 0) {
              appendData(data);
              updateScrollbar();
			  currentColor = currentColor === 'red' ? '#09b576' : 'red';
	  		  container.style.boxShadow = '-10px 10px 20px ' + currentColor;
              startIndex += data.length; // Update start index based on fetched data
            }
          });
      }

      updateScrollbar();
    });
    function clearInput() {
    document.getElementById('search-input').value = '';
  }

    // Update scrollbar when the window is resized
    window.addEventListener('resize', updateScrollbar);
  </script>

</body>

</html>
`
