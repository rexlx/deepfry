package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rexlx/parser"
)

var (
	isReal = flag.Bool("isReal", false, "is not docker instance")
	conn   = flag.String("conn", "fairlady:5432", "postgres database address")
	dbAddr = flag.String("dbAddr", "fairlady:5432", "postgres database address")
	dbName = flag.String("dbName", "deepfry", "postgres database name")
	dbUser = flag.String("dbUser", "rxlx", "postgres database user")
	dbPass = flag.String("dbPass", "thereISnosp0)n", "postgres database password")
)

type Server struct {
	ID      string                 `json:"id"`
	Logger  *log.Logger            `json:"-"`
	Parser  *parser.Contextualizer `json:"-"`
	DB      *pgxpool.Pool          `json:"-"`
	Memory  *sync.RWMutex          `json:"-"`
	Gateway *http.ServeMux         `json:"-"`
}

type Match struct {
	Kind    string `json:"kind"`
	ID      int    `json:"id"`
	Created int64  `json:"created"`
	Saved   bool   `json:"saved"`
	Value   string `json:"value"`
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// --HELPERS
func NewSerer(id string, logger *log.Logger, parser *parser.Contextualizer, db *pgxpool.Pool) *Server {
	return &Server{
		ID:      id,
		Logger:  logger,
		Parser:  parser,
		DB:      db,
		Memory:  &sync.RWMutex{},
		Gateway: http.NewServeMux(),
	}
}

func sanitizeIdentifier(s string) string {
	return nonAlphanumericRegex.ReplaceAllString(s, "")
}

func (s *Server) ProcessAndSave(ctx context.Context, text string) {
	var wg sync.WaitGroup
	for kind, regex := range s.Parser.Expressions {
		matches := s.Parser.GetMatches(text, kind, regex)
		wg.Add(1)
		go func(kind string, matches []parser.Match) {
			defer wg.Done()
			for _, match := range matches {
				if err := s.SaveMatch(ctx, Match{
					Kind:    kind,
					Created: time.Now().Unix(),
					Saved:   true,
					Value:   match.Value,
				}); err != nil {
					log.Printf("Error saving match: %v", err)
				}
			}
		}(kind, matches)
	}
	wg.Wait()
}

// --DB
func (s *Server) SaveMatch(ctx context.Context, match Match) error {
	if len(match.Value) == 0 {
		return fmt.Errorf("cannot save a match with an empty value")
	}

	tablePrefix := sanitizeIdentifier(match.Kind)
	firstCharSuffix := sanitizeIdentifier(string(match.Value[0]))

	if tablePrefix == "" || firstCharSuffix == "" {
		return fmt.Errorf("could not generate a valid table name from match: %+v", match)
	}
	if firstCharSuffix == "." || firstCharSuffix == "/" {
		firstCharSuffix = "other"
	}

	tableName := fmt.Sprintf("%s_%s", tablePrefix, firstCharSuffix)

	createTableSQL := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
			created BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
			saved BOOLEAN NOT NULL DEFAULT FALSE,
			kind TEXT NOT NULL,
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL UNIQUE
        );`, tableName)
	// fmt.Println("Executing SQL:", createTableSQL)
	if _, err := s.DB.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("failed to execute 'CREATE TABLE' for table %s: %w", tableName, err)
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (created, saved, kind, value) VALUES ($1, $2, $3, $4) ON CONFLICT(value) DO NOTHING;`, tableName)
	// fmt.Println("Executing SQL:", insertSQL)
	if _, err := s.DB.Exec(ctx, insertSQL, match.Created, match.Saved, match.Kind, match.Value); err != nil {
		return fmt.Errorf("failed to execute 'INSERT INTO' for table %s: %w", tableName, err)
	}

	log.Printf("Successfully processed match. Table: %s, Value: %s", tableName, match.Value)
	return nil
}

func (s *Server) FindMatchByValue(ctx context.Context, kind string, value string) (*Match, error) {
	if kind == "" || value == "" {
		return nil, fmt.Errorf("kind and value cannot be empty")
	}
	start := time.Now()
	defer func(t time.Time) {
		log.Printf("FindMatchByValue took %s for kind '%s' and value '%s'", time.Since(t), kind, value)
	}(start)

	tablePrefix := sanitizeIdentifier(kind)
	firstCharSuffix := sanitizeIdentifier(string(value[0]))

	if tablePrefix == "" || firstCharSuffix == "" {
		return nil, fmt.Errorf("could not generate a valid table name from kind '%s' and value '%s'", kind, value)
	}
	if firstCharSuffix == "." || firstCharSuffix == "/" {
		firstCharSuffix = "other"
	}
	tableName := fmt.Sprintf("%s_%s", tablePrefix, firstCharSuffix)

	selectSQL := fmt.Sprintf(`
        SELECT id, value, kind, created, saved FROM %s WHERE value = $1
    `, tableName)

	row := s.DB.QueryRow(ctx, selectSQL, value)

	var foundMatch Match
	err := row.Scan(&foundMatch.ID, &foundMatch.Value, &foundMatch.Kind, &foundMatch.Created, &foundMatch.Saved)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("no match found for kind '%s' and value '%s'", kind, value)
		}
		return nil, fmt.Errorf("error querying table %s: %w", tableName, err)
	}

	return &foundMatch, nil
}

type MatchRequest struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type MatchResponse struct {
	Matched bool   `json:"matched"`
	Kind    string `json:"kind"`
	Value   string `json:"value"`
	ID      int    `json:"id,omitempty"`
	Created int64  `json:"created,omitempty"`
}

// --HANDLERS
func (s *Server) HandleFindMatchByValue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req MatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	match, err := s.FindMatchByValue(ctx, req.Kind, req.Value)
	if err != nil || match == nil {
		result := MatchResponse{
			Matched: false,
			Kind:    req.Kind,
			Value:   req.Value,
		}
		out, err := json.Marshal(result)
		if err != nil {
			http.Error(w, fmt.Sprintf("error encoding response: %v", err), http.StatusInternalServerError)
			return
		}
		go func() {
			if saveErr := s.SaveMatch(context.Background(), Match{
				Kind:    req.Kind,
				Created: time.Now().Unix(),
				Saved:   true,
				Value:   req.Value,
			}); saveErr != nil {
				log.Printf("error saving match for kind '%s' and value '%s': %v", req.Kind, req.Value, saveErr)
			}
		}()
		w.Header().Set("Content-Type", "application/json")
		// w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write(out); err != nil {
			log.Printf("error writing response: %v", err)
			return
		}
		return
	}
	result := MatchResponse{
		Matched: true,
		Kind:    match.Kind,
		Value:   match.Value,
		ID:      match.ID,
		Created: match.Created,
	}
	out, err := json.Marshal(result)
	if err != nil {
		http.Error(w, fmt.Sprintf("error encoding response: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(out); err != nil {
		log.Printf("error writing response: %v", err)
		return
	}
}

type ParserRequest struct {
	Value string `json:"value"`
}

func (s *Server) ParserHandler(w http.ResponseWriter, r *http.Request) {
	var results []Match
	var req ParserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if req.Value == "" {
		http.Error(w, "value cannot be empty", http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	for kind, regex := range s.Parser.Expressions {
		matches := s.Parser.GetMatches(req.Value, kind, regex)
		if len(matches) == 0 {
			continue
		}
		for _, match := range matches {
			m, err := s.FindMatchByValue(ctx, kind, match.Value)
			if err != nil {
				fmt.Println("Error finding match:", err)
				// if errors.Is(err, fmt.Errorf("no match found for kind '%s' and value '%s'", kind, match.Value)) {
				// If no match found, save it
				fmt.Println("No match found, saving new match:", match.Value)
				if err := s.SaveMatch(ctx, Match{
					Kind:    kind,
					Created: time.Now().Unix(),
					Saved:   true,
					Value:   match.Value,
				}); err != nil {
					log.Printf("error saving match for kind '%s' and value '%s': %v", kind, match.Value, err)
				}
			} else {
				results = append(results, *m)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	out, err := json.Marshal(results)
	if err != nil {
		http.Error(w, fmt.Sprintf("error encoding response: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(out); err != nil {
		log.Printf("error writing response: %v", err)
		return
	}
	s.Logger.Printf("Processed %d matches for value: %s", len(results), req.Value)
}
