// Package application define los puertos y casos de uso del servicio de búsqueda.
package application

import (
	"context"

	"github.com/manuXD270516/go-search-sync/internal/domain/catalog"
	"github.com/manuXD270516/go-search-sync/internal/domain/search"
)

// SearchEngine abstrae el motor de búsqueda.
//
// El dominio no conoce OpenSearch: sustituirlo por Elasticsearch, Typesense o
// cualquier otro es implementar esta interfaz.
type SearchEngine interface {
	SearchMedicines(ctx context.Context, q search.Query) (search.Result, error)
	Suggest(ctx context.Context, prefix string, size int) ([]string, error)
	GetMedicine(ctx context.Context, id string) (catalog.Medicine, error)
	Health(ctx context.Context) error
}

// AvailabilityReader consulta la disponibilidad en el motor de búsqueda.
type AvailabilityReader interface {
	ByMedicine(ctx context.Context, medicineID, city string, onlyInStock bool) ([]catalog.Availability, error)
}
