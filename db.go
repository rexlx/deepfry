package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Server) SaveIP4(ip Ip4) {
	firstOctect := string(ip.Value[:1])
	if firstOctect == "0" || firstOctect == "127" {
		return
	}
	tableName := fmt.Sprintf("ip4_%s", firstOctect)
	createTableQuery := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id SERIAL PRIMARY KEY,
				value VARCHAR(15) UNIQUE NOT NULL
			)
		`, tableName)
	if _, err := s.DB.Exec(context.Background(), createTableQuery); err != nil {
		fmt.Println(err)
		return
	}
	insertQuery := fmt.Sprintf("INSERT INTO %s (value) VALUES ($1)", tableName)
	if _, err := s.DB.Exec(context.Background(), insertQuery, ip.Value); err != nil {
		fmt.Println(err)
		return
	}
}

func (s *Server) TestConnection() {
	if _, err := s.DB.Exec(context.Background(), "SELECT 1"); err != nil {
		fmt.Println(err)
		s.Reconnect()
	}
}

// func (s *Server) SaveStats() {
// 	s.Memory.RLock()
// 	stats := s.Intel.Stats
// 	s.Memory.RUnlock()
// 	var sb strings.Builder
// 	sb.WriteString("INSERT INTO access (key, value) VALUES ")
// 	args := make([]interface{}, 0, len(stats)*2)
// 	i := 1
// 	for key, value := range stats {
// 		if i < 1 {
// 			sb.WriteString(", ")
// 		}
// 		sb.WriteString(fmt.Sprintf("($%d, $%d)", i, i+1))
// 		args = append(args, key, value)
// 		i += 2
// 	}
// 	sb.WriteString(" ON CONFLICT (key) DO UPDATE SET value = excluded.value")
// 	s.Memory.Lock()
// 	if _, err := s.DB.Exec(context.Background(), sb.String(), args...); err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	s.Memory.Unlock()
// }

func (s *Server) SaveStats() {
	s.Memory.Lock()
	defer s.Memory.Unlock()
	stats := s.Intel.Stats
	ok := stats.IsSaved()
	if ok {
		return
	}
	// Construct the SQL statement with placeholders
	var sb strings.Builder
	sb.WriteString("INSERT INTO access (key, value) VALUES ")
	args := make([]interface{}, 0, len(stats)*2)

	for i, key := range keys(stats) { // Use a helper function to get ordered keys
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		args = append(args, key, stats[key])
	}

	sb.WriteString(" ON CONFLICT (key) DO UPDATE SET value = excluded.value")

	if _, err := s.DB.Exec(context.Background(), sb.String(), args...); err != nil {
		fmt.Println(err)
		return
	}
	stats.Save() // Mark as saved
}

// Helper function to get map keys in a consistent order
func keys(m Stats) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Sort for consistent ordering
	return keys
}

func (s *Server) GetStats() map[string]int {
	createTableQuery := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		id SERIAL PRIMARY KEY,
		key VARCHAR(255) NOT NULL UNIQUE, -- Added UNIQUE constraint
		value INT NOT NULL
	)
`, "access")
	if _, err := s.DB.Exec(context.Background(), createTableQuery); err != nil {
		fmt.Println(err)
		return nil
	}
	selectQuery := "SELECT key, value FROM access"
	rows, err := s.DB.Query(context.Background(), selectQuery)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	stats := make(map[string]int)
	for rows.Next() {
		var key string
		var value int
		if err := rows.Scan(&key, &value); err != nil {
			fmt.Println(err)
			return nil
		}
		stats[key] = value
	}
	if err := rows.Err(); err != nil {
		fmt.Println(err)
		return nil
	}
	fmt.Println(stats)
	return stats
}

func (s *Server) BulkSaveIp4(octect string, ips []Ip4) {
	defer func(t time.Time) {
		fmt.Println("BulkSaveIp4 took", time.Since(t))
	}(time.Now())
	tableName := fmt.Sprintf("ip4_%s", octect)
	createTableQuery := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id SERIAL PRIMARY KEY,
				value VARCHAR(15) UNIQUE NOT NULL
			)
		`, tableName)
	if _, err := s.DB.Exec(context.Background(), createTableQuery); err != nil {
		fmt.Println(err)
		return
	}
	insertQuery := fmt.Sprintf("INSERT INTO %s (value) VALUES ", tableName)
	// s.Memory.Lock()
	if s.Intel.SavedIp4Addresses == nil {
		fmt.Println("WHT THE FLIP!!!!")
		s.Intel.SavedIp4Addresses = make(map[string]Ip4)
	}
	for i, ip := range ips {
		s.Intel.SavedIp4Addresses[ip.Value] = ip
		insertQuery += fmt.Sprintf("('%s')", ip.Value)
		if i != len(ips)-1 {
			insertQuery += ", "
		}
	}
	// s.Memory.Unlock()
	insertQuery += " ON CONFLICT (value) DO NOTHING"
	if _, err := s.DB.Exec(context.Background(), insertQuery); err != nil {
		fmt.Println(err)
		return
	}
}

func (s *Server) GetIP4s() map[string]Ip4 {
	defer func(t time.Time) {
		fmt.Println("GetIP4s took", time.Since(t))
	}(time.Now())
	ips := make(map[string]Ip4)
	for i := 1; i <= 9; i++ {
		tableName := fmt.Sprintf("ip4_%d", i)
		selectQuery := fmt.Sprintf("SELECT id, value FROM %s", tableName)
		rows, err := s.DB.Query(context.Background(), selectQuery)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		for rows.Next() {
			var ip Ip4
			if err := rows.Scan(&ip.ID, &ip.Value); err != nil {
				fmt.Println(err)
				return nil
			}
			ips[ip.Value] = ip
		}
		if err := rows.Err(); err != nil {
			fmt.Println(err)
			return nil
		}
	}
	return ips
}
