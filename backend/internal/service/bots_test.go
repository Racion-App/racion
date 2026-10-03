package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/messenger"
	"racion/internal/planner"
)

func botPlan() planner.Plan {
	return planner.Plan{
		ID:      "68a5d55d-0e0d-452a-bd27-2cb4f5892263",
		Country: planner.CountryOf("RU"),
		Shopping: []planner.ShopGroup{
			{Category: "dairy", Label: "Молочное", Items: []planner.ShopItem{
				{IngredientID: "milk", Name: "Молоко 2,5%", Unit: "ml", Pack: 930, Packs: 2, Buy: 1860, Cost: 170},
				{IngredientID: "cheese_hard", Name: "Сыр твёрдый", Unit: "g", Pack: 200, Packs: 1, Buy: 200, Cost: 220},
			}},
			{Category: "pantry", Label: "Бакалея", Items: []planner.ShopItem{
				{IngredientID: "salt", Name: "Соль", Unit: "g", Pantry: true, Cost: 20}, // домашнее: не показываем
			}},
			{Category: "vegetables", Label: "Овощи", Items: []planner.ShopItem{
				{IngredientID: "potato", Name: "Картофель", Unit: "g", Loose: true, Buy: 1200, Cost: 66},
				{IngredientID: "onion", Name: "Лук", Unit: "g", AtHome: true}, // уже дома
			}},
		},
	}
}

func TestShopPagesSkipPantryAndHome(t *testing.T) {
	pages := shopPages(botPlan(), nil, i18n.RU)
	if len(pages) != 2 || pages[0].Label != "Молочное" || pages[1].Label != "Овощи" || len(pages[1].Items) != 1 {
		t.Fatalf("отделы %+v", pages)
	}
	if it := pages[0].Items[0]; it.ID != "milk" || it.Qty != "2 × 930 мл" || it.Cost != 170 {
		t.Errorf("молоко %+v", it)
	}
}

func TestShopPagesOwnItemsLast(t *testing.T) {
	pages := shopPages(botPlan(), []domain.Extra{{ID: 12, Name: "Молоко для кофе", Qty: "1 л"}, {ID: 13, Name: "Батарейки"}}, i18n.RU)
	own := pages[len(pages)-1]
	if len(pages) != 3 || len(own.Items) != 2 || own.Items[0].ID != "extra:12" || own.Items[1].Qty != "" {
		t.Fatalf("свои покупки %+v", pages)
	}
	if it, ok := findItem(pages, 0, "extra:13"); !ok || it.Name != "Батарейки" {
		t.Errorf("своя покупка из чужого отдела: %+v", it)
	}
	// всё куплено, кроме своих: суммы нет, «≈ 0 ₽» не пишем
	b := &Bots{baseURL: "https://racion.app"}
	m := b.listMessage(botPlan(), pages, map[string]bool{"milk": true, "cheese_hard": true, "potato": true, "extra:12": true}, 2, "k", i18n.RU)
	if !strings.Contains(m.Text, "Осталось купить 1 из 5") || strings.Contains(m.Text, "≈") {
		t.Errorf("остались свои: %q", m.Text)
	}
	if m.Rows[1][0].Text != "Батарейки" || m.Rows[1][0].Data != "t:k:2:extra:13" {
		t.Errorf("строка без количества: %+v", m.Rows[1][0])
	}
}

func TestListMessage(t *testing.T) {
	b := &Bots{baseURL: "https://racion.app"}
	key, _ := planKey(botPlan().ID)
	pages := shopPages(botPlan(), nil, i18n.RU)
	m := b.listMessage(botPlan(), pages, map[string]bool{"milk": true}, 0, key, i18n.RU)
	if !strings.HasPrefix(m.Text, "<i>Рацион</i>\n<b>Молочное</b> · отдел 1 из 2\n") || !strings.Contains(m.Text, "Осталось купить 2 из 3 · ≈ ") {
		t.Errorf("заголовок: %q", m.Text)
	}
	// два продукта, листание вперёд, ссылка на сайт
	if len(m.Rows) != 4 {
		t.Fatalf("рядов %d: %+v", len(m.Rows), m.Rows)
	}
	if !strings.HasPrefix(m.Rows[0][0].Text, "✓ Молоко") || !strings.Contains(m.Rows[0][0].Text, "2 × 930") {
		t.Errorf("купленное молоко: %q", m.Rows[0][0].Text)
	}
	if m.Rows[2][0].Data != "l:"+key+":1" || !strings.HasPrefix(m.Rows[2][0].Text, "Овощи") {
		t.Errorf("листание: %+v", m.Rows[2])
	}
	if !strings.Contains(m.Rows[3][0].URL, "/plan/"+botPlan().ID+"?mode=shop") {
		t.Errorf("ссылка: %+v", m.Rows[3])
	}
	// последний отдел: назад есть, вперёд нет; номер страницы за пределами — последний отдел
	last := b.listMessage(botPlan(), pages, nil, 9, key, i18n.RU)
	if !strings.Contains(last.Text, "Овощи") || !strings.HasPrefix(last.Rows[1][0].Text, "‹ Молочное") || len(last.Rows[1]) != 1 {
		t.Errorf("последний отдел: %q %+v", last.Text, last.Rows)
	}
	all := b.listMessage(botPlan(), pages, map[string]bool{"milk": true, "cheese_hard": true, "potato": true}, 0, key, i18n.RU)
	if strings.Contains(all.Text, "Осталось") {
		t.Errorf("всё куплено, а пишем «осталось»: %q", all.Text)
	}
	empty := b.listMessage(botPlan(), nil, nil, 0, key, i18n.RU)
	if !strings.Contains(empty.Text, "Покупать нечего") || len(empty.Rows) != 1 {
		t.Errorf("пустой список: %+v", empty)
	}
}

func TestToggleDataFitsTelegram(t *testing.T) {
	key, _ := planKey("68a5d55d-0e0d-452a-bd27-2cb4f5892263")
	d := toggleData(key, 3, "tomato_sauce_krasnodar", 0)
	if len(d) > 64 || !strings.HasSuffix(d, "tomato_sauce_krasnodar") {
		t.Errorf("%q, %d байт", d, len(d))
	}
	long := toggleData(key, 3, strings.Repeat("x", 60), 7)
	if len(long) > 64 || !strings.HasSuffix(long, ":#7") {
		t.Errorf("длинный id: %q", long)
	}
	pages := shopPages(botPlan(), nil, i18n.RU)
	if it, ok := findItem(pages, 0, "#1"); !ok || it.ID != "cheese_hard" {
		t.Errorf("по номеру: %+v", it)
	}
	if _, ok := findItem(pages, 0, "#9"); ok {
		t.Error("несуществующий номер найден")
	}
	// на сайте заменили блюдо, отделы сдвинулись: продукт ищем по id во всём списке
	if it, ok := findItem(pages, 0, "potato"); !ok || it.Name != "Картофель" {
		t.Errorf("сдвиг отделов: %+v", it)
	}
}

type stubClient struct {
	messenger.Client
	p messenger.Platform
}

func (s stubClient) Platform() messenger.Platform { return s.p }

// Описания бота на всех языках влезают в лимиты: Telegram — 120 знаков строки профиля и 512
// описания, команда — до 256 (у MAX до 128); пустой ключ — для языков, которых у нас нет.
func TestProfilesFitLimits(t *testing.T) {
	for _, p := range []messenger.Platform{messenger.Telegram, messenger.Max} {
		all := profiles(stubClient{p: p})
		if len(all) != len(i18n.Langs)+1 || all[""].About != i18n.T(i18n.EN, "bot.about") {
			t.Fatalf("%s: языков %d", p, len(all))
		}
		for lang, pr := range all {
			about, desc, cmd := utf8.RuneCountInString(pr.About), utf8.RuneCountInString(pr.Description), utf8.RuneCountInString(pr.Commands[0].Description)
			if about > 120 || desc > 512 || cmd > 128 || strings.HasPrefix(pr.About, "bot.") || strings.Contains(pr.Description, "{") {
				t.Errorf("%s/%s: %d/%d/%d %q", p, lang, about, desc, cmd, pr.Description)
			}
		}
	}
}

func TestPlanKeyRoundTrip(t *testing.T) {
	id := "68a5d55d-0e0d-452a-bd27-2cb4f5892263"
	key, ok := planKey(id)
	if !ok || len(key) != 22 || strings.ContainsAny(key, "+/=") {
		t.Fatalf("ключ %q", key)
	}
	if back, ok := planIDFromKey(key); !ok || back != id {
		t.Errorf("обратно %q", back)
	}
	if _, ok := planIDFromKey("not-a-key"); ok {
		t.Error("мусор принят за неделю")
	}
}
