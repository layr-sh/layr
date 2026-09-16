package referencedata

// CountryName represents the common, official, and native names of a country.
type CountryName struct {
	Common   string                         `json:"common"`
	Official string                         `json:"official"`
	Native   map[string]CountryNativeDetail `json:"native,omitempty"`
}

// CountryNativeDetail represents the localized official and common name.
type CountryNativeDetail struct {
	Official string `json:"official"`
	Common   string `json:"common"`
}

// CurrencyDetail represents the currency name and symbol for a country.
type CurrencyDetail struct {
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
}

// IDDDetail represents international direct dialing root and suffixes.
type IDDDetail struct {
	Root     string   `json:"root"`
	Suffixes []string `json:"suffixes"`
}

// TranslationDetail represents official and common translated country names.
type TranslationDetail struct {
	Common   string `json:"common"`
	Official string `json:"official"`
}

// DemonymDetail represents female and male demonym designations.
type DemonymDetail struct {
	F string `json:"f"`
	M string `json:"m"`
}

// Country models the full country structure matching mledoze/countries and tzdb timezones.
type Country struct {
	Name            CountryName                  `json:"name"`
	TLD             []string                     `json:"tld"`
	CCA2            string                       `json:"cca2"`
	CCN3            string                       `json:"ccn3"`
	CCA3            string                       `json:"cca3"`
	CIOC            string                       `json:"cioc"`
	Independent     *bool                        `json:"independent"`
	Status          string                       `json:"status"`
	UNMember        bool                         `json:"unMember"`
	UNRegionalGroup *string                      `json:"unRegionalGroup"`
	Currencies      map[string]CurrencyDetail    `json:"currencies"`
	IDD             IDDDetail                    `json:"idd"`
	Capital         []string                     `json:"capital"`
	AltSpellings    []string                     `json:"altSpellings"`
	Region          string                       `json:"region"`
	Subregion       string                       `json:"subregion"`
	Languages       map[string]string            `json:"languages"`
	Translations    map[string]TranslationDetail `json:"translations"`
	LatLng          []float64                    `json:"latlng"`
	Landlocked      bool                         `json:"landlocked"`
	Borders         []string                     `json:"borders"`
	Area            float64                      `json:"area"`
	Flag            string                       `json:"flag"`
	Demonyms        map[string]DemonymDetail     `json:"demonyms"`
	Timezones       []string                     `json:"timezones"`
}

// Currency models the ISO-4217 currency record.
type Currency struct {
	CountryName    string `json:"countryName"`
	Name           string `json:"name"`
	AlphabeticCode string `json:"alphabeticCode"`
	NumericCode    *int   `json:"numericCode"`
	MinorUnit      *int   `json:"minorUnit"`
}
