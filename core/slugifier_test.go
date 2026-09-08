package core

import (
	"testing"
)

type slugifyTestCase struct {
	input    string
	expected string
}

func testSlugify(t *testing.T, slugifier Slugifier, input, expected string) {
	t.Helper()
	actual := slugifier.Slugify(input)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}

func testSlugifyCases(t *testing.T, slugifier Slugifier, cases []slugifyTestCase) {
	t.Helper()
	for _, testItem := range cases {
		testSlugify(t, slugifier, testItem.input, testItem.expected)
	}
}

func TestCoreSlugifyDefaultsUnit(t *testing.T) {
	cases := []slugifyTestCase{
		{"", ""},
		{"abc", "abc"},
		{"abc234", "abc234"},
		{"This is a test ---", "this-is-a-test"},
		{"___This is a test___", "this-is-a-test"},
		{"This -- is a ## test ---", "this-is-a-test"},
		{"北京kožušček", "bei-jing-kozuscek"},
		{"Nín hǎo. Wǒ shì zhōng guó rén", "nin-hao-wo-shi-zhong-guo-ren"},
		{`C\'est déjà l\'été.`, "c-est-deja-l-ete"},
	}

	slugifier := NewSlugifier()
	testSlugifyCases(t, slugifier, cases)
}

func TestCoreSlugifyToLowerUnit(t *testing.T) {
	cases := []slugifyTestCase{
		{"", ""},
		{"abc", "abc"},
		{"abc234", "abc234"},
		{"This is a test ---", "This-is-a-test"},
		{"___This is a test___", "This-is-a-test"},
		{"This -- is a ## test ---", "This-is-a-test"},
		{"北京kožušček", "Bei-Jing-kozuscek"},
		{"Nín hǎo. Wǒ shì zhōng guó rén", "Nin-hao-Wo-shi-zhong-guo-ren"},
		{`C\'est déjà l\'été.`, "C-est-deja-l-ete"},
	}

	slugifier := NewSlugifier()
	slugifier.ToLower(false)
	testSlugifyCases(t, slugifier, cases)
}

func TestCoreSlugifyWordSeparatorUnit(t *testing.T) {
	cases := []slugifyTestCase{
		{"", ""},
		{"abc", "abc"},
		{"abc234", "abc234"},
		{"This is a test ---", "this_is_a_test"},
		{"_-_This is a test", "this_is_a_test"},
		{"This -- is \t\t  \r\n a ## test ---", "this_--_is_a_--_test"},
		{"北京kožušček", "bei_jing_kozuscek"},
		{"Nín hǎo. Wǒ shì zhōng guó rén", "nin_hao-_wo_shi_zhong_guo_ren"},
		{`C\'est déjà l\'été.`, "c--est_deja_l--ete"},
	}

	slugifier := NewSlugifier()
	slugifier.WordSeparator("_")
	testSlugifyCases(t, slugifier, cases)
}

func TestCoreSlugifyEmptyWordSeparatorUnit(t *testing.T) {
	cases := []slugifyTestCase{
		{"", ""},
		{"abc", "abc"},
		{"abc234", "abc234"},
		{"This is a test ---", "thisisatest"},
		{"_-_This is a test", "thisisatest"},
		{"This -- is \t\t  \r\n a ## test ---", "this--isa--test"},
		{"北京kožušček", "beijingkozuscek"},
		{"Nín hǎo. Wǒ shì zhōng guó rén", "ninhao-woshizhongguoren"},
		{`C\'est déjà l\'été.`, "c--estdejal--ete"},
	}

	slugifier := NewSlugifier()
	slugifier.WordSeparator("")
	testSlugifyCases(t, slugifier, cases)
}

func TestCoreSlugifyInvalidCharReplacementUnit(t *testing.T) {
	cases := []slugifyTestCase{
		{"", ""},
		{"abc", "abc"},
		{"abc234", "abc234"},
		{"This is a test ---", "this-is-a-test"},
		{"_-_This is a test", "this-is-a-test"},
		{"This -- is \t\t  \r\n a ## test ---", "this-is-a-__-test"},
		{"北京kožušček", "bei-jing-kozuscek"},
		{"Nín hǎo. Wǒ shì zhōng guó rén", "nin-hao_-wo-shi-zhong-guo-ren"},
		{`C\'est déjà l\'été.`, "c__est-deja-l__ete"},
	}

	slugifier := NewSlugifier()
	slugifier.InvalidCharacter("_")
	testSlugifyCases(t, slugifier, cases)
}

func TestCoreSlugifyEmptyInvalidCharReplacementUnit(t *testing.T) {
	cases := []slugifyTestCase{
		{"", ""},
		{"abc", "abc"},
		{"abc234", "abc234"},
		{"This is a test ---", "this-is-a-test"},
		{"_-_This is a test", "this-is-a-test"},
		{"This -- is \t\t  \r\n a ## test ---", "this-is-a-test"},
		{"北京kožušček", "bei-jing-kozuscek"},
		{"Nín hǎo. Wǒ shì zhōng guó rén", "nin-hao-wo-shi-zhong-guo-ren"},
		{`C\'est déjà l\'été.`, "cest-deja-lete"},
	}

	slugifier := NewSlugifier()
	slugifier.InvalidCharacter("")
	testSlugifyCases(t, slugifier, cases)
}

func TestCoreSlugifyEmptyWordSeparatorAndInvalidCharReplacementUnit(t *testing.T) {
	cases := []slugifyTestCase{
		{"", ""},
		{"abc", "abc"},
		{"abc234", "abc234"},
		{"This is a test ---", "thisisatest"},
		{"_-_This is a test", "thisisatest"},
		{"This -- is \t\t  \r\n a ## test ---", "thisisatest"},
		{"北京kožušček", "beijingkozuscek"},
		{"Nín hǎo. Wǒ shì zhōng guó rén", "ninhaowoshizhongguoren"},
		{`C\'est déjà l\'été.`, "cestdejalete"},
	}

	slugifier := NewSlugifier()
	slugifier.InvalidCharacter("")
	slugifier.WordSeparator("")
	testSlugifyCases(t, slugifier, cases)
}

func TestCoreSlugifyReplacementsBecomeValidCharactersUnit(t *testing.T) {
	cases := []slugifyTestCase{
		{"**##x**##**x##**", "x*##*x"},
		{"##**x##**##x**##", "x##*##x"},
	}

	slugifier := NewSlugifier()
	slugifier.InvalidCharacter("#")
	slugifier.WordSeparator("*")
	testSlugifyCases(t, slugifier, cases)
}

func TestCoreSlugifyChainingSetupUnit(t *testing.T) {
	cases := []slugifyTestCase{
		{"This -- is \t\t  \r\n a ## test ---", "This*##*is*a*##*test"},
	}

	slugifier := (&Slugifier{}).ToLower(false).InvalidCharacter("#").WordSeparator("*")
	testSlugifyCases(t, *slugifier, cases)
}

func TestCoreSlugifyAllowedSetUnit(t *testing.T) {
	slugifier := NewSlugifier()
	slugifier.AllowedSet("a-zA-Z0-9_")
	actual := slugifier.Slugify("Hello_World-123!")
	expected := "hello_world-123"
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}

func TestCoreSlugifyUninitializedUnit(t *testing.T) {
	var slugifier Slugifier
	actual := slugifier.Slugify("Uninitialized Slugifier Test")
	expected := "uninitialized-slugifier-test"
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}
