package planner

import "testing"

func TestRegionByPlace(t *testing.T) {
	regs := []Region{
		{Code: "msk", Name: "Москва", Kind: "region"}, {Code: "mo", Name: "Московская область", Kind: "region"},
		{Code: "kursk", Name: "Курская область", Kind: "region"}, {Code: "kurgan", Name: "Курганская область", Kind: "region"},
		{Code: "sverdl", Name: "Свердловская область", Kind: "region"}, {Code: "ekb", Name: "Екатеринбург", Kind: "city", Parent: "sverdl"},
		{Code: "tat", Name: "Республика Татарстан", Kind: "region"}, {Code: "kzn", Name: "Казань", Kind: "city", Parent: "tat"},
		{Code: "nn", Name: "Нижегородская область", Kind: "region"}, {Code: "nnov", Name: "Нижний Новгород", Kind: "city", Parent: "nn"},
		{Code: "spb", Name: "Санкт-Петербург", Kind: "region"}, {Code: "osetia", Name: "Республика Северная Осетия-Алания", Kind: "region"},
		{Code: "krd", Name: "Краснодарский край", Kind: "region"}, {Code: "sochi", Name: "Сочи", Kind: "city", Parent: "krd"},
	}
	cases := []struct{ sub, city, want string }{
		{"Moscow", "Moscow", "msk"}, {"Moscow Oblast", "Balashikha", "mo"}, {"Kursk", "Kursk", "kursk"}, {"Kurgan", "Kurgan", "kurgan"},
		{"Sverdlovsk", "Yekaterinburg", "ekb"}, {"Sverdlovsk", "Nizhny Tagil", "sverdl"}, {"Tatarstan", "Kazan’", "kzn"},
		{"Nizhniy Novgorod", "Nizhniy Novgorod", "nnov"}, {"St.-Petersburg", "Saint Petersburg", "spb"}, {"North Ossetia", "Vladikavkaz", "osetia"},
		{"Krasnodar Krai", "Sochi", "sochi"}, {"", "", ""}, {"Bavaria", "Munich", ""},
	}
	for _, c := range cases {
		if got := RegionByPlace(regs, c.sub, c.city); got != c.want {
			t.Errorf("%q/%q: got %q want %q", c.sub, c.city, got, c.want)
		}
	}
}
