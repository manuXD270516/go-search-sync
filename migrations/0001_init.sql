-- Esquema de la fuente de verdad: PostgreSQL.
-- OpenSearch es una proyección de estos datos, nunca el origen.

CREATE TABLE IF NOT EXISTS medicines (
    id                TEXT        PRIMARY KEY,
    name              TEXT        NOT NULL,
    active_ingredient TEXT        NOT NULL,
    form              TEXT        NOT NULL,
    strength          TEXT,
    laboratory        TEXT,
    requires_rx       BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pharmacies (
    id         TEXT        PRIMARY KEY,
    name       TEXT        NOT NULL,
    city       TEXT        NOT NULL,
    address    TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS availability (
    id          BIGSERIAL   PRIMARY KEY,
    medicine_id TEXT        NOT NULL REFERENCES medicines (id) ON DELETE CASCADE,
    pharmacy_id TEXT        NOT NULL REFERENCES pharmacies (id) ON DELETE CASCADE,
    pharmacy    TEXT        NOT NULL,
    city        TEXT        NOT NULL,
    stock       INTEGER     NOT NULL DEFAULT 0 CHECK (stock >= 0),
    price_cents BIGINT      NOT NULL CHECK (price_cents >= 0),
    currency    CHAR(3)     NOT NULL DEFAULT 'BOB',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (medicine_id, pharmacy_id)
);

CREATE INDEX IF NOT EXISTS idx_availability_medicine ON availability (medicine_id);
CREATE INDEX IF NOT EXISTS idx_availability_city     ON availability (city, medicine_id);

-- Debezium necesita el valor anterior completo de la fila para construir
-- eventos de actualización y borrado útiles aguas abajo.
ALTER TABLE medicines    REPLICA IDENTITY FULL;
ALTER TABLE availability REPLICA IDENTITY FULL;

-- Publicación lógica que alimenta al conector de origen.
DROP PUBLICATION IF EXISTS pharmacy_pub;
CREATE PUBLICATION pharmacy_pub FOR TABLE medicines, availability;

-- Datos de ejemplo para poder probar el flujo de punta a punta.
INSERT INTO medicines (id, name, active_ingredient, form, strength, laboratory, requires_rx) VALUES
    ('med_001', 'Panadol',      'paracetamol',              'comprimido', '500 mg', 'GSK',       FALSE),
    ('med_002', 'Ibupirac',     'ibuprofeno',               'comprimido', '400 mg', 'Pfizer',    FALSE),
    ('med_003', 'Amoxidal',     'amoxicilina',              'capsula',    '500 mg', 'Roemmers',  TRUE),
    ('med_004', 'Aspirina',     'acido acetilsalicilico',   'comprimido', '100 mg', 'Bayer',     FALSE),
    ('med_005', 'Losartan LCH', 'losartan',                 'comprimido', '50 mg',  'Chile Lab', TRUE)
ON CONFLICT (id) DO NOTHING;

INSERT INTO pharmacies (id, name, city, address) VALUES
    ('ph_001', 'Farmacia Central',   'Santa Cruz', 'Av. Monseñor Rivero 100'),
    ('ph_002', 'Farmacia del Pueblo','Santa Cruz', 'Calle Libertad 250'),
    ('ph_003', 'Farmacorp',          'La Paz',     'Av. Arce 500')
ON CONFLICT (id) DO NOTHING;

INSERT INTO availability (medicine_id, pharmacy_id, pharmacy, city, stock, price_cents) VALUES
    ('med_001', 'ph_001', 'Farmacia Central',    'Santa Cruz', 42,  2500),
    ('med_001', 'ph_003', 'Farmacorp',           'La Paz',     8,   2800),
    ('med_002', 'ph_001', 'Farmacia Central',    'Santa Cruz', 0,   3200),
    ('med_002', 'ph_002', 'Farmacia del Pueblo', 'Santa Cruz', 15,  3100),
    ('med_003', 'ph_002', 'Farmacia del Pueblo', 'Santa Cruz', 5,   8900),
    ('med_004', 'ph_001', 'Farmacia Central',    'Santa Cruz', 60,  1200)
ON CONFLICT (medicine_id, pharmacy_id) DO NOTHING;
