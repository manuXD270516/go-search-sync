// Package opensearch implementa el motor de búsqueda sobre OpenSearch.
//
// Se habla el protocolo HTTP directamente con la biblioteca estándar: evita
// arrastrar un SDK pesado para el puñado de operaciones que el servicio usa, y
// mantiene el binario y la superficie de dependencias al mínimo.
package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/manuXD270516/go-search-sync/internal/domain/catalog"
	"github.com/manuXD270516/go-search-sync/internal/domain/search"
)

// Errores del adaptador.
var (
	ErrEngineUnavailable = errors.New("search engine unavailable")
	ErrBadResponse       = errors.New("unexpected response from search engine")
)

// Client habla con un clúster OpenSearch/Elasticsearch.
type Client struct {
	BaseURL           string
	HTTP              *http.Client
	MedicinesIndex    string
	AvailabilityIndex string
}

// New construye el cliente con un timeout obligatorio: ninguna llamada
// saliente queda sin límite de tiempo.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		BaseURL:           strings.TrimRight(baseURL, "/"),
		HTTP:              &http.Client{Timeout: timeout},
		MedicinesIndex:    "medicines",
		AvailabilityIndex: "availability",
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var buf *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("serializando consulta: %w", err)
		}
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, buf)
	if err != nil {
		return fmt.Errorf("construyendo request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEngineUnavailable, err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return catalog.ErrNotFound
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrBadResponse, res.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("%w: %v", ErrBadResponse, err)
	}
	return nil
}

// respuestaBusqueda refleja la forma de la respuesta del motor.
type respuestaBusqueda struct {
	Took int64 `json:"took"`
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string         `json:"_id"`
			Score  float64        `json:"_score"`
			Source map[string]any `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// SearchMedicines traduce la consulta de dominio al DSL del motor.
//
// La estrategia combina tres cosas: multi_match con tolerancia a errores de
// tipeo (fuzziness), un impulso al nombre comercial sobre el resto de campos,
// y filtros que no afectan el puntaje pero acotan el conjunto.
func (c *Client) SearchMedicines(ctx context.Context, q search.Query) (search.Result, error) {
	must := []map[string]any{{
		"multi_match": map[string]any{
			"query":     q.Text,
			"fields":    []string{"name^3", "active_ingredient^2", "laboratory", "search_text"},
			"fuzziness": "AUTO",
			"operator":  "and",
		},
	}}
	filtros := []map[string]any{}
	if q.RequiresRx != nil {
		filtros = append(filtros, map[string]any{"term": map[string]any{"requires_rx": *q.RequiresRx}})
	}

	cuerpo := map[string]any{
		"from": q.From,
		"size": q.Size,
		"query": map[string]any{
			"bool": map[string]any{"must": must, "filter": filtros},
		},
		// Se piden solo los campos necesarios: menos bytes por la red y
		// respuestas más rápidas.
		"_source": []string{"id", "name", "active_ingredient", "form", "strength", "laboratory", "requires_rx"},
	}

	var raw respuestaBusqueda
	inicio := time.Now()
	if err := c.do(ctx, http.MethodPost, "/"+c.MedicinesIndex+"/_search", cuerpo, &raw); err != nil {
		return search.Result{}, err
	}
	hits := make([]search.Hit, 0, len(raw.Hits.Hits))
	for _, h := range raw.Hits.Hits {
		hits = append(hits, search.Hit{ID: h.ID, Score: h.Score, Source: h.Source})
	}
	took := raw.Took
	if took == 0 {
		took = time.Since(inicio).Milliseconds()
	}
	return search.Result{Total: raw.Hits.Total.Value, Hits: hits, TookMS: took, Query: q}, nil
}

// Suggest devuelve sugerencias por prefijo para autocompletado.
func (c *Client) Suggest(ctx context.Context, prefix string, size int) ([]string, error) {
	cuerpo := map[string]any{
		"size": size,
		"query": map[string]any{
			"match_phrase_prefix": map[string]any{"name": map[string]any{"query": prefix}},
		},
		"_source": []string{"name"},
	}
	var raw respuestaBusqueda
	if err := c.do(ctx, http.MethodPost, "/"+c.MedicinesIndex+"/_search", cuerpo, &raw); err != nil {
		return nil, err
	}
	vistos := make(map[string]struct{}, len(raw.Hits.Hits))
	out := make([]string, 0, len(raw.Hits.Hits))
	for _, h := range raw.Hits.Hits {
		if n, ok := h.Source["name"].(string); ok {
			if _, dup := vistos[n]; !dup {
				vistos[n] = struct{}{}
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// GetMedicine recupera un medicamento por su identificador.
func (c *Client) GetMedicine(ctx context.Context, id string) (catalog.Medicine, error) {
	var raw struct {
		Source catalog.Medicine `json:"_source"`
		Found  bool             `json:"found"`
	}
	if err := c.do(ctx, http.MethodGet, "/"+c.MedicinesIndex+"/_doc/"+url.PathEscape(id), nil, &raw); err != nil {
		return catalog.Medicine{}, err
	}
	if !raw.Found {
		return catalog.Medicine{}, catalog.ErrNotFound
	}
	return raw.Source, nil
}

// ByMedicine devuelve la disponibilidad de un medicamento, opcionalmente
// filtrada por ciudad y por existencia de stock.
func (c *Client) ByMedicine(ctx context.Context, medicineID, city string, onlyInStock bool) ([]catalog.Availability, error) {
	filtros := []map[string]any{{"term": map[string]any{"medicine_id": medicineID}}}
	if city != "" {
		filtros = append(filtros, map[string]any{"term": map[string]any{"city": city}})
	}
	if onlyInStock {
		filtros = append(filtros, map[string]any{"range": map[string]any{"stock": map[string]any{"gt": 0}}})
	}
	cuerpo := map[string]any{
		"size":  search.MaxSize,
		"query": map[string]any{"bool": map[string]any{"filter": filtros}},
		"sort":  []map[string]any{{"price_cents": map[string]any{"order": "asc"}}},
	}
	var raw struct {
		Hits struct {
			Hits []struct {
				Source catalog.Availability `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := c.do(ctx, http.MethodPost, "/"+c.AvailabilityIndex+"/_search", cuerpo, &raw); err != nil {
		return nil, err
	}
	out := make([]catalog.Availability, 0, len(raw.Hits.Hits))
	for _, h := range raw.Hits.Hits {
		out = append(out, h.Source)
	}
	return out, nil
}

// Health verifica que el motor responda.
func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/_cluster/health", nil, nil)
}
