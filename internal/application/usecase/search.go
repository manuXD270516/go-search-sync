// Package usecase orquesta las consultas al catálogo farmacéutico.
package usecase

import (
	"context"
	"fmt"

	"github.com/manuXD270516/go-search-sync/internal/application"
	"github.com/manuXD270516/go-search-sync/internal/domain/catalog"
	"github.com/manuXD270516/go-search-sync/internal/domain/search"
)

// Catalog agrupa los casos de uso de búsqueda y disponibilidad.
type Catalog struct {
	Engine       application.SearchEngine
	Availability application.AvailabilityReader
}

// SearchInput son los parámetros crudos que llegan desde la API.
type SearchInput struct {
	Text        string
	City        string
	OnlyInStock bool
	RequiresRx  *bool
	From, Size  int
}

// Search valida la consulta y la delega al motor.
func (c *Catalog) Search(ctx context.Context, in SearchInput) (search.Result, error) {
	q, err := search.NewQuery(in.Text, in.City, in.OnlyInStock, in.RequiresRx, in.From, in.Size)
	if err != nil {
		return search.Result{}, err
	}
	return c.Engine.SearchMedicines(ctx, q)
}

// Suggest devuelve sugerencias de autocompletado por prefijo.
func (c *Catalog) Suggest(ctx context.Context, prefix string, size int) ([]string, error) {
	if len(prefix) < 2 {
		return nil, fmt.Errorf("%w: se requieren al menos 2 caracteres", search.ErrEmptyQuery)
	}
	if size <= 0 || size > 20 {
		size = 10
	}
	return c.Engine.Suggest(ctx, prefix, size)
}

// MedicineDetail combina la ficha del medicamento con su disponibilidad.
type MedicineDetail struct {
	Medicine     catalog.Medicine       `json:"medicine"`
	Availability []catalog.Availability `json:"availability"`
}

// Detail recupera la ficha y dónde conseguir el medicamento.
func (c *Catalog) Detail(ctx context.Context, id, city string, onlyInStock bool) (MedicineDetail, error) {
	m, err := c.Engine.GetMedicine(ctx, id)
	if err != nil {
		return MedicineDetail{}, err
	}
	disp, err := c.Availability.ByMedicine(ctx, id, city, onlyInStock)
	if err != nil {
		return MedicineDetail{}, err
	}
	return MedicineDetail{Medicine: m, Availability: disp}, nil
}

// Health comprueba la salud del motor de búsqueda.
func (c *Catalog) Health(ctx context.Context) error { return c.Engine.Health(ctx) }
