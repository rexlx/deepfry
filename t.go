package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Server) BulkSaveIp4(octect string, ips []Ip4) {
	defer func(t time.Time) {
		fmt.Printf("BulkSaveIp4 for octet %s with %d initial IPs took %v\n", octect, len(ips), time.Since(t))
	}(time.Now())

	if len(ips) == 0 {
		return
	}

	tableName := fmt.Sprintf("ip4_%s", octect)
	createTableQuery := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id SERIAL PRIMARY KEY,
				value VARCHAR(15) UNIQUE NOT NULL
			)
		`, tableName)
	if _, err := s.DB.Exec(context.Background(), createTableQuery); err != nil {
		fmt.Printf("DB_ERROR: Error creating table %s in BulkSaveIp4: %v\n", tableName, err)
		return
	}

	batchSize := 500
	totalIPsAttemptedInFunction := 0
	successfullyInsertedCount := 0

	for i := 0; i < len(ips); i += batchSize {
		end := i + batchSize
		if end > len(ips) {
			end = len(ips)
		}
		currentBatch := ips[i:end]

		if len(currentBatch) == 0 {
			continue
		}

		var valuePlaceholders []string
		var queryArgs []interface{}
		paramNumber := 1

		for _, ip := range currentBatch {
			if len(ip.Value) > 15 {
				continue
			}
			if len(ip.Value) == 0 {
				continue
			}
			valuePlaceholders = append(valuePlaceholders, fmt.Sprintf("($%d)", paramNumber))
			queryArgs = append(queryArgs, ip.Value)
			paramNumber++
		}

		if len(valuePlaceholders) == 0 {
			continue
		}

		totalIPsAttemptedInFunction += len(queryArgs)

		insertQuery := fmt.Sprintf("INSERT INTO %s (value) VALUES %s ON CONFLICT (value) DO NOTHING",
			tableName,
			strings.Join(valuePlaceholders, ", "),
		)

		fmt.Printf("DB_DEBUG: Query for %s: [%s] with %d args: %v\n", tableName, insertQuery, len(queryArgs), queryArgs)

		result, err := s.DB.Exec(context.Background(), insertQuery, queryArgs...)
		if err != nil {
			fmt.Printf("DB_ERROR: Error inserting batch into table %s: %v. Query was: [%s]. Args: %v\n",
				tableName, err, insertQuery, queryArgs)

			continue
		}
		rowsAffected := result.RowsAffected()
		successfullyInsertedCount += int(rowsAffected)

	}

	s.Memory.Lock()
	if s.Intel.SavedIp4Addresses == nil {
		s.Intel.SavedIp4Addresses = make(map[string]Ip4)
	}
	for _, ip := range ips {
		if len(ip.Value) > 0 && len(ip.Value) <= 15 {
			s.Intel.SavedIp4Addresses[ip.Value] = ip
		}
	}
	s.Memory.Unlock()

	if totalIPsAttemptedInFunction > 0 {
		fmt.Printf("DB_INFO: For octet %s, attempted to save %d valid IPs. Successfully inserted/conflicted: %d.\n", octect, totalIPsAttemptedInFunction, successfullyInsertedCount)
	}
}
