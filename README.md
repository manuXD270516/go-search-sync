# go-search-sync

Búsqueda de medicamentos y disponibilidad en farmacias, con los datos sincronizados desde PostgreSQL hacia OpenSearch mediante Kafka Connect.

## El problema que resuelve

Buscar un medicamento tiene exigencias que una base relacional no cubre bien: la gente escribe mal los nombres ("amoxilina"), busca por principio activo cuando el producto se vende con nombre comercial ("paracetamol" debería encontrar "Panadol"), y necesita filtrar por ciudad y stock al mismo tiempo que ordena por relevancia. Un `LIKE '%texto%'` no hace nada de eso y además escala mal.

La solución no es reemplazar la base, sino proyectarla: PostgreSQL sigue siendo la fuente de verdad y OpenSearch es una vista optimizada para consulta, mantenida al día automáticamente.

## Arquitectura

```
┌────────────┐   CDC    ┌───────┐   Kafka Connect   ┌────────────┐    HTTP    ┌─────────┐
│ PostgreSQL │─────────▶│ Kafka │──────────────────▶│ OpenSearch │◀───────────│ API Go  │
│  (verdad)  │ Debezium │       │   sink + DLQ      │  (índice)  │            │ + Swagger│
└────────────┘          └───────┘                   └────────────┘            └─────────┘
```

**Por qué CDC y no escritura dual:** si la aplicación escribiera en la base y en el índice, un fallo entre ambas escrituras dejaría los datos divergentes sin forma automática de detectarlo. Con captura de cambios, el índice se deriva del log de transacciones: si el consumidor se cae, retoma desde su offset y converge solo.

## Características

- Búsqueda con tolerancia a errores de tipeo y sinónimos comercial ↔ principio activo
- Autocompletado por prefijo
- Filtros por ciudad, stock y necesidad de receta
- Caché en memoria con expiración para consultas repetidas
- Readiness que verifica la dependencia real, separado de liveness
- Errores RFC 7807 y documentación OpenAPI servida por el propio binario
- Reintentos y cola de mensajes fallidos (DLQ) en el conector

## Stack

Go 1.22 (biblioteca estándar) · PostgreSQL 16 · Kafka · Kafka Connect (Debezium) · OpenSearch 2 · Docker

## Arranque

```bash
make demo    # levanta todo, aplica plantillas y registra conectores
```

Luego:

```bash
curl 'localhost:8082/v1/medicines/search?q=paracetamol&city=Santa+Cruz'
curl 'localhost:8082/v1/medicines/suggest?q=para'
open http://localhost:8082/docs
```

## Documentación

- [`DEPLOYMENT.md`](DEPLOYMENT.md) — despliegue con contenedores y operación del pipeline
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — decisiones de diseño
- `connect/` — configuraciones de los conectores y plantillas de índice
