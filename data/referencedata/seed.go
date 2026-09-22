package referencedata

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

//go:embed countries.json
var embeddedCountriesJSON []byte

//go:embed currencies.json
var embeddedCurrenciesJSON []byte

const estimatedRecordsPerCountry = 30

// SetEmbeddedJSONForTesting overrides embedded seed bytes for test verification and returns a cleanup callback.
func SetEmbeddedJSONForTesting(countriesJSON, currenciesJSON []byte) func() {
	originalCountries := embeddedCountriesJSON
	originalCurrencies := embeddedCurrenciesJSON
	embeddedCountriesJSON = countriesJSON
	embeddedCurrenciesJSON = currenciesJSON
	return func() {
		embeddedCountriesJSON = originalCountries
		embeddedCurrenciesJSON = originalCurrencies
	}
}

// LoadSeedData parses and returns the embedded seed data for countries and currencies.
func LoadSeedData() ([]Country, []Currency, error) {
	var countries []Country
	if err := json.Unmarshal(embeddedCountriesJSON, &countries); err != nil {
		return nil, nil, fmt.Errorf("failed to parse embedded countries seed data: %w", err)
	}

	var currencies []Currency
	if err := json.Unmarshal(embeddedCurrenciesJSON, &currencies); err != nil {
		return nil, nil, fmt.Errorf("failed to parse embedded currencies seed data: %w", err)
	}

	return countries, currencies, nil
}

// Seed populates the reference_data schema with normalized reference datasets.
func Seed(ctx context.Context, kernel *core.Kernel) error {
	var exists bool
	err := kernel.DB().QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM reference_data.countries LIMIT 1)").Scan(&exists)
	if err == nil && exists {
		return nil
	}

	countries, currencies, err := LoadSeedData()
	if err != nil {
		return err
	}

	return SeedWithData(ctx, kernel, countries, currencies)
}

// SeedWithData inserts custom or embedded reference data into reference_data schema tables.
func SeedWithData(ctx context.Context, kernel *core.Kernel, countries []Country, currencies []Currency) error {
	tx, err := kernel.DB().Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin reference data seeding transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, _ = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('reference_data_seed'))")

	// 1. Seed Regions & Subregions
	regionMap := make(map[string]uuid.UUID)
	subregionMap := make(map[string]uuid.UUID)

	for _, country := range countries {
		regionName := strings.TrimSpace(country.Region)
		if regionName != "" && regionMap[regionName] == uuid.Nil() {
			var regionID uuid.UUID
			err := tx.QueryRow(ctx, `
				INSERT INTO reference_data.regions (name)
				VALUES ($1)
				ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
				RETURNING id
			`, regionName).Scan(&regionID)
			if err != nil {
				return fmt.Errorf("failed to seed region %q: %w", regionName, err)
			}
			regionMap[regionName] = regionID
		}

		subregionName := strings.TrimSpace(country.Subregion)
		if subregionName != "" && subregionMap[subregionName] == uuid.Nil() {
			regionID := regionMap[regionName]
			var subregionID uuid.UUID
			var queryErr error
			if regionID != uuid.Nil() {
				queryErr = tx.QueryRow(ctx, `
					INSERT INTO reference_data.subregions (region_id, name)
					VALUES ($1, $2)
					ON CONFLICT (name) DO UPDATE SET region_id = EXCLUDED.region_id
					RETURNING id
				`, regionID, subregionName).Scan(&subregionID)
			} else {
				queryErr = tx.QueryRow(ctx, `
					INSERT INTO reference_data.subregions (name)
					VALUES ($1)
					ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
					RETURNING id
				`, subregionName).Scan(&subregionID)
			}
			if queryErr != nil {
				return fmt.Errorf("failed to seed subregion %q: %w", subregionName, queryErr)
			}
			subregionMap[subregionName] = subregionID
		}
	}

	// 2. Seed Currencies
	currencyMap := make(map[string]uuid.UUID)
	for _, currency := range currencies {
		code := strings.ToUpper(strings.TrimSpace(currency.AlphabeticCode))
		if code == "" || currencyMap[code] != uuid.Nil() {
			continue
		}
		var currencyID uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO reference_data.currencies (code, name, numeric_code, minor_unit)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, numeric_code = EXCLUDED.numeric_code, minor_unit = EXCLUDED.minor_unit
			RETURNING id
		`, code, currency.Name, currency.NumericCode, currency.MinorUnit).Scan(&currencyID)
		if err != nil {
			return fmt.Errorf("failed to seed currency %q: %w", code, err)
		}
		currencyMap[code] = currencyID
	}

	// Also insert any currencies defined in country definitions that might not be in ISO XML
	updatedCurrencies := make(map[string]bool)
	for _, country := range countries {
		for code, detail := range country.Currencies {
			code = strings.ToUpper(strings.TrimSpace(code))
			if code == "" {
				continue
			}
			if currencyMap[code] == uuid.Nil() {
				var currencyID uuid.UUID
				err := tx.QueryRow(ctx, `
					INSERT INTO reference_data.currencies (code, name, symbol)
					VALUES ($1, $2, $3)
					ON CONFLICT (code) DO UPDATE SET symbol = COALESCE(EXCLUDED.symbol, reference_data.currencies.symbol)
					RETURNING id
				`, code, detail.Name, detail.Symbol).Scan(&currencyID)
				if err != nil {
					return fmt.Errorf("failed to seed country currency %q: %w", code, err)
				}
				currencyMap[code] = currencyID
			} else if detail.Symbol != "" && !updatedCurrencies[code] {
				updatedCurrencies[code] = true
				_, _ = tx.Exec(ctx, `
					UPDATE reference_data.currencies SET symbol = $1 WHERE code = $2 AND symbol IS NULL
				`, detail.Symbol, code)
			}
		}
	}

	// 3. Seed Languages
	languageMap := make(map[string]uuid.UUID)
	for _, country := range countries {
		for code, name := range country.Languages {
			code = strings.ToLower(strings.TrimSpace(code))
			if code == "" || languageMap[code] != uuid.Nil() {
				continue
			}
			var languageID uuid.UUID
			err := tx.QueryRow(ctx, `
				INSERT INTO reference_data.languages (code, name)
				VALUES ($1, $2)
				ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
				RETURNING id
			`, code, name).Scan(&languageID)
			if err != nil {
				return fmt.Errorf("failed to seed language %q: %w", code, err)
			}
			languageMap[code] = languageID
		}
	}

	// 4. Seed Timezones
	timezoneMap := make(map[string]uuid.UUID)
	for _, country := range countries {
		for _, identifier := range country.Timezones {
			identifier = strings.TrimSpace(identifier)
			if identifier == "" || timezoneMap[identifier] != uuid.Nil() {
				continue
			}
			var timezoneID uuid.UUID
			err := tx.QueryRow(ctx, `
				INSERT INTO reference_data.timezones (identifier)
				VALUES ($1)
				ON CONFLICT (identifier) DO UPDATE SET identifier = EXCLUDED.identifier
				RETURNING id
			`, identifier).Scan(&timezoneID)
			if err != nil {
				return fmt.Errorf("failed to seed timezone %q: %w", identifier, err)
			}
			timezoneMap[identifier] = timezoneID
		}
	}

	// 5. Seed Countries
	countryCCA2Map := make(map[string]uuid.UUID)
	countryCCA3Map := make(map[string]uuid.UUID)

	for _, country := range countries {
		cca2 := strings.ToUpper(strings.TrimSpace(country.CCA2))
		cca3 := strings.ToUpper(strings.TrimSpace(country.CCA3))
		if cca2 == "" || cca3 == "" {
			continue
		}

		var regionID *uuid.UUID
		if id, ok := regionMap[country.Region]; ok && id != uuid.Nil() {
			regionID = &id
		}

		var subregionID *uuid.UUID
		if id, ok := subregionMap[country.Subregion]; ok && id != uuid.Nil() {
			subregionID = &id
		}

		var latitude, longitude *float64
		if len(country.LatLng) >= 2 {
			latitude = &country.LatLng[0]
			longitude = &country.LatLng[1]
		}

		iddSuffixes := country.IDD.Suffixes
		if iddSuffixes == nil {
			iddSuffixes = []string{}
		}
		capital := country.Capital
		if capital == nil {
			capital = []string{}
		}
		altSpellings := country.AltSpellings
		if altSpellings == nil {
			altSpellings = []string{}
		}
		tld := country.TLD
		if tld == nil {
			tld = []string{}
		}
		latlng := country.LatLng
		if latlng == nil {
			latlng = []float64{}
		}

		var countryID uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO reference_data.countries (
				cca2, cca3, ccn3, cioc, common_name, official_name,
				region_id, subregion_id, region, subregion, status,
				independent, un_member, un_regional_group, idd_root, idd_suffixes,
				capital, alt_spellings, tld, latitude, longitude, latlng,
				landlocked, area, flag
			)
			VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11,
				$12, $13, $14, $15, $16,
				$17, $18, $19, $20, $21, $22,
				$23, $24, $25
			)
			ON CONFLICT (cca2) DO UPDATE SET
				cca3 = EXCLUDED.cca3,
				ccn3 = EXCLUDED.ccn3,
				cioc = EXCLUDED.cioc,
				common_name = EXCLUDED.common_name,
				official_name = EXCLUDED.official_name,
				region_id = EXCLUDED.region_id,
				subregion_id = EXCLUDED.subregion_id,
				region = EXCLUDED.region,
				subregion = EXCLUDED.subregion,
				status = EXCLUDED.status,
				independent = EXCLUDED.independent,
				un_member = EXCLUDED.un_member,
				un_regional_group = EXCLUDED.un_regional_group,
				idd_root = EXCLUDED.idd_root,
				idd_suffixes = EXCLUDED.idd_suffixes,
				capital = EXCLUDED.capital,
				alt_spellings = EXCLUDED.alt_spellings,
				tld = EXCLUDED.tld,
				latitude = EXCLUDED.latitude,
				longitude = EXCLUDED.longitude,
				latlng = EXCLUDED.latlng,
				landlocked = EXCLUDED.landlocked,
				area = EXCLUDED.area,
				flag = EXCLUDED.flag
			RETURNING id
		`,
			cca2, cca3, country.CCN3, country.CIOC, country.Name.Common, country.Name.Official,
			regionID, subregionID, country.Region, country.Subregion, country.Status,
			country.Independent, country.UNMember, country.UNRegionalGroup, country.IDD.Root, iddSuffixes,
			capital, altSpellings, tld, latitude, longitude, latlng,
			country.Landlocked, country.Area, country.Flag,
		).Scan(&countryID)
		if err != nil {
			return fmt.Errorf("failed to seed country %q (%s): %w", country.Name.Common, cca2, err)
		}

		countryCCA2Map[cca2] = countryID
		countryCCA3Map[cca3] = countryID
	}

	// 6. Seed Detail & Junction Tables for each Country
	batch := &pgx.Batch{}
	batchLabels := make([]string, 0, len(countries)*estimatedRecordsPerCountry)

	for _, country := range countries {
		countryID := countryCCA2Map[strings.ToUpper(strings.TrimSpace(country.CCA2))]
		if countryID == uuid.Nil() {
			continue
		}

		// 6a. Country Currencies
		for code, detail := range country.Currencies {
			currencyID := currencyMap[strings.ToUpper(strings.TrimSpace(code))]
			if currencyID != uuid.Nil() {
				batch.Queue(`
					INSERT INTO reference_data.country_currencies (country_id, currency_id, symbol)
					VALUES ($1, $2, $3)
					ON CONFLICT (country_id, currency_id) DO UPDATE SET symbol = EXCLUDED.symbol
				`, countryID, currencyID, detail.Symbol)
				batchLabels = append(batchLabels, fmt.Sprintf("country_currency for %s", country.CCA2))
			}
		}

		// 6b. Country Languages
		for code := range country.Languages {
			languageID := languageMap[strings.ToLower(strings.TrimSpace(code))]
			if languageID != uuid.Nil() {
				batch.Queue(`
					INSERT INTO reference_data.country_languages (country_id, language_id)
					VALUES ($1, $2)
					ON CONFLICT (country_id, language_id) DO NOTHING
				`, countryID, languageID)
				batchLabels = append(batchLabels, fmt.Sprintf("country_language for %s", country.CCA2))
			}
		}

		// 6c. Country Timezones
		for _, timezoneIdentifier := range country.Timezones {
			timezoneID := timezoneMap[strings.TrimSpace(timezoneIdentifier)]
			if timezoneID != uuid.Nil() {
				batch.Queue(`
					INSERT INTO reference_data.country_timezones (country_id, timezone_id)
					VALUES ($1, $2)
					ON CONFLICT (country_id, timezone_id) DO NOTHING
				`, countryID, timezoneID)
				batchLabels = append(batchLabels, fmt.Sprintf("country_timezone for %s", country.CCA2))
			}
		}

		// 6d. Country Borders
		for _, borderCCA3 := range country.Borders {
			borderCountryID := countryCCA3Map[strings.ToUpper(strings.TrimSpace(borderCCA3))]
			if borderCountryID != uuid.Nil() && borderCountryID != countryID {
				batch.Queue(`
					INSERT INTO reference_data.country_borders (country_id, border_country_id)
					VALUES ($1, $2)
					ON CONFLICT (country_id, border_country_id) DO NOTHING
				`, countryID, borderCountryID)
				batchLabels = append(batchLabels, fmt.Sprintf("country_borders for %s", country.CCA2))
			}
		}

		// 6e. Country Name Translations
		for langCode, translation := range country.Translations {
			langCode = strings.ToLower(strings.TrimSpace(langCode))
			if langCode != "" {
				batch.Queue(`
					INSERT INTO reference_data.country_name_translations (country_id, language_code, common_name, official_name)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (country_id, language_code) DO UPDATE SET
						common_name = EXCLUDED.common_name,
						official_name = EXCLUDED.official_name
				`, countryID, langCode, translation.Common, translation.Official)
				batchLabels = append(batchLabels, fmt.Sprintf("country_name_translations for %s", country.CCA2))
			}
		}

		// 6f. Country Native Names
		for langCode, native := range country.Name.Native {
			langCode = strings.ToLower(strings.TrimSpace(langCode))
			if langCode != "" {
				batch.Queue(`
					INSERT INTO reference_data.country_native_names (country_id, language_code, common_name, official_name)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (country_id, language_code) DO UPDATE SET
						common_name = EXCLUDED.common_name,
						official_name = EXCLUDED.official_name
				`, countryID, langCode, native.Common, native.Official)
				batchLabels = append(batchLabels, fmt.Sprintf("country_native_names for %s", country.CCA2))
			}
		}

		// 6g. Country Demonyms
		for langCode, demonym := range country.Demonyms {
			langCode = strings.ToLower(strings.TrimSpace(langCode))
			if langCode != "" {
				batch.Queue(`
					INSERT INTO reference_data.country_demonyms (country_id, language_code, female, male)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (country_id, language_code) DO UPDATE SET
						female = EXCLUDED.female,
						male = EXCLUDED.male
				`, countryID, langCode, demonym.F, demonym.M)
				batchLabels = append(batchLabels, fmt.Sprintf("country_demonyms for %s", country.CCA2))
			}
		}

		// 6h. Country Capitals
		for _, capitalName := range country.Capital {
			capitalName = strings.TrimSpace(capitalName)
			if capitalName != "" {
				batch.Queue(`
					INSERT INTO reference_data.country_capitals (country_id, name)
					VALUES ($1, $2)
					ON CONFLICT (country_id, name) DO NOTHING
				`, countryID, capitalName)
				batchLabels = append(batchLabels, fmt.Sprintf("country_capitals for %s", country.CCA2))
			}
		}

		// 6i. Country TLDs
		for _, topLevelDomain := range country.TLD {
			topLevelDomain = strings.TrimSpace(topLevelDomain)
			if topLevelDomain != "" {
				batch.Queue(`
					INSERT INTO reference_data.country_tlds (country_id, tld)
					VALUES ($1, $2)
					ON CONFLICT (country_id, tld) DO NOTHING
				`, countryID, topLevelDomain)
				batchLabels = append(batchLabels, fmt.Sprintf("country_tlds for %s", country.CCA2))
			}
		}

		// 6j. Country Alt Spellings
		for _, spelling := range country.AltSpellings {
			spelling = strings.TrimSpace(spelling)
			if spelling != "" {
				batch.Queue(`
					INSERT INTO reference_data.country_alt_spellings (country_id, spelling)
					VALUES ($1, $2)
					ON CONFLICT (country_id, spelling) DO NOTHING
				`, countryID, spelling)
				batchLabels = append(batchLabels, fmt.Sprintf("country_alt_spellings for %s", country.CCA2))
			}
		}
	}

	if batch.Len() > 0 {
		batchResults := tx.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, execErr := batchResults.Exec(); execErr != nil {
				_ = batchResults.Close()
				return fmt.Errorf("failed to insert %s: %w", batchLabels[i], execErr)
			}
		}
		_ = batchResults.Close()
	}

	return tx.Commit(ctx)
}
