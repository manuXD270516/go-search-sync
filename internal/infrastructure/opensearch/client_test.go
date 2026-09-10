package opensearch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/manuXD270516/go-search-sync/internal/domain/catalog"
	"github.com/manuXD270516/go-search-sync/internal/domain/search"
)

// servidorFalso simula al motor de búsqueda, lo que permite probar la
// traducción de consultas y respuestas sin levantar un clúster.
func servidorFalso(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(srv.URL, 2*time.Second)
}

func TestSearchTraduceLaConsulta(t *testing.T) {
	var recibido map[string]any
	c := servidorFalso(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &recibido)
		_, _ = w.Write([]byte(`{"took":7,"hits":{"total":{"value":1},"hits":[
			{"_id":"med_1","_score":3.2,"_source":{"name":"Panadol","active_ingredient":"paracetamol"}}]}}`))
	})

	rx := false
	q, err := search.NewQuery("paracetamol", "Santa Cruz", true, &rx, 0, 20)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	res, err := c.SearchMedicines(context.Background(), q)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.Total != 1 || len(res.Hits) != 1 || res.Hits[0].ID != "med_1" {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	if res.TookMS != 7 {
		t.Errorf("took = %d", res.TookMS)
	}
	// La consulta enviada debe pedir tolerancia a errores de tipeo.
	crudo, _ := json.Marshal(recibido)
	if !strings.Contains(string(crudo), "fuzziness") {
		t.Error("la consulta debe incluir fuzziness para tolerar errores de tipeo")
	}
	if !strings.Contains(string(crudo), "requires_rx") {
		t.Error("el filtro de receta debe viajar en la consulta")
	}
}

func TestSearchPropagaErrorDelMotor(t *testing.T) {
	c := servidorFalso(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	q, _ := search.NewQuery("aspirina", "", false, nil, 0, 10)
	if _, err := c.SearchMedicines(context.Background(), q); !errors.Is(err, ErrBadResponse) {
		t.Fatalf("se esperaba ErrBadResponse, se obtuvo %v", err)
	}
}

func TestGetMedicineNoEncontrado(t *testing.T) {
	c := servidorFalso(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := c.GetMedicine(context.Background(), "med_x"); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, se obtuvo %v", err)
	}
}

func TestSuggestDeduplica(t *testing.T) {
	c := servidorFalso(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":3},"hits":[
			{"_id":"1","_source":{"name":"Paracetamol"}},
			{"_id":"2","_source":{"name":"Paracetamol"}},
			{"_id":"3","_source":{"name":"Paracetamol Forte"}}]}}`))
	})
	out, err := c.Suggest(context.Background(), "para", 10)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("se esperaban 2 sugerencias sin duplicados, hay %d: %v", len(out), out)
	}
}

func TestDisponibilidadFiltraPorStockYCiudad(t *testing.T) {
	var recibido map[string]any
	c := servidorFalso(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &recibido)
		_, _ = w.Write([]byte(`{"hits":{"hits":[
			{"_source":{"medicine_id":"med_1","pharmacy":"Farmacia Central","city":"Santa Cruz","stock":12,"price_cents":2500,"currency":"BOB"}}]}}`))
	})
	out, err := c.ByMedicine(context.Background(), "med_1", "Santa Cruz", true)
	if err != nil {
		t.Fatalf("disponibilidad: %v", err)
	}
	if len(out) != 1 || !out[0].InStock() {
		t.Fatalf("resultado inesperado: %+v", out)
	}
	crudo, _ := json.Marshal(recibido)
	if !strings.Contains(string(crudo), `"stock"`) || !strings.Contains(string(crudo), "Santa Cruz") {
		t.Error("los filtros de stock y ciudad deben viajar en la consulta")
	}
}

// Una llamada sin respuesta no puede quedar colgada indefinidamente.
func TestTimeoutObligatorio(t *testing.T) {
	c := servidorFalso(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
	})
	c.HTTP.Timeout = 20 * time.Millisecond
	q, _ := search.NewQuery("ibuprofeno", "", false, nil, 0, 10)
	if _, err := c.SearchMedicines(context.Background(), q); !errors.Is(err, ErrEngineUnavailable) {
		t.Fatalf("se esperaba ErrEngineUnavailable por timeout, se obtuvo %v", err)
	}
}
