package server

import (
    "encoding/json"
    "log"
    "net/http"
    "sort"

    "github.com/yourusername/sequence-calc/internal/models"
)

func (s *Server) handleCalculate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var requests []models.Request
    if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    grouped := make(map[float64][]int)
    for _, req := range requests {
        grouped[req.R] = append(grouped[req.R], req.N)
    }
    
    for r := range grouped {
        sort.Ints(grouped[r])
    }

    ctx := r.Context()
    responses := make([]models.Response, 0, len(requests))
    
    for r, nValues := range grouped {
        for _, n := range nValues {
            result, err := s.engine.Compute(ctx, r, n)
            if err != nil {
                log.Printf("Compute error: %v", err)
                continue
            }
            responses = append(responses, models.Response{R: r, N: n, Result: result})
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(responses)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}