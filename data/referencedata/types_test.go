package referencedata

import (
	"encoding/json"
	"testing"
)

func TestReferencedataTypesSerializationUnit(t *testing.T) {
	independent := true
	numericCode := 840
	minorUnit := 2
	unRegionalGroup := "Western European and Others"

	country := Country{
		Name: CountryName{
			Common:   "United States",
			Official: "United States of America",
			Native: map[string]CountryNativeDetail{
				"eng": {
					Official: "United States of America",
					Common:   "United States",
				},
			},
		},
		TLD:             []string{".us"},
		CCA2:            "US",
		CCN3:            "840",
		CCA3:            "USA",
		CIOC:            "USA",
		Independent:     &independent,
		Status:          "officially-assigned",
		UNMember:        true,
		UNRegionalGroup: &unRegionalGroup,
		Currencies: map[string]CurrencyDetail{
			"USD": {
				Name:   "United States dollar",
				Symbol: "$",
			},
		},
		IDD: IDDDetail{
			Root:     "+1",
			Suffixes: []string{"201", "202"},
		},
		Capital:      []string{"Washington, D.C."},
		AltSpellings: []string{"US", "USA"},
		Region:       "Americas",
		Subregion:    "Northern America",
		Languages: map[string]string{
			"eng": "English",
		},
		Translations: map[string]TranslationDetail{
			"fra": {
				Common:   "États-Unis",
				Official: "Les états-unis d'Amérique",
			},
		},
		LatLng:     []float64{38.0, -97.0},
		Landlocked: false,
		Borders:    []string{"CAN", "MEX"},
		Area:       9372610.0,
		Flag:       "🇺🇸",
		Demonyms: map[string]DemonymDetail{
			"eng": {
				F: "American",
				M: "American",
			},
		},
		Timezones: []string{"America/New_York", "America/Los_Angeles"},
	}

	marshaledCountryJSON, err := json.Marshal(country)
	if err != nil {
		t.Fatalf("failed to marshal country: %v", err)
	}

	var unmarshaledCountry Country
	if err = json.Unmarshal(marshaledCountryJSON, &unmarshaledCountry); err != nil {
		t.Fatalf("failed to unmarshal country: %v", err)
	}

	if unmarshaledCountry.CCA2 != "US" || unmarshaledCountry.Name.Common != "United States" {
		t.Fatalf("unexpected unmarshaled country: %+v", unmarshaledCountry)
	}

	currency := Currency{
		CountryName:    "UNITED STATES OF AMERICA (THE)",
		Name:           "US Dollar",
		AlphabeticCode: "USD",
		NumericCode:    &numericCode,
		MinorUnit:      &minorUnit,
	}

	marshaledCurrencyJSON, err := json.Marshal(currency)
	if err != nil {
		t.Fatalf("failed to marshal currency: %v", err)
	}

	var unmarshaledCurrency Currency
	if err = json.Unmarshal(marshaledCurrencyJSON, &unmarshaledCurrency); err != nil {
		t.Fatalf("failed to unmarshal currency: %v", err)
	}

	if unmarshaledCurrency.AlphabeticCode != "USD" || *unmarshaledCurrency.NumericCode != 840 {
		t.Fatalf("unexpected unmarshaled currency: %+v", unmarshaledCurrency)
	}
}
