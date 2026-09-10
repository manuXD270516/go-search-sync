package search

import (
	"errors"
	"testing"
)

func TestNewQuery(t *testing.T) {
	rx := true
	tests := []struct {
		name     string
		text     string
		from     int
		size     int
		wantErr  error
		wantSize int
	}{
		{"consulta valida", "paracetamol", 0, 20, nil, 20},
		{"tamano por defecto", "ibuprofeno", 0, 0, nil, DefaultSize},
		{"tamano acotado al maximo", "amoxicilina", 0, 500, nil, MaxSize},
		{"texto vacio", "   ", 0, 20, ErrEmptyQuery, 0},
		{"pagina negativa", "aspirina", -1, 20, ErrInvalidPage, 0},
		{"pagina excesiva", "aspirina", MaxFrom + 1, 20, ErrInvalidPage, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q, err := NewQuery(tc.text, "Santa Cruz", true, &rx, tc.from, tc.size)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, se esperaba %v", err, tc.wantErr)
			}
			if err == nil && q.Size != tc.wantSize {
				t.Errorf("size = %d, se esperaba %d", q.Size, tc.wantSize)
			}
		})
	}
}

func TestNormalizaEspacios(t *testing.T) {
	q, err := NewQuery("  paracetamol  ", "  Santa Cruz  ", false, nil, 0, 10)
	if err != nil {
		t.Fatalf("new query: %v", err)
	}
	if q.Text != "paracetamol" || q.City != "Santa Cruz" {
		t.Fatalf("no se normalizaron los espacios: %q / %q", q.Text, q.City)
	}
}
