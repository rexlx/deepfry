package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
)

type BulkIp4 struct {
	Values []string `json:"values"`
}

type Cache struct {
	IPs    []string      `json:"ips"`
	Memory *sync.RWMutex `json:"-"`
}

var LocalCache = Cache{
	IPs:    []string{},
	Memory: &sync.RWMutex{},
}

func (s *Server) Ip4Handler(w http.ResponseWriter, r *http.Request) {
	var ip4 Ip4
	if err := json.NewDecoder(r.Body).Decode(&ip4); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// fmt.Println(ip4)
	if !ip4.IsValid() {
		http.Error(w, "invalid ip4", http.StatusBadRequest)
		return
	}
	go s.AddIp4(ip4)
	// fmt.Println("Added IP4", ip4)
	res := struct {
		Message string `json:"message"`
	}{
		Message: "IP4 added",
	}
	out, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (s *Server) BulkIp4Handler(w http.ResponseWriter, r *http.Request) {
	var bulkIp4 BulkIp4
	if err := json.NewDecoder(r.Body).Decode(&bulkIp4); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, value := range bulkIp4.Values {
		ip4 := Ip4{Value: value}
		if !ip4.IsValid() {
			http.Error(w, "invalid ip4", http.StatusBadRequest)
			return
		}
		go s.AddIp4(ip4)
		// fmt.Println("Added IP4", ip4)
	}
	res := struct {
		Message string `json:"message"`
	}{
		Message: "IP4s added",
	}
	out, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (s *Server) Ip4sHandler(w http.ResponseWriter, r *http.Request) {
	s.Memory.RLock()
	out, _ := json.Marshal(s.Intel.Ip4Addresses)
	s.Memory.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (s *Server) Ipv4ViewHandler(w http.ResponseWriter, r *http.Request) {
	// view := fmt.Sprintf(BaseHtml)
	fmt.Fprint(w, BaseHtml)
}

func (s *Server) GetIpsAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	end, _ := strconv.Atoi(r.URL.Query().Get("end"))
	if start < 1 {
		start = 0
		end = 100
	}

	fmt.Println("CachedIp4Handler start", start, end)
	LocalCache.Memory.RLock()
	defer LocalCache.Memory.RUnlock()
	if end > len(LocalCache.IPs) {
		end = len(LocalCache.IPs)
	}
	var html string
	for _, item := range LocalCache.IPs[start:end] {
		html += fmt.Sprintf(`<p class="has-text-link">%v</p>`, item)
	}

	// Send the HTML response
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

func (s *Server) CachedIp4Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	end, _ := strconv.Atoi(r.URL.Query().Get("end"))
	if start < 0 {
		start = 0
	}
	fmt.Println("CachedIp4Handler start", start, end)
	LocalCache.Memory.RLock()
	defer LocalCache.Memory.RUnlock()
	if end > len(LocalCache.IPs) {
		end = len(LocalCache.IPs)
	}
	out, _ := json.Marshal(LocalCache.IPs[start:end])
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (s *Server) GetIP4FromFormHandler(w http.ResponseWriter, r *http.Request) {
	var out string
	ip := r.FormValue("ip")
	ip4 := Ip4{Value: ip}
	if !ip4.IsValid() {
		http.Error(w, "invalid ip4", http.StatusBadRequest)
		return
	}
	s.Memory.RLock()
	val, ok := s.Intel.SavedIp4Addresses[ip]
	s.Memory.RUnlock()
	if !ok {
		out = fmt.Sprintf("<p>IP4 not found %v...saving</p>", ip)
		go s.AddIp4(ip4)
	} else {
		out = fmt.Sprintf("<p>IP4 found %v</p>", val.Value)
	}
	fmt.Fprint(w, out)
}

func (s *Server) GetIp4Handler(w http.ResponseWriter, r *http.Request) {
	var ip Ip4
	if err := json.NewDecoder(r.Body).Decode(&ip); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res := struct {
		Message string `json:"message"`
		Value   string `json:"value"`
		Error   bool   `json:"error"`
		ID      int    `json:"id"`
	}{
		Message: "",
		Value:   ip.Value,
		Error:   false,
	}
	s.Memory.RLock()
	val, ok := s.Intel.SavedIp4Addresses[ip.Value]
	s.Memory.RUnlock()
	if !ok {
		res.Message = "IP4 not found"
		res.Error = true
		out, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
		go s.AddIp4(ip)
		return
	}
	res.Message = "IP4 found"
	res.ID = val.ID
	out, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}
