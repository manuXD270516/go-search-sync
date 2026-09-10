// Package search modela las consultas al motor de búsqueda.
package search

import (
	"errors"
	"strings"
)

// Errores de dominio de la búsqueda.
var (
	ErrEmptyQuery  = errors.New("empty query")
	ErrInvalidPage = errors.New("invalid pagination")
)

// Límites de paginación: acotan el costo de una consulta y evitan que un
// cliente pida páginas arbitrariamente grandes.
const (
	MaxSize     = 100
	DefaultSize = 20
	MaxFrom     = 10000
)

// Query es una consulta de búsqueda ya validada.
type Query struct {
	Text        string `json:"text"`
	City        string `json:"city"`
	OnlyInStock bool   `json:"only_in_stock"`
	RequiresRx  *bool  `json:"requires_rx,omitempty"`
	From        int    `json:"from"`
	Size        int    `json:"size"`
}

// NewQuery normaliza y valida los parámetros de búsqueda.
func NewQuery(text, city string, onlyInStock bool, requiresRx *bool, from, size int) (Query, error) {
	t := strings.TrimSpace(text)
	if t == "" {
		return Query{}, ErrEmptyQuery
	}
	if size <= 0 {
		size = DefaultSize
	}
	if size > MaxSize {
		size = MaxSize
	}
	if from < 0 || from > MaxFrom {
		return Query{}, ErrInvalidPage
	}
	return Query{
		Text: t, City: strings.TrimSpace(city), OnlyInStock: onlyInStock,
		RequiresRx: requiresRx, From: from, Size: size,
	}, nil
}

// Hit es un resultado individual con su puntaje de relevancia.
type Hit struct {
	ID     string         `json:"id"`
	Score  float64        `json:"score"`
	Source map[string]any `json:"source"`
}

// Result es la respuesta completa de una búsqueda.
type Result struct {
	Total  int64 `json:"total"`
	Hits   []Hit `json:"hits"`
	TookMS int64 `json:"took_ms"`
	Query  Query `json:"query"`
}
