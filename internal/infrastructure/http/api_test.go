package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/manuXD270516/go-search-sync/internal/application/usecase"
	"github.com/manuXD270516/go-search-sync/internal/domain/catalog"
	"github.com/manuXD270516/go-search-sync/internal/domain/search"
)

// motorFalso implementa los puertos del motor de búsqueda, lo que permite
// probar la API sin levantar un clúster.
type motorFalso struct {
	busquedas int
	sano      bool
	falla     error
}

func (m *motorFalso) SearchMedicines(_ context.Context, q search.Query) (search.Result, error) {
	if m.falla != nil {
		return search.Result{}, m.falla
	}
	m.busquedas++
	return search.Result{Total: 1, TookMS: 3, Query: q, Hits: []search.Hit{
		{ID: "med_1", Score: 2.5, Source: map[string]any{"name": "Panadol"}},
	}}, nil
}

func (m *motorFalso) Suggest(context.Context, string, int) ([]string, error) {
	return []string{"Paracetamol", "Paracetamol Forte"}, nil
}

func (m *motorFalso) GetMedicine(_ context.Context, id string) (catalog.Medicine, error) {
	if id != "med_1" {
		return catalog.Medicine{}, catalog.ErrNotFound
	}
	return catalog.Medicine{ID: id, Name: "Panadol", ActiveIngredient: "paracetamol"}, nil
}

func (m *motorFalso) Health(context.Context) error {
	if !m.sano {
		return catalog.ErrNotFound
	}
	return nil
}

func (m *motorFalso) ByMedicine(context.Context, string, string, bool) ([]catalog.Availability, error) {
	return []catalog.Availability{{MedicineID: "med_1", Pharmacy: "Farmacia Central", Stock: 5}}, nil
}

func nuevoServidor(t *testing.T, motor *motorFalso, ttl time.Duration) *httptest.Server {
	t.Helper()
	api := &API{
		Catalog: &usecase.Catalog{Engine: motor, Availability: motor},
		Log:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		Cache:   NewCache(ttl),
	}
	srv := httptest.NewServer(api.Routes())
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, path string) (*http.Response, map[string]any) {
	t.Helper()
	res, err := srv.Client().Get(srv.URL + path)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return res, out
}

func TestBusqueda(t *testing.T) {
	srv := nuevoServidor(t, &motorFalso{sano: true}, time.Minute)
	res, body := get(t, srv, "/v1/medicines/search?q=paracetamol&city=Santa+Cruz")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if body["total"].(float64) != 1 {
		t.Fatalf("total = %v", body["total"])
	}
}

func TestConsultaVacia(t *testing.T) {
	srv := nuevoServidor(t, &motorFalso{sano: true}, time.Minute)
	res, body := get(t, srv, "/v1/medicines/search?q=")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, se esperaba 400", res.StatusCode)
	}
	if body["code"] != "empty_query" {
		t.Errorf("code = %v", body["code"])
	}
}

// La caché debe evitar que consultas repetidas golpeen el motor.
func TestCacheEvitaConsultasRepetidas(t *testing.T) {
	motor := &motorFalso{sano: true}
	srv := nuevoServidor(t, motor, time.Minute)

	res1, _ := get(t, srv, "/v1/medicines/search?q=ibuprofeno")
	if res1.Header.Get("X-Cache") != "miss" {
		t.Errorf("la primera consulta debe ser miss, fue %s", res1.Header.Get("X-Cache"))
	}
	res2, _ := get(t, srv, "/v1/medicines/search?q=ibuprofeno")
	if res2.Header.Get("X-Cache") != "hit" {
		t.Errorf("la segunda consulta debe salir de cache, fue %s", res2.Header.Get("X-Cache"))
	}
	if motor.busquedas != 1 {
		t.Fatalf("el motor debio consultarse una sola vez, se consulto %d veces", motor.busquedas)
	}
}

func TestCacheExpira(t *testing.T) {
	motor := &motorFalso{sano: true}
	srv := nuevoServidor(t, motor, 20*time.Millisecond)
	get(t, srv, "/v1/medicines/search?q=amoxicilina")
	time.Sleep(40 * time.Millisecond)
	res, _ := get(t, srv, "/v1/medicines/search?q=amoxicilina")
	if res.Header.Get("X-Cache") != "miss" {
		t.Error("tras expirar el TTL la consulta debe volver al motor")
	}
	if motor.busquedas != 2 {
		t.Fatalf("busquedas = %d, se esperaban 2", motor.busquedas)
	}
}

func TestFichaConDisponibilidad(t *testing.T) {
	srv := nuevoServidor(t, &motorFalso{sano: true}, time.Minute)
	res, body := get(t, srv, "/v1/medicines/med_1?city=Santa+Cruz&in_stock=true")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if _, ok := body["availability"].([]any); !ok {
		t.Fatal("la ficha debe incluir la disponibilidad")
	}
}

func TestFichaInexistente(t *testing.T) {
	srv := nuevoServidor(t, &motorFalso{sano: true}, time.Minute)
	res, body := get(t, srv, "/v1/medicines/med_999")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, se esperaba 404", res.StatusCode)
	}
	if body["code"] != "not_found" {
		t.Errorf("code = %v", body["code"])
	}
}

func TestSugerenciasPrefijoCorto(t *testing.T) {
	srv := nuevoServidor(t, &motorFalso{sano: true}, time.Minute)
	res, _ := get(t, srv, "/v1/medicines/suggest?q=p")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("un prefijo de 1 caracter debe rechazarse, status = %d", res.StatusCode)
	}
}

// Readiness debe distinguirse de liveness: sin motor, el servicio no está listo
// aunque el proceso siga vivo.
func TestReadinessReflejaElMotor(t *testing.T) {
	srv := nuevoServidor(t, &motorFalso{sano: false}, time.Minute)
	res, _ := get(t, srv, "/readyz")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readyz = %d, se esperaba 503 con el motor caido", res.StatusCode)
	}
	vivo, _ := get(t, srv, "/healthz")
	if vivo.StatusCode != http.StatusOK {
		t.Fatalf("healthz = %d, el proceso sigue vivo", vivo.StatusCode)
	}
}

func TestDocumentacionDisponible(t *testing.T) {
	srv := nuevoServidor(t, &motorFalso{sano: true}, time.Minute)
	for _, ruta := range []string{"/openapi.yaml", "/docs"} {
		res, err := srv.Client().Get(srv.URL + ruta)
		if err != nil {
			t.Fatalf("get %s: %v", ruta, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("%s devolvio %d", ruta, res.StatusCode)
		}
	}
}
