package data

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// DataDatabaseMigrationVersion is the schema version for the data subsystem.
const DataDatabaseMigrationVersion = 100

// CDCNotificationChannel is the PostgreSQL LISTEN/NOTIFY channel for CDC events.
const CDCNotificationChannel = "cdc"

// DataDatabaseMigration defines the database schema for data and reference_data.
var DataDatabaseMigration = core.DatabaseMigration{
	Version:     DataDatabaseMigrationVersion,
	Description: "Initialize data and reference_data schemas, config table, CDC triggers, and relational tables",
	UpSQL: `
CREATE SCHEMA IF NOT EXISTS data;

CREATE TABLE IF NOT EXISTS data.config (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    key VARCHAR(128) NOT NULL UNIQUE,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE OR REPLACE FUNCTION data.notify_cdc() RETURNS trigger AS $$
DECLARE
    payload JSONB;
    rec_record JSONB;
    old_rec_record JSONB;
BEGIN
    IF (TG_OP = 'DELETE') THEN
        rec_record := to_jsonb(OLD);
        old_rec_record := NULL;
    ELSIF (TG_OP = 'UPDATE') THEN
        rec_record := to_jsonb(NEW);
        old_rec_record := to_jsonb(OLD);
    ELSE
        rec_record := to_jsonb(NEW);
        old_rec_record := NULL;
    END IF;
    
    payload := json_build_object(
        'schema', TG_TABLE_SCHEMA,
        'table', TG_TABLE_NAME,
        'event', TG_OP,
        'commit_timestamp', clock_timestamp(),
        'record', rec_record,
        'old_record', old_rec_record
    );
    
    IF octet_length(payload::text) > 7800 THEN
        payload := json_build_object(
            'schema', TG_TABLE_SCHEMA,
            'table', TG_TABLE_NAME,
            'event', TG_OP,
            'commit_timestamp', clock_timestamp(),
            'record', jsonb_build_object('id', COALESCE(rec_record->>'id', rec_record->>'uuid')),
            'old_record', CASE WHEN old_rec_record IS NOT NULL THEN jsonb_build_object('id', COALESCE(old_rec_record->>'id', old_rec_record->>'uuid')) ELSE NULL END,
            'truncated', true
        );
    END IF;
    
    PERFORM pg_notify('cdc', payload::text);
    IF (TG_OP = 'DELETE') THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE SCHEMA IF NOT EXISTS reference_data;

CREATE TABLE IF NOT EXISTS reference_data.regions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS reference_data.subregions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    region_id UUID REFERENCES reference_data.regions(id) ON DELETE SET NULL,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_reference_data_subregions_region_id ON reference_data.subregions (region_id);

CREATE TABLE IF NOT EXISTS reference_data.countries (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    cca2 VARCHAR(2) NOT NULL UNIQUE,
    cca3 VARCHAR(3) NOT NULL UNIQUE,
    ccn3 VARCHAR(3),
    cioc VARCHAR(3),
    common_name TEXT NOT NULL,
    official_name TEXT NOT NULL,
    region_id UUID REFERENCES reference_data.regions(id) ON DELETE SET NULL,
    subregion_id UUID REFERENCES reference_data.subregions(id) ON DELETE SET NULL,
    region TEXT,
    subregion TEXT,
    status TEXT,
    independent BOOLEAN,
    un_member BOOLEAN,
    un_regional_group TEXT,
    idd_root TEXT,
    idd_suffixes TEXT[] NOT NULL DEFAULT '{}',
    capital TEXT[] NOT NULL DEFAULT '{}',
    alt_spellings TEXT[] NOT NULL DEFAULT '{}',
    tld TEXT[] NOT NULL DEFAULT '{}',
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    latlng DOUBLE PRECISION[] NOT NULL DEFAULT '{}',
    landlocked BOOLEAN,
    area DOUBLE PRECISION,
    flag TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_reference_data_countries_cca2 ON reference_data.countries (cca2);
CREATE INDEX IF NOT EXISTS idx_reference_data_countries_cca3 ON reference_data.countries (cca3);
CREATE INDEX IF NOT EXISTS idx_reference_data_countries_region_id ON reference_data.countries (region_id);
CREATE INDEX IF NOT EXISTS idx_reference_data_countries_subregion_id ON reference_data.countries (subregion_id);

CREATE TABLE IF NOT EXISTS reference_data.currencies (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    code VARCHAR(3) NOT NULL UNIQUE,
    name TEXT NOT NULL,
    numeric_code INT,
    minor_unit INT,
    symbol TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_reference_data_currencies_code ON reference_data.currencies (code);

CREATE TABLE IF NOT EXISTS reference_data.languages (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    code VARCHAR(8) NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_reference_data_languages_code ON reference_data.languages (code);

CREATE TABLE IF NOT EXISTS reference_data.timezones (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    identifier TEXT NOT NULL UNIQUE,
    comments TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_reference_data_timezones_identifier ON reference_data.timezones (identifier);

CREATE TABLE IF NOT EXISTS reference_data.country_currencies (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    currency_id UUID NOT NULL REFERENCES reference_data.currencies(id) ON DELETE CASCADE,
    symbol TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, currency_id)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_currencies_country ON reference_data.country_currencies (country_id);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_currencies_currency ON reference_data.country_currencies (currency_id);

CREATE TABLE IF NOT EXISTS reference_data.country_languages (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    language_id UUID NOT NULL REFERENCES reference_data.languages(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, language_id)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_languages_country ON reference_data.country_languages (country_id);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_languages_language ON reference_data.country_languages (language_id);

CREATE TABLE IF NOT EXISTS reference_data.country_timezones (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    timezone_id UUID NOT NULL REFERENCES reference_data.timezones(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, timezone_id)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_timezones_country ON reference_data.country_timezones (country_id);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_timezones_timezone ON reference_data.country_timezones (timezone_id);

CREATE TABLE IF NOT EXISTS reference_data.country_borders (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    border_country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, border_country_id)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_borders_country ON reference_data.country_borders (country_id);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_borders_border ON reference_data.country_borders (border_country_id);

CREATE TABLE IF NOT EXISTS reference_data.country_name_translations (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    language_code VARCHAR(8) NOT NULL,
    common_name TEXT NOT NULL,
    official_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, language_code)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_translations_country ON reference_data.country_name_translations (country_id);

CREATE TABLE IF NOT EXISTS reference_data.country_native_names (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    language_code VARCHAR(8) NOT NULL,
    common_name TEXT NOT NULL,
    official_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, language_code)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_native_country ON reference_data.country_native_names (country_id);

CREATE TABLE IF NOT EXISTS reference_data.country_demonyms (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    language_code VARCHAR(8) NOT NULL,
    female TEXT NOT NULL,
    male TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, language_code)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_demonyms_country ON reference_data.country_demonyms (country_id);

CREATE TABLE IF NOT EXISTS reference_data.country_capitals (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, name)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_capitals_country ON reference_data.country_capitals (country_id);

CREATE TABLE IF NOT EXISTS reference_data.country_tlds (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    tld VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, tld)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_tlds_country ON reference_data.country_tlds (country_id);

CREATE TABLE IF NOT EXISTS reference_data.country_alt_spellings (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    country_id UUID NOT NULL REFERENCES reference_data.countries(id) ON DELETE CASCADE,
    spelling TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (country_id, spelling)
);
CREATE INDEX IF NOT EXISTS idx_reference_data_country_alt_spellings_country ON reference_data.country_alt_spellings (country_id);

ALTER TABLE reference_data.regions ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.regions FOR SELECT USING (true);

ALTER TABLE reference_data.subregions ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.subregions FOR SELECT USING (true);

ALTER TABLE reference_data.countries ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.countries FOR SELECT USING (true);

ALTER TABLE reference_data.currencies ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.currencies FOR SELECT USING (true);

ALTER TABLE reference_data.languages ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.languages FOR SELECT USING (true);

ALTER TABLE reference_data.timezones ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.timezones FOR SELECT USING (true);

ALTER TABLE reference_data.country_currencies ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_currencies FOR SELECT USING (true);

ALTER TABLE reference_data.country_languages ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_languages FOR SELECT USING (true);

ALTER TABLE reference_data.country_timezones ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_timezones FOR SELECT USING (true);

ALTER TABLE reference_data.country_borders ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_borders FOR SELECT USING (true);

ALTER TABLE reference_data.country_name_translations ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_name_translations FOR SELECT USING (true);

ALTER TABLE reference_data.country_native_names ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_native_names FOR SELECT USING (true);

ALTER TABLE reference_data.country_demonyms ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_demonyms FOR SELECT USING (true);

ALTER TABLE reference_data.country_capitals ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_capitals FOR SELECT USING (true);

ALTER TABLE reference_data.country_tlds ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_tlds FOR SELECT USING (true);

ALTER TABLE reference_data.country_alt_spellings ENABLE ROW LEVEL SECURITY;
CREATE POLICY select_public ON reference_data.country_alt_spellings FOR SELECT USING (true);
`,
	DownSQL: `
DROP TABLE IF EXISTS reference_data.country_alt_spellings CASCADE;
DROP TABLE IF EXISTS reference_data.country_tlds CASCADE;
DROP TABLE IF EXISTS reference_data.country_capitals CASCADE;
DROP TABLE IF EXISTS reference_data.country_demonyms CASCADE;
DROP TABLE IF EXISTS reference_data.country_native_names CASCADE;
DROP TABLE IF EXISTS reference_data.country_name_translations CASCADE;
DROP TABLE IF EXISTS reference_data.country_borders CASCADE;
DROP TABLE IF EXISTS reference_data.country_timezones CASCADE;
DROP TABLE IF EXISTS reference_data.country_languages CASCADE;
DROP TABLE IF EXISTS reference_data.country_currencies CASCADE;
DROP TABLE IF EXISTS reference_data.timezones CASCADE;
DROP TABLE IF EXISTS reference_data.languages CASCADE;
DROP TABLE IF EXISTS reference_data.currencies CASCADE;
DROP TABLE IF EXISTS reference_data.countries CASCADE;
DROP TABLE IF EXISTS reference_data.subregions CASCADE;
DROP TABLE IF EXISTS reference_data.regions CASCADE;
DROP SCHEMA IF EXISTS reference_data CASCADE;
DROP FUNCTION IF EXISTS data.notify_cdc() CASCADE;
DROP TABLE IF EXISTS data.config CASCADE;
DROP SCHEMA IF EXISTS data CASCADE;
`,
}

// Migrations contains the DDL migrations for data and reference_data.
var Migrations = []core.DatabaseMigration{
	DataDatabaseMigration,
}
