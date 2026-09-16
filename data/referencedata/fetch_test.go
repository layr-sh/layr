package referencedata_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"layr.sh/data/referencedata"
)

const (
	testDirectoryPerm = 0755
)

func TestReferencedataParseZoneTabUnit(t *testing.T) {
	rawTab := `# Comment line
ID	-0610+10648	Asia/Jakarta	Java & Sumatra
MY	+0310+10142	Asia/Kuala_Lumpur	peninsular Malaysia
US	+404251-0740023	America/New_York	Eastern (most areas)
`
	entries := referencedata.ParseZoneTab(rawTab)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Country != "ID" || entries[0].Identifier != "Asia/Jakarta" || entries[0].Comments != "Java & Sumatra" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].Country != "MY" || entries[1].Identifier != "Asia/Kuala_Lumpur" {
		t.Fatalf("unexpected second entry: %+v", entries[1])
	}
	if entries[2].Country != "US" || entries[2].Identifier != "America/New_York" {
		t.Fatalf("unexpected third entry: %+v", entries[2])
	}
}

func TestReferencedataParseCurrenciesXMLUnit(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<ISO_4217>
	<CcyTbl>
		<CcyNtry>
			<CtryNm>UNITED STATES OF AMERICA (THE)</CtryNm>
			<CcyNm>US Dollar</CcyNm>
			<Ccy>USD</Ccy>
			<CcyNbr>840</CcyNbr>
			<CcyMnrUnts>2</CcyMnrUnts>
		</CcyNtry>
		<CcyNtry>
			<CtryNm>ANTARCTICA</CtryNm>
			<CcyNm>No universal currency</CcyNm>
			<Ccy></Ccy>
			<CcyNbr></CcyNbr>
			<CcyMnrUnts></CcyMnrUnts>
		</CcyNtry>
	</CcyTbl>
</ISO_4217>`)

	currencies, err := referencedata.ParseCurrenciesXML(xmlContent)
	if err != nil {
		t.Fatalf("unexpected error parsing XML: %v", err)
	}

	if len(currencies) != 2 {
		t.Fatalf("expected 2 currencies, got %d", len(currencies))
	}

	if currencies[0].AlphabeticCode != "USD" || *currencies[0].NumericCode != 840 || *currencies[0].MinorUnit != 2 {
		t.Fatalf("unexpected first currency: %+v", currencies[0])
	}

	if currencies[1].AlphabeticCode != "" || currencies[1].NumericCode != nil {
		t.Fatalf("unexpected second currency: %+v", currencies[1])
	}

	// Invalid XML
	if _, err := referencedata.ParseCurrenciesXML([]byte("invalid-xml")); err == nil {
		t.Fatalf("expected error for invalid XML")
	}
}

func TestReferencedataFetchCountriesAndCurrenciesUnit(t *testing.T) {
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/countries.json", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`[
			{
				"name": { "common": "Indonesia", "official": "Republic of Indonesia" },
				"cca2": "ID",
				"cca3": "IDN",
				"region": "Asia",
				"subregion": "South-Eastern Asia"
			},
			{
				"name": { "common": "Unknown Land", "official": "Unknown" },
				"cca2": "XX",
				"cca3": "XXX"
			}
		]`))
	})

	serveMux.HandleFunc("/zone.tab", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "text/plain")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ID\t+0000+00000\tAsia/Jakarta\tJakarta\n"))
	})

	serveMux.HandleFunc("/list-one.xml", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/xml")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<ISO_4217>
	<CcyTbl>
		<CcyNtry>
			<CtryNm>INDONESIA</CtryNm>
			<CcyNm>Rupiah</CcyNm>
			<Ccy>IDR</Ccy>
			<CcyNbr>360</CcyNbr>
			<CcyMnrUnts>2</CcyMnrUnts>
		</CcyNtry>
	</CcyTbl>
</ISO_4217>`))
	})

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()

	fetcher := referencedata.NewFetcher(testServer.Client())
	fetcher.SetURLs(
		testServer.URL+"/countries.json",
		testServer.URL+"/zone.tab",
		testServer.URL+"/list-one.xml",
	)

	ctx := context.Background()

	// 1. Fetch Countries
	countries, err := fetcher.FetchCountries(ctx)
	if err != nil {
		t.Fatalf("FetchCountries failed: %v", err)
	}

	if len(countries) != 2 {
		t.Fatalf("expected 2 countries, got %d", len(countries))
	}

	var indonesiaCountry referencedata.Country
	for _, country := range countries {
		if country.CCA2 == "ID" {
			indonesiaCountry = country
			break
		}
	}

	if indonesiaCountry.CCA2 != "ID" || len(indonesiaCountry.Timezones) != 1 || indonesiaCountry.Timezones[0] != "Asia/Jakarta" {
		t.Fatalf("unexpected indonesia data: %+v", indonesiaCountry)
	}

	// 2. Fetch Currencies
	currencies, err := fetcher.FetchCurrencies(ctx)
	if err != nil {
		t.Fatalf("FetchCurrencies failed: %v", err)
	}

	if len(currencies) != 1 || currencies[0].AlphabeticCode != "IDR" {
		t.Fatalf("unexpected currencies data: %+v", currencies)
	}

	// 3. FetchAndSaveDataFiles
	tempDir := t.TempDir()
	if err = fetcher.FetchAndSaveDataFiles(ctx, tempDir); err != nil {
		t.Fatalf("FetchAndSaveDataFiles failed: %v", err)
	}

	countriesBytes, readCountriesErr := os.ReadFile(filepath.Join(tempDir, "countries.json"))
	if readCountriesErr != nil || len(countriesBytes) == 0 {
		t.Fatalf("failed to read saved countries.json: %v", readCountriesErr)
	}

	currenciesBytes, readCurrenciesErr := os.ReadFile(filepath.Join(tempDir, "currencies.json"))
	if readCurrenciesErr != nil || len(currenciesBytes) == 0 {
		t.Fatalf("failed to read saved currencies.json: %v", readCurrenciesErr)
	}
}

func TestReferencedataFetchAndSaveDataFilesErrorsUnit(t *testing.T) {
	ctx := context.Background()
	fetcher := referencedata.NewFetcher(nil)

	// Fetch countries error
	fetcher.SetURLs("http://[::1]:namedport", "", "")
	if err := fetcher.FetchAndSaveDataFiles(ctx, t.TempDir()); err == nil {
		t.Fatalf("expected error on failed countries fetch")
	}

	// Fetch currencies error
	countriesServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`[]`))
	}))
	defer countriesServer.Close()

	fetcher.SetURLs(countriesServer.URL, countriesServer.URL, "http://[::1]:namedport")
	if err := fetcher.FetchAndSaveDataFiles(ctx, t.TempDir()); err == nil {
		t.Fatalf("expected error on failed currencies fetch")
	}

	// Unwritable directory error
	currenciesServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/xml")
		_, _ = responseWriter.Write([]byte(`<ISO_4217><CcyTbl></CcyTbl></ISO_4217>`))
	}))
	defer currenciesServer.Close()

	fetcher.SetURLs(countriesServer.URL, countriesServer.URL, currenciesServer.URL)

	// Invalid dir path on unix
	uncreatablePath := "/dev/null/cannot_create_dir"
	if err := fetcher.FetchAndSaveDataFiles(ctx, uncreatablePath); err == nil {
		t.Fatalf("expected error on uncreatable output dir")
	}

	// Write error for file (if path is a directory named countries.json)
	dirAsFile := filepath.Join(t.TempDir(), "blocker")
	_ = os.MkdirAll(filepath.Join(dirAsFile, "countries.json"), testDirectoryPerm)
	if err := fetcher.FetchAndSaveDataFiles(ctx, dirAsFile); err == nil {
		t.Fatalf("expected error on blocked countries.json")
	}

	// Write error for currencies.json
	dirAsCurrFile := filepath.Join(t.TempDir(), "curr_blocker")
	_ = os.MkdirAll(filepath.Join(dirAsCurrFile, "currencies.json"), testDirectoryPerm)
	if err := fetcher.FetchAndSaveDataFiles(ctx, dirAsCurrFile); err == nil {
		t.Fatalf("expected error on blocked currencies.json")
	}
}

func TestReferencedataFetcherErrorsUnit(t *testing.T) {
	fetcher := referencedata.NewFetcher(&http.Client{Timeout: 1 * time.Second})

	// 1. Invalid countries URL (NewRequest error)
	fetcher.SetURLs("http://[::1]:namedport", "", "")
	if _, err := fetcher.FetchCountries(context.Background()); err == nil {
		t.Fatalf("expected error on invalid countries URL")
	}

	// 2. Fetch error (unreachable port)
	fetcher.SetURLs("http://127.0.0.1:59999/countries.json", "", "")
	if _, err := fetcher.FetchCountries(context.Background()); err == nil {
		t.Fatalf("expected error on unreachable countries endpoint")
	}

	// 3. Countries non-200
	errorServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	fetcher.SetURLs(errorServer.URL, "", "")
	if _, err := fetcher.FetchCountries(context.Background()); err == nil {
		t.Fatalf("expected error on countries 500 status")
	}

	// 4. Invalid JSON
	badJSONServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("not-json"))
	}))
	defer badJSONServer.Close()

	fetcher.SetURLs(badJSONServer.URL, "", "")
	if _, err := fetcher.FetchCountries(context.Background()); err == nil {
		t.Fatalf("expected error on invalid countries JSON")
	}

	// 5. Timezones errors
	validCountriesServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`[]`))
	}))
	defer validCountriesServer.Close()

	fetcher.SetURLs(validCountriesServer.URL, "http://[::1]:namedport", "")
	if _, err := fetcher.FetchCountries(context.Background()); err == nil {
		t.Fatalf("expected error on invalid timezones URL")
	}

	fetcher.SetURLs(validCountriesServer.URL, "http://127.0.0.1:59999/zone.tab", "")
	if _, err := fetcher.FetchCountries(context.Background()); err == nil {
		t.Fatalf("expected error on unreachable timezones endpoint")
	}

	fetcher.SetURLs(validCountriesServer.URL, errorServer.URL, "")
	if _, err := fetcher.FetchCountries(context.Background()); err == nil {
		t.Fatalf("expected error on timezones 500 status")
	}

	// 6. Currencies errors
	fetcher.SetURLs("", "", "http://[::1]:namedport")
	if _, err := fetcher.FetchCurrencies(context.Background()); err == nil {
		t.Fatalf("expected error on invalid currencies URL")
	}

	fetcher.SetURLs("", "", "http://127.0.0.1:59999/list-one.xml")
	if _, err := fetcher.FetchCurrencies(context.Background()); err == nil {
		t.Fatalf("expected error on unreachable currencies endpoint")
	}

	fetcher.SetURLs("", "", errorServer.URL)
	if _, err := fetcher.FetchCurrencies(context.Background()); err == nil {
		t.Fatalf("expected error on currencies 500 status")
	}

	// 7. Body read errors
	errClient := &http.Client{
		Transport: &mockErrTransport{},
	}
	errBodyFetcher := referencedata.NewFetcher(errClient)
	errBodyFetcher.SetURLs("http://example.com/countries", "http://example.com/timezone", "http://example.com/currencies")
	if _, err := errBodyFetcher.FetchCountries(context.Background()); err == nil {
		t.Fatalf("expected error on countries body read failure")
	}
	if _, err := errBodyFetcher.FetchCurrencies(context.Background()); err == nil {
		t.Fatalf("expected error on currencies body read failure")
	}

	// Body read error on second call (timezones)
	timezoneErrorClient := &http.Client{
		Transport: &mockTimezoneErrTransport{},
	}
	timezoneErrorFetcher := referencedata.NewFetcher(timezoneErrorClient)
	timezoneErrorFetcher.SetURLs("http://example.com/countries", "http://example.com/timezone", "http://example.com/currencies")
	if _, err := timezoneErrorFetcher.FetchCountries(context.Background()); err == nil {
		t.Fatalf("expected error on timezones body read failure")
	}
}

type mockErrReader struct{}

func (mockErrReader *mockErrReader) Read(payload []byte) (int, error) {
	return 0, fmt.Errorf("simulated read error")
}

func (mockErrReader *mockErrReader) Close() error {
	return nil
}

type mockErrTransport struct{}

func (mockErrTransport *mockErrTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockErrReader{},
		Header:     make(http.Header),
	}, nil
}

type mockTimezoneErrTransport struct{}

func (mockTimezoneErrTransport *mockTimezoneErrTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if strings.Contains(request.URL.Path, "countries") {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("[]")),
			Header:     make(http.Header),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &mockErrReader{},
		Header:     make(http.Header),
	}, nil
}
