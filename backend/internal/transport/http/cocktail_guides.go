package http

import (
	"bytes"
	"net/http"

	"go.uber.org/zap"

	"racion/internal/domain"
)

type cocktailSection struct {
	Heading, Note string
	Recipes       []string
}

type cocktailGuide struct {
	Slug, Name, Description string
	TableSlug, TableName    string
	Sections                []cocktailSection
	FAQ                     []domain.QA
}

func (g cocktailGuide) Path() string { return "/collection/cocktails/" + g.Slug }

var cocktailGuides = []cocktailGuide{
	{
		Slug: "new-year", Name: "Коктейли на Новый год",
		Description: "Мандариновый беллини и кир рояль для тоста в полночь, гранатовый спритц и мандарин с клюквой для гостей, которые хотят поднять бокал без алкоголя.",
		TableSlug:   "new-year-table", TableName: "Что приготовить на новогодний стол",
		Sections: []cocktailSection{
			{"С игристым к полуночи", "Если времени мало, выберите кир рояль. На бокал нужно 10 мл смородинового ликёра и 110 мл игристого. Для беллини заранее очистите мандарины и приготовьте пюре. Игристое открывайте, когда гости соберутся за столом.", []string{"ny_kir_royal", "ny_mandarin_bellini", "bar_mimosa"}},
			{"Безалкогольные коктейли для тоста", "Гранат и имбирь дают спритцу терпкость, у мандарина с клюквой вкус кислее. Поставьте соки в холодильник заранее и оставьте газированную воду закрытой до подачи.", []string{"ny_pomegranate_spritz", "bar_mandarin_cranberry_soda", "bar_blackcurrant_apple_sparkler"}},
		},
		FAQ: []domain.QA{{Q: "Сколько игристого взять для «Мимозы» на шестерых?", A: "Для шести бокалов по этому рецепту понадобится 390 мл игристого и столько же апельсинового сока. Отмерьте по 65 мл холодного сока в каждый бокал и долейте по 65 мл вина."}},
	},
	{
		Slug: "christmas", Name: "Коктейли на Рождество",
		Description: "Горячий яблочный пунш с корицей, груша с розмарином и клюквенный шот. Что подать гостям с мороза и какой напиток выбрать к рождественской выпечке.",
		TableSlug:   "christmas-table", TableName: "Меню на Рождество",
		Sections: []cocktailSection{
			{"Пунш на яблочном соке без алкоголя", "После прогулки разлейте горячий яблочный пунш по кружкам. Гвоздика и имбирь дают ему пряный вкус. Для более мягкого напитка возьмите грушу с розмарином. Оба пунша удобно готовить в одной большой кастрюле по числу гостей.", []string{"hot_apple_punch", "ny_pear_punch"}},
			{"К печенью и пряникам", "К имбирному печенью подойдёт кислая клюква с корицей в маленькой рюмке. Для тех, кто любит менее сладкие напитки, смешайте яблочный сок с холодным чёрным чаем. В рецепте айс-ти нет алкоголя.", []string{"bar_cranberry_cinnamon_shot", "bar_apple_black_tea", "bar_spiced_grape_punch"}},
		},
		FAQ: []domain.QA{{Q: "Как подать горячий пунш, если гости приходят в разное время?", A: "Процедите яблочный пунш, когда специи отдадут аромат. Оставьте под крышкой и подогревайте небольшими порциями. Так у последней кружки не будет резкого привкуса гвоздики."}},
	},
	{
		Slug: "halloween", Name: "Коктейли на Хэллоуин",
		Description: "Зелёное «Зелье ведьмы» с киви, томатная «Кровавая Мэри» и дынный сауэр. Цвет этим коктейлям дают сами ингредиенты, пищевой краситель не нужен.",
		TableSlug:   "halloween-party", TableName: "Закуски и блюда на Хэллоуин",
		Sections: []cocktailSection{
			{"Зелёные коктейли для вечеринки", "Для общей чаши сделайте безалкогольное «Зелье ведьмы» из киви, мяты и яблочного сока. Разлейте в прозрачные стаканы, чтобы были видны чёрные зёрнышки киви. Для алкогольного варианта отдельно смешайте «Мидори сауэр» с зелёным дынным ликёром.", []string{"hw_witches_brew_punch", "bar_midori_sour"}},
			{"Красные коктейли с перцем или ягодами", "«Кровавая Мэри» с томатом и перцем подойдёт к солёным закускам. Рядом можно поставить «Вёрджин Мэри» без водки. Для любителей сладкого есть рубиновый каркаде с апельсином и медовым сиропом.", []string{"bar_bloody_mary", "bar_virgin_mary", "bar_hibiscus_orange_punch"}},
		},
		FAQ: []domain.QA{{Q: "Как различить похожие красные коктейли на столе?", A: "Подайте томатные коктейли в стаканах разной формы и подпишите оба кувшина. Острый соус поставьте рядом, чтобы каждый мог добавить его по вкусу."}},
	},
	{
		Slug: "birthday", Name: "Коктейли на день рождения",
		Description: "Напитки на день рождения дома для тех, кто собирает друзей за одним столом. Пунш на компанию, коктейли с гренадином и клубничный лимонад в кувшине.",
		TableSlug:   "birthday-table", TableName: "Меню на день рождения дома",
		Sections: []cocktailSection{
			{"Что смешать на компанию", "В «Плантерс панч» идут тёмный ром, лайм и биттер. Отмерьте основу сразу на несколько порций, а охлаждайте перед подачей. Второй кувшин отдайте клубничному лимонаду или ягодному мохито. Обе версии без алкоголя, мяту и базилик добавьте ближе к приходу гостей.", []string{"bar_planter_s_punch", "bar_strawberry_basil_lemonade", "vd_berry_mojito_nonalco"}},
			{"Что подать в отдельных бокалах", "Для двухцветного «Санрайза» налейте гренадин в бокал с апельсиновым соком и дайте сиропу осесть. Тот же переход от красного к золотому получится и в версии без текилы. К десерту можно предложить гостям ореховый шот «Шоколадный торт».", []string{"bar_tequila_sunrise", "bar_sunrise_without_tequila", "bar_chocolate_cake_shot"}},
		},
		FAQ: []domain.QA{{Q: "Сколько газированной воды нужно на восемь клубничных лимонадов?", A: "На восемь порций по этому рецепту нужно 920 мл газированной воды и 320 г клубники. Приготовьте ягодную основу заранее, а воду добавьте перед подачей. Для льда оставьте место в стаканах."}},
	},
	{
		Slug: "friday", Name: "Простые коктейли на вечер пятницы",
		Description: "Что смешать дома в пятницу без шейкера. Джин-тоник и виски-кола из двух ингредиентов, грейпфрутовая «Палома» и имбирный мул без алкоголя.",
		Sections: []cocktailSection{
			{"Прямо в стакане за две минуты", "Если дома только виски и кола, на один стакан хватит 50 мл виски и 150 мл холодной колы. Для джин-тоника пропорция та же. Когда хочется цитрусовой кислинки, возьмите «Палому» с лаймом и грейпфрутовой газировкой.", []string{"bar_whisky_cola", "bar_gin_and_tonic", "bar_paloma"}},
			{"Без алкоголя к ужину или после него", "Имбирный мул с лаймом подойдёт к острой закуске. К десерту можно сделать эспрессо-тоник, если хочется кофе, или смешать клюкву с лаймом и льдом. Все три напитка готовятся порционно.", []string{"bar_ginger_lime_mule_zero", "bar_espresso_tonic", "bar_cranberry_lime_cooler"}},
		},
		FAQ: []domain.QA{{Q: "Как сделать коктейль на двоих, если дома нет мерного стаканчика?", A: "Для виски-колы подойдёт небольшая рюмка. На одну рюмку виски возьмите три рюмки колы, добавьте лёд и повторите для второго стакана. Той же рюмкой можно отмерить джин и тоник."}},
	},
}

func (s *Server) cocktailGuidePage(w http.ResponseWriter, r *http.Request) {
	for _, g := range cocktailGuides {
		if g.Slug != r.PathValue("occasion") {
			continue
		}
		pl, _ := s.stableLocale(r)
		rememberCountry(w, r)
		type sectionView struct {
			cocktailSection
			Cards []recipeCard
		}
		var sections []sectionView
		var all []recipeCard
		cover := ""
		for _, section := range g.Sections {
			var cards []recipeCard
			for _, id := range section.Recipes {
				if rc, ok := s.catalog.RecipeByID[id]; ok && !rc.Hidden && rc.Slot == "cocktail" {
					cards = append(cards, s.card(rc, pl))
					if cover == "" {
						cover = rc.Image
					}
				}
			}
			if len(cards) > 0 {
				sections = append(sections, sectionView{section, cards})
				all = append(all, cards...)
			}
		}
		if len(all) == 0 {
			break
		}
		base := s.baseURL(r)
		data := map[string]any{
			"Base": pageBase{Title: g.Name + " — рецепты с фото — Рацион", Description: g.Description, Canonical: base + g.Path(), OGImage: topicOG(base, cover, pl.L), OGType: "article", OGWide: cover == "", User: currentUser(r) != nil,
				JSONLD: collectionLD(base, pl, g.Name, g.Description, "cocktails/"+g.Slug, all, g.FAQ, [2]string{"Коктейли", base + "/collection/cocktails"})},
			"L": pl.L, "P": pl.P, "Country": pl.Country, "NavRecipes": true, "Guide": g, "Sections": sections, "Cover": cover,
		}
		var buf bytes.Buffer
		if err := pageTpl.ExecuteTemplate(&buf, "cocktail_guide.html", data); err != nil {
			s.log.Error("cocktail guide", zap.Error(err))
			s.errorPage(w, r, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = buf.WriteTo(w)
		return
	}
	s.notFoundPage(w, r)
}
