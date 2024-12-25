package main

import (
	"context"
	"fmt"
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
