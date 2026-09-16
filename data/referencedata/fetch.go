// Package referencedata provides types, seed data, and fetchers for global reference data.
package referencedata

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCountriesURL  = "https://raw.githubusercontent.com/mledoze/countries/master/countries.json"
	defaultTimezonesURL  = "https://raw.githubusercontent.com/eggert/tz/main/zone.tab"
	defaultCurrenciesURL = "https://www.six-group.com/dam/download/financial-information/data-center/iso-currrency/lists/list-one.xml"

	minZoneTabParts            = 3
	minZoneTabPartsWithComment = 4
	directoryPermission        = 0755
	filePermission             = 0644
	maxResponseBodyBytes       = 15 * 1024 * 1024
)

// Fetcher encapsulates HTTP data fetching and parsing for reference data.
type Fetcher struct {
	httpClient    *http.Client
	countriesURL  string
	timezonesURL  string
	currenciesURL string
}

// NewFetcher creates a new reference data fetcher.
func NewFetcher(httpClient *http.Client) *Fetcher {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Fetcher{
		httpClient:    httpClient,
		countriesURL:  defaultCountriesURL,
		timezonesURL:  defaultTimezonesURL,
		currenciesURL: defaultCurrenciesURL,
	}
}

// SetURLs overrides the default fetch URLs (useful for unit testing).
func (fetcher *Fetcher) SetURLs(countriesURL, timezonesURL, currenciesURL string) {
	if countriesURL != "" {
		fetcher.countriesURL = countriesURL
	}
	if timezonesURL != "" {
		fetcher.timezonesURL = timezonesURL
	}
	if currenciesURL != "" {
		fetcher.currenciesURL = currenciesURL
	}
}

// TimezoneEntry holds a parsed timezone entry from zone.tab.
type TimezoneEntry struct {
	Country    string
	Identifier string
	Comments   string
}

// ParseZoneTab parses raw zone.tab string into a list of timezone entries.
func ParseZoneTab(zoneTabContent string) []TimezoneEntry {
	lines := strings.Split(zoneTabContent, "\n")
	var entries []TimezoneEntry
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= minZoneTabParts {
			timezoneEntry := TimezoneEntry{
				Country:    parts[0],
				Identifier: parts[2],
			}
			if len(parts) >= minZoneTabPartsWithComment {
				timezoneEntry.Comments = parts[3]
			}
			entries = append(entries, timezoneEntry)
		}
	}
	return entries
}

// FetchCountries fetches mledoze/countries JSON and eggert/tz zone.tab and combines them.
func (fetcher *Fetcher) FetchCountries(ctx context.Context) ([]Country, error) {
	countriesRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, fetcher.countriesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create countries request: %w", err)
	}
	countriesResponse, err := fetcher.httpClient.Do(countriesRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch countries JSON: %w", err)
	}
	defer func() { _ = countriesResponse.Body.Close() }()

	if countriesResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("countries endpoint returned status %d", countriesResponse.StatusCode)
	}

	countriesBytes, err := io.ReadAll(io.LimitReader(countriesResponse.Body, maxResponseBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read countries response body: %w", err)
	}

	var rawCountries []Country
	if unmarshalErr := json.Unmarshal(countriesBytes, &rawCountries); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse countries JSON: %w", unmarshalErr)
	}

	timezonesRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, fetcher.timezonesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create timezones request: %w", err)
	}
	timezonesResponse, err := fetcher.httpClient.Do(timezonesRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch zone.tab: %w", err)
	}
	defer func() { _ = timezonesResponse.Body.Close() }()

	if timezonesResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("timezones endpoint returned status %d", timezonesResponse.StatusCode)
	}

	timezonesBytes, err := io.ReadAll(io.LimitReader(timezonesResponse.Body, maxResponseBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read timezones response body: %w", err)
	}

	timezoneEntries := ParseZoneTab(string(timezonesBytes))
	timezoneMap := make(map[string][]string)
	for _, entry := range timezoneEntries {
		timezoneMap[entry.Country] = append(timezoneMap[entry.Country], entry.Identifier)
	}

	countries := make([]Country, len(rawCountries))
	for countryIndex, country := range rawCountries {
		updatedCountry := country
		updatedCountry.Timezones = timezoneMap[country.CCA2]
		if updatedCountry.Timezones == nil {
			updatedCountry.Timezones = []string{}
		}
		countries[countryIndex] = updatedCountry
	}

	return countries, nil
}

type iso4217RootXML struct {
	XMLName xml.Name `xml:"ISO_4217"`
	CcyTbl  struct {
		CcyNtry []ccyNtryXML `xml:"CcyNtry"`
	} `xml:"CcyTbl"`
}

type ccyNtryXML struct {
	CtryNm     string `xml:"CtryNm"`
	CcyNm      string `xml:"CcyNm"`
	Ccy        string `xml:"Ccy"`
	CcyNbr     string `xml:"CcyNbr"`
	CcyMnrUnts string `xml:"CcyMnrUnts"`
}

// ParseCurrenciesXML parses raw ISO-4217 XML into a list of Currency structs.
func ParseCurrenciesXML(xmlContent []byte) ([]Currency, error) {
	var rootXML iso4217RootXML
	if err := xml.Unmarshal(xmlContent, &rootXML); err != nil {
		return nil, fmt.Errorf("failed to parse ISO-4217 XML: %w", err)
	}

	var currencies []Currency
	for _, item := range rootXML.CcyTbl.CcyNtry {
		currency := Currency{
			CountryName:    item.CtryNm,
			Name:           item.CcyNm,
			AlphabeticCode: item.Ccy,
		}

		if num, err := strconv.Atoi(strings.TrimSpace(item.CcyNbr)); err == nil {
			currency.NumericCode = &num
		}
		if minor, err := strconv.Atoi(strings.TrimSpace(item.CcyMnrUnts)); err == nil {
			currency.MinorUnit = &minor
		}

		currencies = append(currencies, currency)
	}

	return currencies, nil
}

// FetchCurrencies fetches and parses the SIX Group ISO-4217 XML.
func (fetcher *Fetcher) FetchCurrencies(ctx context.Context) ([]Currency, error) {
	currenciesRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, fetcher.currenciesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create currencies request: %w", err)
	}
	currenciesResponse, err := fetcher.httpClient.Do(currenciesRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ISO-4217 XML: %w", err)
	}
	defer func() { _ = currenciesResponse.Body.Close() }()

	if currenciesResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("currencies endpoint returned status %d", currenciesResponse.StatusCode)
	}

	xmlBytes, err := io.ReadAll(io.LimitReader(currenciesResponse.Body, maxResponseBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read currencies response body: %w", err)
	}

	return ParseCurrenciesXML(xmlBytes)
}

// FetchAndSaveDataFiles fetches countries and currencies and writes them as JSON into outputDir.
func (fetcher *Fetcher) FetchAndSaveDataFiles(ctx context.Context, outputDir string) error {
	countries, err := fetcher.FetchCountries(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch countries: %w", err)
	}

	currencies, err := fetcher.FetchCurrencies(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch currencies: %w", err)
	}

	if err := os.MkdirAll(outputDir, directoryPermission); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	countriesJSON, _ := json.MarshalIndent(countries, "", "  ")
	if err := os.WriteFile(filepath.Join(outputDir, "countries.json"), countriesJSON, filePermission); err != nil {
		return fmt.Errorf("failed to write countries.json: %w", err)
	}

	currenciesJSON, _ := json.MarshalIndent(currencies, "", "  ")
	if err := os.WriteFile(filepath.Join(outputDir, "currencies.json"), currenciesJSON, filePermission); err != nil {
		return fmt.Errorf("failed to write currencies.json: %w", err)
	}

	return nil
}
