# ARCHITECTURE — go-search-sync

## Capas

```
       cmd/api  (composición y arranque)
            │
  internal/infrastructure  (opensearch, http)
            │  implementa los puertos
  internal/application  (casos de uso; dependen de PUERTOS)
            │  usa
  internal/domain  (catálogo, consulta) ← sin dependencias externas
```

El dominio no sabe que existe OpenSearch. `SearchEngine` es una interfaz; cambiar de motor es escribir otro adaptador.

## Decisiones (ADR resumidas)

**ADR-001 — CDC en lugar de escritura dual.** La aplicación escribe solo en PostgreSQL. Debezium lee el WAL y publica los cambios. Esto evita la inconsistencia clásica de escribir en dos sistemas sin transacción distribuida, y permite reconstruir el índice completo re-consumiendo el tópico.

**ADR-002 — PostgreSQL como fuente de verdad.** El índice es descartable: si se corrompe, se borra y se reconstruye desde el snapshot inicial del conector. Ninguna consulta de escritura toca OpenSearch.

**ADR-003 — Plantillas de índice explícitas.** El mapeo dinámico de OpenSearch infiere tipos y suele elegir `text` donde hace falta `keyword`, lo que rompe filtros y ordenamientos de forma silenciosa. Las plantillas se aplican antes de que el sink cree el índice.

**ADR-004 — Sinónimos en el analizador.** La equivalencia entre nombre comercial y principio activo se resuelve en tiempo de indexación y consulta, no en la aplicación: es donde el motor es bueno y donde el mantenimiento es más simple.

**ADR-005 — Caché breve en la API.** Las búsquedas populares se repiten mucho. Un TTL corto (30 s por defecto) recorta la carga sobre el motor sin que el usuario perciba datos viejos, dado que el pipeline ya tiene una latencia de segundos.

**ADR-006 — Solo biblioteca estándar.** El servicio habla HTTP con OpenSearch directamente. Para el puñado de operaciones que usa, un SDK completo agregaría superficie de dependencias sin beneficio: el binario queda pequeño y las actualizaciones de seguridad son menos.

**ADR-007 — Readiness distinto de liveness.** `/healthz` dice si el proceso vive; `/readyz` consulta al motor. Así el orquestador no reinicia un pod sano que solo espera a su dependencia, pero tampoco le manda tráfico que no puede atender.

**ADR-008 — Tolerancia a errores en el sink.** `errors.tolerance=all` con DLQ: un documento malformado no detiene la replicación de todo el flujo, y queda registrado para inspección.

## Rendimiento

- Se solicitan solo los campos necesarios (`_source` acotado): menos bytes por la red.
- Paginación con límites duros (`size` máximo 100, `from` máximo 10000) para acotar el costo por consulta.
- `batch.size` y `linger.ms` en el sink agrupan escrituras al índice en lugar de una por evento.
- Caché con expiración para las consultas más repetidas.
- `refresh_interval` de 5 s en el índice: prioriza el rendimiento de escritura sobre la visibilidad inmediata, aceptable para este caso de uso.
