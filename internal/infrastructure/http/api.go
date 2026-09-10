// Package http expone el catálogo farmacéutico como API REST.
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/manuXD270516/go-search-sync/internal/application/usecase"
	"github.com/manuXD270516/go-search-sync/internal/domain/catalog"
	"github.com/manuXD270516/go-search-sync/internal/domain/search"
	"github.com/manuXD270516/go-search-sync/internal/infrastructure/opensearch"
)

// Problem implementa el formato de error RFC 7807.
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	Code   string `json:"code"`
}

// API agrupa las dependencias de los handlers.
type API struct {
	Catalog *usecase.Catalog
	Log     *slog.Logger
	Cache   *Cache
}

func writeProblem(w http.ResponseWriter, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Type:  "https://docs.farmasearch.dev/errors/" + code,
		Title: title, Status: status, Detail: detail, Code: code,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func mapError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, search.ErrEmptyQuery):
		writeProblem(w, http.StatusBadRequest, "empty_query", "La consulta no puede estar vacia", err.Error())
	case errors.Is(err, search.ErrInvalidPage):
		writeProblem(w, http.StatusBadRequest, "invalid_pagination", "Paginacion invalida", "")
	case errors.Is(err, catalog.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "Medicamento no encontrado", "")
	case errors.Is(err, opensearch.ErrEngineUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "search_unavailable", "Motor de busqueda no disponible", "")
	default:
		writeProblem(w, http.StatusInternalServerError, "internal_error", "Error interno", "")
	}
}

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var rx *bool
	if v := q.Get("requires_rx"); v != "" {
		b := v == "true"
		rx = &b
	}
	in := usecase.SearchInput{
		Text:        q.Get("q"),
		City:        q.Get("city"),
		OnlyInStock: q.Get("in_stock") == "true",
		RequiresRx:  rx,
		From:        atoi(q.Get("from"), 0),
		Size:        atoi(q.Get("size"), search.DefaultSize),
	}

	// Las búsquedas populares se repiten mucho; una caché breve reduce carga
	// sobre el motor sin que los resultados queden perceptiblemente viejos.
	clave := r.URL.RawQuery
	if cached, ok := a.Cache.Get(clave); ok {
		w.Header().Set("X-Cache", "hit")
		writeJSON(w, http.StatusOK, cached)
		return
	}
	res, err := a.Catalog.Search(r.Context(), in)
	if err != nil {
		mapError(w, err)
		return
	}
	a.Cache.Set(clave, res)
	w.Header().Set("X-Cache", "miss")
	writeJSON(w, http.StatusOK, res)
}

func (a *API) suggest(w http.ResponseWriter, r *http.Request) {
	out, err := a.Catalog.Suggest(r.Context(), r.URL.Query().Get("q"), atoi(r.URL.Query().Get("size"), 10))
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": out})
}

func (a *API) detail(w http.ResponseWriter, r *http.Request) {
	d, err := a.Catalog.Detail(r.Context(), r.PathValue("id"),
		r.URL.Query().Get("city"), r.URL.Query().Get("in_stock") == "true")
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func atoi(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// Routes construye el enrutador con API, documentación y health checks.
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/medicines/search", a.search)
	mux.HandleFunc("GET /v1/medicines/suggest", a.suggest)
	mux.HandleFunc("GET /v1/medicines/{id}", a.detail)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	// Readiness verifica la dependencia real: sin motor de búsqueda, el
	// servicio no puede cumplir su función aunque el proceso esté vivo.
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := contextWithTimeout(r, 2*time.Second)
		defer cancel()
		if err := a.Catalog.Health(ctx); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "not_ready", "Motor de busqueda no disponible", "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /openapi.yaml", serveOpenAPI)
	mux.HandleFunc("GET /docs", serveSwaggerUI)

	return logRequests(a.Log, mux)
}

func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		next.ServeHTTP(w, r)
		log.Info("request", "method", r.Method, "path", r.URL.Path,
			"duration_ms", time.Since(inicio).Milliseconds())
	})
}

// Cache es una caché en memoria con expiración por entrada.
type Cache struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]entrada
}

type entrada struct {
	valor  search.Result
	expira time.Time
}

// NewCache crea una caché con el tiempo de vida indicado.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, items: make(map[string]entrada)}
}

// Get devuelve un valor vigente, si existe.
func (c *Cache) Get(k string) (search.Result, bool) {
	if c == nil {
		return search.Result{}, false
	}
	c.mu.RLock()
	e, ok := c.items[k]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expira) {
		return search.Result{}, false
	}
	return e.valor, true
}

// Set guarda un valor con su vencimiento.
func (c *Cache) Set(k string, v search.Result) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[k] = entrada{valor: v, expira: time.Now().Add(c.ttl)}
}
