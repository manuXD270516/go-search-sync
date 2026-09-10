// Package catalog modela el catálogo farmacéutico que se indexa y consulta.
//
// El rubro elegido es la disponibilidad de medicamentos en farmacias: un
// dominio donde la búsqueda tiene exigencias reales —tolerancia a errores de
// tipeo en nombres difíciles, sinónimos entre nombre comercial y principio
// activo, y filtrado por cercanía y stock— que justifican un motor de búsqueda
// además de la base relacional.
package catalog

import (
	"errors"
	"strings"
	"time"
)

// Errores de dominio del catálogo.
var (
	ErrInvalidMedicine = errors.New("invalid medicine")
	ErrNotFound        = errors.New("medicine not found")
)

// Medicine es un medicamento del catálogo.
type Medicine struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`              // nombre comercial
	ActiveIngredient string    `json:"active_ingredient"` // principio activo
	Form             string    `json:"form"`              // comprimido, jarabe, ampolla
	Strength         string    `json:"strength"`          // 500 mg
	Laboratory       string    `json:"laboratory"`
	RequiresRx       bool      `json:"requires_rx"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Validate comprueba los campos mínimos para que un registro sea indexable.
func (m Medicine) Validate() error {
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.Name) == "" {
		return ErrInvalidMedicine
	}
	if strings.TrimSpace(m.ActiveIngredient) == "" {
		return ErrInvalidMedicine
	}
	return nil
}

// SearchText compone el texto que alimenta al índice.
//
// Concatenar nombre comercial y principio activo permite que quien busca
// "paracetamol" encuentre "Panadol", que es el caso de uso central.
func (m Medicine) SearchText() string {
	partes := []string{m.Name, m.ActiveIngredient, m.Form, m.Strength, m.Laboratory}
	var limpias []string
	for _, p := range partes {
		if s := strings.TrimSpace(p); s != "" {
			limpias = append(limpias, s)
		}
	}
	return strings.Join(limpias, " ")
}

// Availability es la disponibilidad de un medicamento en una farmacia.
type Availability struct {
	MedicineID string    `json:"medicine_id"`
	PharmacyID string    `json:"pharmacy_id"`
	Pharmacy   string    `json:"pharmacy"`
	City       string    `json:"city"`
	Stock      int       `json:"stock"`
	PriceCents int64     `json:"price_cents"`
	Currency   string    `json:"currency"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// InStock indica si hay unidades disponibles.
func (a Availability) InStock() bool { return a.Stock > 0 }
